package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sanchesfree/posiflora_mvp/internal/model"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- Shops ---

func (r *Repository) GetShopByID(id int) (*model.Shop, error) {
	shop := &model.Shop{}
	err := r.db.QueryRow("SELECT id, name FROM shops WHERE id = $1", id).Scan(&shop.ID, &shop.Name)
	if err != nil {
		return nil, err
	}
	return shop, nil
}

// --- Telegram Integration ---

func (r *Repository) UpsertTelegramIntegration(shopID int, botToken, chatID string, enabled bool) (*model.TelegramIntegration, error) {
	integ := &model.TelegramIntegration{}
	query := `
		INSERT INTO telegram_integrations (shop_id, bot_token, chat_id, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (shop_id) DO UPDATE SET
			bot_token = EXCLUDED.bot_token,
			chat_id = EXCLUDED.chat_id,
			enabled = EXCLUDED.enabled,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, shop_id, bot_token, chat_id, enabled, created_at, updated_at
	`
	err := r.db.QueryRow(query, shopID, botToken, chatID, enabled).Scan(
		&integ.ID, &integ.ShopID, &integ.BotToken, &integ.ChatID,
		&integ.Enabled, &integ.CreatedAt, &integ.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert telegram integration: %w", err)
	}
	return integ, nil
}

func (r *Repository) GetTelegramIntegration(shopID int) (*model.TelegramIntegration, error) {
	integ := &model.TelegramIntegration{}
	err := r.db.QueryRow(
		"SELECT id, shop_id, bot_token, chat_id, enabled, created_at, updated_at FROM telegram_integrations WHERE shop_id = $1",
		shopID,
	).Scan(&integ.ID, &integ.ShopID, &integ.BotToken, &integ.ChatID, &integ.Enabled, &integ.CreatedAt, &integ.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return integ, nil
}

// --- Orders ---

func (r *Repository) CreateOrder(shopID int, number string, total float64, customerName string) (*model.Order, error) {
	order := &model.Order{}
	query := `
		INSERT INTO orders (shop_id, number, total, customer_name, created_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		RETURNING id, shop_id, number, total, customer_name, created_at
	`
	err := r.db.QueryRow(query, shopID, number, total, customerName).Scan(
		&order.ID, &order.ShopID, &order.Number, &order.Total, &order.CustomerName, &order.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	return order, nil
}

func (r *Repository) GetOrderByShopAndNumber(shopID int, number string) (*model.Order, error) {
	order := &model.Order{}
	err := r.db.QueryRow(
		"SELECT id, shop_id, number, total, customer_name, created_at FROM orders WHERE shop_id = $1 AND number = $2",
		shopID, number,
	).Scan(&order.ID, &order.ShopID, &order.Number, &order.Total, &order.CustomerName, &order.CreatedAt)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// --- Telegram Send Log ---

func (r *Repository) HasSendLog(shopID, orderID int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM telegram_send_log WHERE shop_id = $1 AND order_id = $2)",
		shopID, orderID,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) InsertSendLog(shopID, orderID int, message, status, errMsg string) error {
	query := `
		INSERT INTO telegram_send_log (shop_id, order_id, message, status, error, sent_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (shop_id, order_id) DO NOTHING
	`
	_, err := r.db.Exec(query, shopID, orderID, message, status, errMsg)
	return err
}

// --- Status ---

func (r *Repository) GetSendLog(shopID int, limit int) ([]model.SendLogEntry, error) {
	query := `
		SELECT o.id, o.number, o.total, o.customer_name,
			COALESCE(sl.message, ''), COALESCE(sl.status, 'SKIPPED'),
			COALESCE(sl.error, 'Заказ без отправки'), o.created_at
		FROM orders o
		LEFT JOIN telegram_send_log sl ON sl.order_id = o.id AND sl.shop_id = $1
		WHERE o.shop_id = $1
		ORDER BY o.created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(query, shopID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries = make([]model.SendLogEntry, 0)
	for rows.Next() {
		var e model.SendLogEntry
		if err := rows.Scan(&e.OrderID, &e.OrderNumber, &e.Total, &e.CustomerName, &e.Message, &e.Status, &e.Error, &e.SentAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *Repository) GetTelegramStatus(shopID int) (*model.TelegramStatus, error) {
	integ, err := r.GetTelegramIntegration(shopID)
	if err != nil {
		return nil, err
	}

	// Mask chat_id for security: show first 3 chars + ***
	maskedChatID := integ.ChatID
	if len(maskedChatID) > 3 {
		maskedChatID = maskedChatID[:3] + "***"
	}

	status := &model.TelegramStatus{
		Enabled:      integ.Enabled,
		ChatID:       maskedChatID,
		BotConnected: integ.BotToken != "" && integ.ChatID != "",
	}

	// Last sent at
	var lastSentAt *time.Time
	row := r.db.QueryRow(
		"SELECT MAX(sent_at) FROM telegram_send_log WHERE shop_id = $1 AND status = 'SENT'",
		shopID,
	)
	if err := row.Scan(&lastSentAt); err == nil {
		status.LastSentAt = lastSentAt
	}

	// Counts for last 7 days
	since := time.Now().AddDate(0, 0, -7)
	_ = r.db.QueryRow(
		"SELECT COUNT(*) FROM telegram_send_log WHERE shop_id = $1 AND status = 'SENT' AND sent_at >= $2",
		shopID, since,
	).Scan(&status.SentCount)

	_ = r.db.QueryRow(
		"SELECT COUNT(*) FROM telegram_send_log WHERE shop_id = $1 AND status = 'FAILED' AND sent_at >= $2",
		shopID, since,
	).Scan(&status.FailedCount)

	return status, nil
}
