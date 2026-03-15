package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/segmentio/kafka-go"
	_ "modernc.org/sqlite"
)

const (
	reservePriceCents = 1
	topicImpressions  = "impressions"
	topicClicks       = "clicks"
)

var (
	db                  *sql.DB
	impressWriter       *kafka.Writer
	clickWriter         *kafka.Writer
	brokers             []string
	rankingServiceURL   string
	rankingClient       *http.Client
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/adengine.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("mkdir: %v", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	if err := runSchema(); err != nil {
		log.Fatalf("run schema: %v", err)
	}
	if err := runSeedIfEmpty(ctx); err != nil {
		log.Fatalf("run seed: %v", err)
	}

	brokers = getBrokers()
	if len(brokers) > 0 {
		impressWriter = &kafka.Writer{
			Addr:     kafka.TCP(brokers[0]),
			Topic:    topicImpressions,
			Balancer: &kafka.LeastBytes{},
		}
		clickWriter = &kafka.Writer{
			Addr:     kafka.TCP(brokers[0]),
			Topic:    topicClicks,
			Balancer: &kafka.LeastBytes{},
		}
		defer impressWriter.Close()
		defer clickWriter.Close()
		log.Printf("kafka producer: %v", brokers)
	} else {
		log.Printf("kafka disabled (no KAFKA_BROKERS)")
	}

	rankingServiceURL = strings.TrimSuffix(os.Getenv("RANKING_SERVICE_URL"), "/")
	if rankingServiceURL != "" {
		rankingClient = &http.Client{Timeout: 500 * time.Millisecond}
		log.Printf("ranking service: %s", rankingServiceURL)
	} else {
		log.Printf("ranking service disabled (no RANKING_SERVICE_URL)")
	}

	router := httprouter.New()
	router.GET("/v1/ads", handleGetAds)
	router.POST("/v1/click", handleClick)
	router.GET("/health", handleHealth)

	addr := ":8080"
	log.Printf("ad-server listening on %s (sqlite: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func getBrokers() []string {
	s := os.Getenv("KAFKA_BROKERS")
	if s == "" {
		return nil
	}
	var out []string
	for _, b := range strings.Split(s, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			out = append(out, b)
		}
	}
	return out
}

func runSchema() error {
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	_, err = db.Exec(string(schema))
	return err
}

func runSeedIfEmpty(ctx context.Context) error {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM campaigns").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	seed, err := os.ReadFile("seed.sql")
	if err != nil {
		return fmt.Errorf("read seed: %w", err)
	}
	_, err = db.Exec(string(seed))
	return err
}

func handleHealth(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type candidate struct {
	AdID       int64
	CampaignID int64
	BidCents   int64
	Score      float64 // pCTR from ranking service; 1.0 if no ranking
	Title      string
	Body       string
	ImageURL   string
	LandingURL string
}

// effectiveBid returns bid_cents * score for ranking-based auction.
func (c candidate) effectiveBid() float64 {
	if c.Score <= 0 {
		return float64(c.BidCents)
	}
	return float64(c.BidCents) * c.Score
}

// byEffectiveBid sorts by effective bid descending, then AdID for tie-break.
type byEffectiveBid []candidate

func (b byEffectiveBid) Len() int { return len(b) }
func (b byEffectiveBid) Less(i, j int) bool {
	ei, ej := b[i].effectiveBid(), b[j].effectiveBid()
	return ei > ej || (ei == ej && b[i].AdID < b[j].AdID)
}
func (b byEffectiveBid) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// runSecondPriceAuction returns winner and price_paid_cents (second-highest effective bid or reserve).
func runSecondPriceAuction(candidates []candidate) (winner candidate, pricePaidCents int64) {
	if len(candidates) == 0 {
		return candidate{}, 0
	}
	sorted := make([]candidate, len(candidates))
	copy(sorted, candidates)
	sort.Sort(byEffectiveBid(sorted))
	winner = sorted[0]
	if len(sorted) < 2 {
		pricePaidCents = reservePriceCents
		return
	}
	secondEff := sorted[1].effectiveBid()
	pricePaidCents = int64(secondEff)
	if pricePaidCents < reservePriceCents {
		pricePaidCents = reservePriceCents
	}
	return
}

type impressionEvent struct {
	RequestID      string `json:"request_id"`
	UserID         string `json:"user_id"`
	AdID           int64  `json:"ad_id"`
	CampaignID     int64  `json:"campaign_id"`
	EventType      string `json:"event_type"`
	PricePaidCents int64  `json:"price_paid_cents"`
	Timestamp      string `json:"timestamp"`
}

func produceImpression(reqID, userID string, adID, campaignID, pricePaidCents int64) {
	if impressWriter == nil {
		return
	}
	ev := impressionEvent{
		RequestID:      reqID,
		UserID:         userID,
		AdID:           adID,
		CampaignID:     campaignID,
		EventType:      "impression",
		PricePaidCents: pricePaidCents,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(ev)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := impressWriter.WriteMessages(ctx, kafka.Message{Key: []byte(reqID), Value: body}); err != nil {
		log.Printf("kafka write impression: %v", err)
	}
}

func handleGetAds(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	candidates, err := fetchCandidates(ctx)
	if err != nil {
		log.Printf("fetch candidates: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(candidates) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ad": nil, "request_id": requestID})
		return
	}

	// Attach ranking scores (pCTR); default 1.0 if ranking service unavailable
	if err := attachScores(ctx, candidates, userID); err != nil {
		log.Printf("ranking: %v (using score=1.0)", err)
		for i := range candidates {
			if candidates[i].Score == 0 {
				candidates[i].Score = 1.0
			}
		}
	} else {
		for i := range candidates {
			if candidates[i].Score == 0 {
				candidates[i].Score = 1.0
			}
		}
	}

	winner, pricePaidCents := runSecondPriceAuction(candidates)

	go produceImpression(requestID, userID, winner.AdID, winner.CampaignID, pricePaidCents)

	resp := map[string]interface{}{
		"ad": map[string]interface{}{
			"ad_id":       winner.AdID,
			"campaign_id": winner.CampaignID,
			"title":       winner.Title,
			"body":        winner.Body,
			"image_url":   winner.ImageURL,
			"landing_url": winner.LandingURL,
		},
		"user_id":     userID,
		"request_id":  requestID,
		"price_paid_cents": pricePaidCents,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type clickRequest struct {
	RequestID  string `json:"request_id"`
	AdID       int64  `json:"ad_id"`
	CampaignID int64  `json:"campaign_id"`
	UserID     string `json:"user_id,omitempty"`
}

func produceClick(ev impressionEvent) {
	if clickWriter == nil {
		return
	}
	ev.EventType = "click"
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(ev)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := clickWriter.WriteMessages(ctx, kafka.Message{Key: []byte(ev.RequestID), Value: body}); err != nil {
		log.Printf("kafka write click: %v", err)
	}
}

func handleClick(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Body == nil {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	var req clickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.AdID == 0 || req.CampaignID == 0 {
		http.Error(w, "ad_id and campaign_id required", http.StatusBadRequest)
		return
	}
	userID := req.UserID
	if userID == "" {
		userID = "anonymous"
	}
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	go produceClick(impressionEvent{
		RequestID:      req.RequestID,
		UserID:         userID,
		AdID:           req.AdID,
		CampaignID:     req.CampaignID,
		EventType:      "click",
		PricePaidCents: 0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ranking API request/response
type rankCandidate struct {
	AdID       int64 `json:"ad_id"`
	CampaignID int64 `json:"campaign_id"`
	BidCents   int64 `json:"bid_cents"`
}
type rankRequest struct {
	Candidates []rankCandidate `json:"candidates"`
	Context    map[string]string `json:"context,omitempty"`
}
type rankScore struct {
	AdID  int64   `json:"ad_id"`
	Score float64 `json:"score"`
}
type rankResponse struct {
	Scores []rankScore `json:"scores"`
}

func attachScores(ctx context.Context, candidates []candidate, userID string) error {
	if rankingClient == nil || rankingServiceURL == "" {
		for i := range candidates {
			candidates[i].Score = 1.0
		}
		return nil
	}
	reqBody := rankRequest{
		Candidates: make([]rankCandidate, len(candidates)),
		Context:    map[string]string{"user_id": userID},
	}
	for i := range candidates {
		reqBody.Candidates[i] = rankCandidate{
			AdID:       candidates[i].AdID,
			CampaignID: candidates[i].CampaignID,
			BidCents:   candidates[i].BidCents,
		}
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", rankingServiceURL+"/rank", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := rankingClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ranking status %d", resp.StatusCode)
	}
	var out rankResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	scoreByAdID := make(map[int64]float64)
	for _, s := range out.Scores {
		scoreByAdID[s.AdID] = s.Score
	}
	for i := range candidates {
		if s, ok := scoreByAdID[candidates[i].AdID]; ok {
			candidates[i].Score = s
		} else {
			candidates[i].Score = 1.0
		}
	}
	return nil
}

func fetchCandidates(ctx context.Context) ([]candidate, error) {
	query := `
		SELECT a.id, a.campaign_id, c.bid_cents, a.title, a.body, a.image_url, a.landing_url
		FROM ads a
		JOIN campaigns c ON c.id = a.campaign_id
		WHERE c.status = 'active'
		AND (c.start_at IS NULL OR c.start_at <= datetime('now'))
		AND (c.end_at IS NULL OR c.end_at >= datetime('now'))
		LIMIT 50
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []candidate
	for rows.Next() {
		var c candidate
		var body, imageURL sql.NullString
		if err := rows.Scan(&c.AdID, &c.CampaignID, &c.BidCents, &c.Title, &body, &imageURL, &c.LandingURL); err != nil {
			return nil, err
		}
		if body.Valid {
			c.Body = body.String
		}
		if imageURL.Valid {
			c.ImageURL = imageURL.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
