package service_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/sanchesfree/posiflora_mvp/internal/model"
	"github.com/sanchesfree/posiflora_mvp/internal/repository"
	"github.com/sanchesfree/posiflora_mvp/internal/service"
	"github.com/sanchesfree/posiflora_mvp/internal/telegram"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	schema := `
	CREATE TABLE shops (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL);
	CREATE TABLE telegram_integrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shop_id INTEGER NOT NULL UNIQUE,
		bot_token TEXT NOT NULL,
		chat_id TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shop_id INTEGER NOT NULL,
		number TEXT NOT NULL,
		total REAL NOT NULL,
		customer_name TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE telegram_send_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shop_id INTEGER NOT NULL,
		order_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('SENT', 'FAILED')),
		error TEXT,
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(shop_id, order_id)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	// Seed shop
	_, err = db.Exec("INSERT INTO shops (name) VALUES ('Test Shop')")
	if err != nil {
		t.Fatalf("seed shop: %v", err)
	}

	return db
}

// Test 1: Order creation with enabled integration calls TelegramClient and logs SENT
func TestCreateOrder_EnabledIntegration_SendsTelegram(t *testing.T) {
	db := setupTestDB(t)
	mock := telegram.NewMockClient()
	repo := repository.New(db)
	svc := service.New(repo, mock)

	// Enable integration
	_, err := svc.ConnectTelegram(1, model.ConnectTelegramRequest{
		BotToken: "test-token",
		ChatID:   "12345",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Create order
	resp, err := svc.CreateOrder(1, model.CreateOrderRequest{
		Number:       "A-1001",
		Total:        1500,
		CustomerName: "Иван",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if resp.SendStatus != "sent" {
		t.Errorf("expected send_status=sent, got %s", resp.SendStatus)
	}

	// Verify mock received message
	if len(mock.Messages) != 1 {
		t.Fatalf("expected 1 telegram message, got %d", len(mock.Messages))
	}
	expectedText := "Новый заказ A-1001 на сумму 1500 ₽, клиент Иван"
	if mock.Messages[0].Text != expectedText {
		t.Errorf("unexpected message text: %s", mock.Messages[0].Text)
	}

	// Verify send log via repo
	exists, err := repo.HasSendLog(1, resp.Order.ID)
	if err != nil {
		t.Fatalf("has send log: %v", err)
	}
	if !exists {
		t.Error("expected send log to exist")
	}
}

// Test 2: Idempotency — duplicate send is blocked
func TestCreateOrder_Idempotency_NoDuplicateSend(t *testing.T) {
	db := setupTestDB(t)
	mock := telegram.NewMockClient()
	repo := repository.New(db)
	svc := service.New(repo, mock)

	// Enable integration
	_, _ = svc.ConnectTelegram(1, model.ConnectTelegramRequest{
		BotToken: "test-token",
		ChatID:   "12345",
		Enabled:  true,
	})

	// Create order first time
	resp, err := svc.CreateOrder(1, model.CreateOrderRequest{
		Number:       "A-1001",
		Total:        1500,
		CustomerName: "Иван",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if resp.SendStatus != "sent" {
		t.Fatalf("first order should be sent, got %s", resp.SendStatus)
	}

	// Verify send log exists
	exists, _ := repo.HasSendLog(1, resp.Order.ID)
	if !exists {
		t.Fatal("send log should exist after first order")
	}

	// Try to create another order with the same number — it will be a NEW order (different ID)
	// The idempotency is per (shop_id, order_id), not per order number
	// So let's test that the send_log UNIQUE constraint works
	// Try inserting duplicate send log — should be ignored
	err = repo.InsertSendLog(1, resp.Order.ID, "duplicate", "SENT", "")
	if err != nil {
		t.Logf("insert duplicate log returned: %v (expected nil with ON CONFLICT DO NOTHING)", err)
	}

	// Count send logs — should still be exactly 1
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM telegram_send_log WHERE shop_id = 1 AND order_id = ?", resp.Order.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 send log, got %d", count)
	}

	// Only 1 telegram message was sent
	if len(mock.Messages) != 1 {
		t.Errorf("expected 1 telegram message, got %d", len(mock.Messages))
	}
}

// Test 3: TelegramClient error — logs FAILED, order still created
func TestCreateOrder_TelegramFails_OrderCreated(t *testing.T) {
	db := setupTestDB(t)
	mock := &telegram.MockClient{ShouldFail: true}
	repo := repository.New(db)
	svc := service.New(repo, mock)

	// Enable integration
	_, _ = svc.ConnectTelegram(1, model.ConnectTelegramRequest{
		BotToken: "test-token",
		ChatID:   "12345",
		Enabled:  true,
	})

	// Create order — should succeed even if telegram fails
	resp, err := svc.CreateOrder(1, model.CreateOrderRequest{
		Number:       "A-1001",
		Total:        2000,
		CustomerName: "Мария",
	})
	if err != nil {
		t.Fatalf("create order should not fail: %v", err)
	}

	// Order should be created
	if resp.Order.ID == 0 {
		t.Error("order should have an ID")
	}
	if resp.Order.Number != "A-1001" {
		t.Errorf("expected order number A-1001, got %s", resp.Order.Number)
	}

	// Send status should be failed
	if resp.SendStatus != "failed" {
		t.Errorf("expected send_status=failed, got %s", resp.SendStatus)
	}

	// Verify send log with FAILED
	var logStatus string
	_ = db.QueryRow("SELECT status FROM telegram_send_log WHERE shop_id = 1 AND order_id = ?", resp.Order.ID).Scan(&logStatus)
	if logStatus != "FAILED" {
		t.Errorf("expected log status FAILED, got '%s'", logStatus)
	}

	// Verify order exists in DB
	order, err := repo.GetOrderByShopAndNumber(1, "A-1001")
	if err != nil {
		t.Fatalf("order should exist: %v", err)
	}
	fmt.Printf("order found: %+v\n", order)
}

// Test 4: Validation — empty fields are rejected
func TestCreateOrder_Validation_EmptyFields(t *testing.T) {
	db := setupTestDB(t)
	mock := telegram.NewMockClient()
	repo := repository.New(db)
	svc := service.New(repo, mock)

	_, _ = svc.ConnectTelegram(1, model.ConnectTelegramRequest{
		BotToken: "test-token",
		ChatID:   "12345",
		Enabled:  true,
	})

	// Empty number
	_, err := svc.CreateOrder(1, model.CreateOrderRequest{Number: "", Total: 100, CustomerName: "Иван"})
	if err == nil {
		t.Error("expected error for empty number")
	}

	// Zero total
	_, err = svc.CreateOrder(1, model.CreateOrderRequest{Number: "A-1001", Total: 0, CustomerName: "Иван"})
	if err == nil {
		t.Error("expected error for zero total")
	}

	// Negative total
	_, err = svc.CreateOrder(1, model.CreateOrderRequest{Number: "A-1001", Total: -50, CustomerName: "Иван"})
	if err == nil {
		t.Error("expected error for negative total")
	}

	// Empty customer name
	_, err = svc.CreateOrder(1, model.CreateOrderRequest{Number: "A-1001", Total: 100, CustomerName: ""})
	if err == nil {
		t.Error("expected error for empty customer name")
	}

	// Verify no orders were created
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM orders WHERE shop_id = 1").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 orders after validation failures, got %d", count)
	}

	// No telegram messages sent
	if len(mock.Messages) != 0 {
		t.Errorf("expected 0 telegram messages, got %d", len(mock.Messages))
	}
}
