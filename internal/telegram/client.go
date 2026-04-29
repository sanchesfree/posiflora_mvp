package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client interface for Telegram API — easy to mock in tests
type Client interface {
	SendMessage(botToken, chatID, text string) error
}

// RealClient sends messages via Telegram Bot API
type RealClient struct {
	HTTPClient *http.Client
}

func NewRealClient() *RealClient {
	return &RealClient{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RealClient) SendMessage(botToken, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api returned status %d", resp.StatusCode)
	}
	return nil
}

// MockClient records messages without sending them
type MockClient struct {
	Messages []MockMessage
	ShouldFail bool
}

type MockMessage struct {
	BotToken string
	ChatID   string
	Text     string
}

func NewMockClient() *MockClient {
	return &MockClient{Messages: make([]MockMessage, 0)}
}

func (m *MockClient) SendMessage(botToken, chatID, text string) error {
	if m.ShouldFail {
		return fmt.Errorf("mock telegram error")
	}
	m.Messages = append(m.Messages, MockMessage{
		BotToken: botToken,
		ChatID:   chatID,
		Text:     text,
	})
	return nil
}
