package model

import "time"

type Shop struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TelegramIntegration struct {
	ID        int       `json:"id"`
	ShopID    int       `json:"shop_id"`
	BotToken  string    `json:"bot_token"`
	ChatID    string    `json:"chat_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Order struct {
	ID           int     `json:"id"`
	ShopID       int     `json:"shop_id"`
	Number       string  `json:"number"`
	Total        float64 `json:"total"`
	CustomerName string  `json:"customer_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type TelegramSendLog struct {
	ID      int    `json:"id"`
	ShopID  int    `json:"shop_id"`
	OrderID int    `json:"order_id"`
	Message string `json:"message"`
	Status  string `json:"status"` // SENT, FAILED, SKIPPED
	Error   string `json:"error,omitempty"`
	SentAt  time.Time `json:"sent_at"`
}

type TelegramStatus struct {
	Enabled      bool       `json:"enabled"`
	ChatID       string     `json:"chat_id"`
	LastSentAt   *time.Time `json:"last_sent_at"`
	SentCount    int        `json:"sent_count"`
	FailedCount  int        `json:"failed_count"`
	BotConnected bool       `json:"bot_connected"`
}

type CreateOrderRequest struct {
	Number       string  `json:"number"`
	Total        float64 `json:"total"`
	CustomerName string  `json:"customerName"`
}

type ConnectTelegramRequest struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
	Enabled  bool   `json:"enabled"`
}

type SendLogEntry struct {
	OrderID      int     `json:"order_id"`
	OrderNumber  string  `json:"order_number"`
	Total        float64 `json:"total"`
	CustomerName string  `json:"customer_name"`
	Message      string  `json:"message"`
	Status       string  `json:"status"`
	Error        string  `json:"error,omitempty"`
	SentAt       time.Time `json:"sent_at"`
}

type CreateOrderResponse struct {
	Order      Order  `json:"order"`
	SendStatus string `json:"send_status"` // sent / failed / skipped
}
