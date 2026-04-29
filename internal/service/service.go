package service

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/sanchesfree/posiflora_mvp/internal/model"
	"github.com/sanchesfree/posiflora_mvp/internal/repository"
	"github.com/sanchesfree/posiflora_mvp/internal/telegram"
)

type Service struct {
	repo     *repository.Repository
	tgClient telegram.Client
}

func New(repo *repository.Repository, tgClient telegram.Client) *Service {
	return &Service{repo: repo, tgClient: tgClient}
}

func (s *Service) ConnectTelegram(shopID int, req model.ConnectTelegramRequest) (*model.TelegramIntegration, error) {
	// Both empty — just toggle enabled on existing integration
	if req.BotToken == "" && req.ChatID == "" {
		existing, err := s.repo.GetTelegramIntegration(shopID)
		if err != nil {
			return nil, fmt.Errorf("Укажите Bot Token и Chat ID для подключения")
		}
		return s.repo.UpsertTelegramIntegration(shopID, existing.BotToken, existing.ChatID, req.Enabled)
	}
	// One filled, other empty — require both
	if req.BotToken == "" || req.ChatID == "" {
		return nil, fmt.Errorf("заполните оба поля: Bot Token и Chat ID")
	}
	return s.repo.UpsertTelegramIntegration(shopID, req.BotToken, req.ChatID, req.Enabled)
}

func (s *Service) CreateOrder(shopID int, req model.CreateOrderRequest) (*model.CreateOrderResponse, error) {
	// Validate
	if req.Number == "" {
		return nil, fmt.Errorf("Укажите номер заказа")
	}
	if req.Total <= 0 {
		return nil, fmt.Errorf("сумма заказа должна быть больше 0")
	}
	if req.CustomerName == "" {
		return nil, fmt.Errorf("Укажите имя клиента")
	}

	// Create order
	order, err := s.repo.CreateOrder(shopID, req.Number, req.Total, req.CustomerName)
	if err != nil {
		// Check for unique violation
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("Заказ с номером %q уже существует", req.Number)
		}
		return nil, fmt.Errorf("Ошибка создания заказа: %w", err)
	}

	resp := &model.CreateOrderResponse{
		Order:      *order,
		SendStatus: "skipped",
	}

	// Check if telegram integration is enabled
	integ, err := s.repo.GetTelegramIntegration(shopID)
	if err != nil || !integ.Enabled {
		text := fmt.Sprintf("Новый заказ %s на сумму %.0f ₽, клиент %s", order.Number, order.Total, order.CustomerName)
		_ = s.repo.InsertSendLog(shopID, order.ID, text, "SKIPPED", "Telegram disabled")
		return resp, nil
	}

	// Idempotency check: already sent?
	exists, err := s.repo.HasSendLog(shopID, order.ID)
	if err != nil {
		log.Printf("warning: check send log: %v", err)
	}
	if exists {
		_ = s.repo.InsertSendLog(shopID, order.ID, "", "SKIPPED", "duplicate")
		return resp, nil
	}

	// Send notification
	text := fmt.Sprintf("Новый заказ %s на сумму %.0f ₽, клиент %s", order.Number, order.Total, order.CustomerName)
	sendErr := s.tgClient.SendMessage(integ.BotToken, integ.ChatID, text)

	if sendErr != nil {
		// Log FAILED but don't fail the order
		log.Printf("telegram send failed: %v", sendErr)
		_ = s.repo.InsertSendLog(shopID, order.ID, text, "FAILED", sendErr.Error())
		resp.SendStatus = "failed"
	} else {
		_ = s.repo.InsertSendLog(shopID, order.ID, text, "SENT", "")
		resp.SendStatus = "sent"
	}

	return resp, nil
}

func (s *Service) GetTelegramStatus(shopID int) (*model.TelegramStatus, error) {
	return s.repo.GetTelegramStatus(shopID)
}

func (s *Service) GetSendLog(shopID int, limit int) ([]model.SendLogEntry, error) {
	return s.repo.GetSendLog(shopID, limit)
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ Error() string }
	if errors.As(err, &pgErr) {
		return strings.Contains(pgErr.Error(), "duplicate key") || strings.Contains(pgErr.Error(), "UNIQUE constraint")
	}
	return false
}
