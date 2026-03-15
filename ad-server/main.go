package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/julienschmidt/httprouter"
)

var db *sql.DB

func main() {
	connStr := os.Getenv("DB_CONN")
	if connStr == "" {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "adengine")
		pass := getEnv("DB_PASSWORD", "adengine")
		dbname := getEnv("DB_NAME", "adengine")
		connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	}
	config, err := stdlib.ParseConfig(connStr)
	if err != nil {
		log.Fatalf("parse config: %v", err)
	}
	db = stdlib.OpenDB(*config)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	router := httprouter.New()
	router.GET("/v1/ads", handleGetAds)
	router.GET("/health", handleHealth)

	addr := ":8080"
	log.Printf("ad-server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type candidate struct {
	AdID       int64
	CampaignID int64
	BidCents   int64
	Title      string
	Body       string
	ImageURL   string
	LandingURL string
}

func handleGetAds(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	_ = r.URL.Query().Get("placement") // optional

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
		json.NewEncoder(w).Encode(map[string]interface{}{"ad": nil})
		return
	}

	// Phase 1: simple auction — highest bid wins
	winner := candidates[0]
	for _, c := range candidates[1:] {
		if c.BidCents > winner.BidCents {
			winner = c
		}
	}

	resp := map[string]interface{}{
		"ad": map[string]interface{}{
			"ad_id":       winner.AdID,
			"campaign_id": winner.CampaignID,
			"title":       winner.Title,
			"body":        winner.Body,
			"image_url":   winner.ImageURL,
			"landing_url": winner.LandingURL,
		},
		"user_id": userID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func fetchCandidates(ctx context.Context) ([]candidate, error) {
	// Eligible: campaign status = active, optional: start_at <= now <= end_at
	query := `
		SELECT a.id, a.campaign_id, c.bid_cents, a.title, a.body, a.image_url, a.landing_url
		FROM ads a
		JOIN campaigns c ON c.id = a.campaign_id
		WHERE c.status = 'active'
		AND (c.start_at IS NULL OR c.start_at <= NOW())
		AND (c.end_at IS NULL OR c.end_at >= NOW())
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
