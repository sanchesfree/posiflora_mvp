package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/sanchesfree/posiflora_mvp/internal/model"
	"github.com/sanchesfree/posiflora_mvp/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /shops/{shopId}/telegram/connect", h.ConnectTelegram)
	mux.HandleFunc("POST /shops/{shopId}/orders", h.CreateOrder)
	mux.HandleFunc("GET /shops/{shopId}/telegram/status", h.GetTelegramStatus)
	mux.HandleFunc("GET /shops/{shopId}/telegram/log", h.GetSendLog)
}

func (h *Handler) ConnectTelegram(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.Atoi(r.PathValue("shopId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный shopId"})
		return
	}

	var req model.ConnectTelegramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный формат JSON"})
		return
	}

	integ, err := h.svc.ConnectTelegram(shopID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, integ)
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.Atoi(r.PathValue("shopId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный shopId"})
		return
	}

	var req model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный формат JSON"})
		return
	}

	resp, err := h.svc.CreateOrder(shopID, req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "уже существует") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) GetTelegramStatus(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.Atoi(r.PathValue("shopId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный shopId"})
		return
	}

	status, err := h.svc.GetTelegramStatus(shopID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Интеграция не найдена"})
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) GetSendLog(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.Atoi(r.PathValue("shopId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный shopId"})
		return
	}

	log, err := h.svc.GetSendLog(shopID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, log)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
