package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lib/pq"
	"github.com/sanchesfree/posiflora_mvp/internal/handler"
	"github.com/sanchesfree/posiflora_mvp/internal/repository"
	"github.com/sanchesfree/posiflora_mvp/internal/service"
	"github.com/sanchesfree/posiflora_mvp/internal/telegram"
)

func main() {
	dbURL := envOrDefault("DATABASE_URL", "postgres://posiflora:posiflora@localhost:5432/posiflora?sslmode=disable")
	port := envOrDefault("PORT", "8080")

	// Connect to DB
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Wait for DB to be ready
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		log.Printf("waiting for db... (%d/30)", i+1)
		if i == 29 {
			log.Fatalf("db not ready after 30 attempts")
		}
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations applied")

	// Seed data
	if err := seedData(db); err != nil {
		log.Printf("seed warning: %v", err)
	}

	// Choose telegram client
	var tgClient telegram.Client
	if os.Getenv("TELEGRAM_REAL") == "true" {
		tgClient = telegram.NewRealClient()
		log.Println("using REAL telegram client")
	} else {
		tgClient = telegram.NewMockClient()
		log.Println("using MOCK telegram client (set TELEGRAM_REAL=true for real)")
	}

	// Wire up layers
	repo := repository.New(db)
	svc := service.New(repo, tgClient)
	h := handler.New(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// CORS middleware for frontend
	corsHandler := corsMiddleware(mux)

	addr := ":" + port
	log.Printf("server starting on %s", addr)
	if err := http.ListenAndServe(addr, corsHandler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runMigrations(db *sql.DB) error {
	migration := `
	CREATE TABLE IF NOT EXISTS shops (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS telegram_integrations (
		id SERIAL PRIMARY KEY,
		shop_id INTEGER NOT NULL REFERENCES shops(id),
		bot_token TEXT NOT NULL,
		chat_id TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(shop_id)
	);

	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		shop_id INTEGER NOT NULL REFERENCES shops(id),
		number TEXT NOT NULL,
		total NUMERIC(12,2) NOT NULL,
		customer_name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(shop_id, number)
	);

	CREATE TABLE IF NOT EXISTS telegram_send_log (
		id SERIAL PRIMARY KEY,
		shop_id INTEGER NOT NULL REFERENCES shops(id),
		order_id INTEGER NOT NULL REFERENCES orders(id),
		message TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('SENT', 'FAILED', 'SKIPPED')),
		error TEXT,
		sent_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(shop_id, order_id)
	);
	`
	_, err := db.Exec(migration)
	return err
}

func seedData(db *sql.DB) error {
	// Check if already seeded
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM shops").Scan(&count)
	if count > 0 {
		log.Println("data already seeded, skipping")
		return nil
	}

	// Seed shop
	var shopID int
	err := db.QueryRow("INSERT INTO shops (name) VALUES ($1) RETURNING id", "Магазин цветов «Роза»").Scan(&shopID)
	if err != nil {
		return fmt.Errorf("seed shop: %w", err)
	}

	// Orders are seeded below — integration is configured via UI

	// Seed orders
	orders := []struct {
		number string
		total  float64
		name   string
	}{
		{"A-1001", 1500, "Иван"},
		{"A-1002", 3200, "Мария"},
		{"A-1003", 890, "Пётр"},
		{"A-1004", 5600, "Елена"},
		{"A-1005", 2490, "Анна"},
		{"A-1006", 1750, "Дмитрий"},
		{"A-1007", 4100, "Ольга"},
		{"A-1008", 980, "Сергей"},
	}

	for _, o := range orders {
		_, err := db.Exec(
			"INSERT INTO orders (shop_id, number, total, customer_name) VALUES ($1, $2, $3, $4)",
			shopID, o.number, o.total, o.name,
		)
		if err != nil {
			return fmt.Errorf("seed order %s: %w", o.number, err)
		}
	}

	log.Printf("seeded shop_id=%d with %d orders", shopID, len(orders))
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// suppress unused import warning
var _ = pq.Driver{}
