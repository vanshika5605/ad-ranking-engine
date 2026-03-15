package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/adengine.db"
	}
	if err := os.MkdirAll("./data", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("mkdir data: %v", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath+"?_foreign_keys=on")
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

	router := httprouter.New()
	router.GET("/v1/ads", handleGetAds)
	router.GET("/health", handleHealth)

	addr := ":8080"
	log.Printf("ad-server listening on %s (sqlite: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
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
