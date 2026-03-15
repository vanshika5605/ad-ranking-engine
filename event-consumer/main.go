package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	_ "modernc.org/sqlite"
)

const (
	topicImpressions = "impressions"
	topicClicks      = "clicks"
)

type eventPayload struct {
	RequestID      string `json:"request_id"`
	UserID         string `json:"user_id"`
	AdID           int64  `json:"ad_id"`
	CampaignID     int64  `json:"campaign_id"`
	EventType      string `json:"event_type"`
	PricePaidCents int64  `json:"price_paid_cents"`
	Timestamp      string `json:"timestamp"`
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/adengine.db"
	}
	brokers := getBrokers()
	if len(brokers) == 0 {
		log.Fatal("KAFKA_BROKERS required")
	}

	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		log.Printf("warning: PRAGMA busy_timeout: %v", err)
	}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		consume(ctx, brokers, topicImpressions, db)
	}()
	go func() {
		defer wg.Done()
		consume(ctx, brokers, topicClicks, db)
	}()
	log.Printf("event-consumer started (kafka=%v, db=%s)", brokers, dbPath)
	wg.Wait()
	log.Println("event-consumer stopped")
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

func consume(ctx context.Context, brokers []string, topic string, db *sql.DB) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:         topic,
		GroupID:       "event-consumer",
		MinBytes:      1,
		MaxBytes:      10e6,
		MaxWait:       time.Second,
		CommitInterval: 0,
	})
	defer r.Close()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[%s] fetch: %v", topic, err)
			continue
		}
		var ev eventPayload
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			log.Printf("[%s] unmarshal: %v", topic, err)
			_ = r.CommitMessages(ctx, msg)
			continue
		}
		if err := persistEvent(ctx, db, &ev); err != nil {
			log.Printf("[%s] persist: %v", topic, err)
			continue
		}
		if err := r.CommitMessages(ctx, msg); err != nil {
			log.Printf("[%s] commit: %v", topic, err)
		}
	}
}

func persistEvent(ctx context.Context, db *sql.DB, ev *eventPayload) error {
	date := dateFromTimestamp(ev.Timestamp)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (request_id, user_id, ad_id, campaign_id, event_type, price_paid_cents, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ev.RequestID, ev.UserID, ev.AdID, ev.CampaignID, ev.EventType, ev.PricePaidCents, ev.Timestamp,
	)
	if err != nil {
		return err
	}
	// Upsert campaign_stats_daily
	if ev.EventType == "impression" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO campaign_stats_daily (campaign_id, date, impressions, clicks, spend_cents) VALUES (?, ?, 1, 0, ?)
			 ON CONFLICT(campaign_id, date) DO UPDATE SET impressions = impressions + 1, spend_cents = spend_cents + excluded.spend_cents`,
			ev.CampaignID, date, ev.PricePaidCents,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO campaign_stats_daily (campaign_id, date, impressions, clicks, spend_cents) VALUES (?, ?, 0, 1, 0)
			 ON CONFLICT(campaign_id, date) DO UPDATE SET clicks = clicks + 1`,
			ev.CampaignID, date,
		)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func dateFromTimestamp(ts string) string {
	if ts == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Now().UTC().Format("2006-01-02")
	}
	return t.UTC().Format("2006-01-02")
}
