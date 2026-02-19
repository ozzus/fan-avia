package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/match"
	"go.uber.org/zap"
)

type ClubHandler struct {
	log     *zap.Logger
	client  *match.Client
	timeout time.Duration
}

type clubResponse struct {
	ClubID string `json:"club_id"`
	NameRU string `json:"name_ru"`
	NameEN string `json:"name_en,omitempty"`
}

func NewClubHandler(log *zap.Logger, client *match.Client, timeout time.Duration) *ClubHandler {
	return &ClubHandler{
		log:     log,
		client:  client,
		timeout: timeout,
	}
}

func (h *ClubHandler) GetClubs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.GetClubs(ctx)
	if err != nil {
		h.log.Error("get clubs failed", zap.Error(err))
		writeError(w, http.StatusBadGateway, "match adapter error")
		return
	}

	clubs := make([]clubResponse, 0, len(resp.GetClubs()))
	for _, c := range resp.GetClubs() {
		clubs = append(clubs, clubResponse{
			ClubID: strings.TrimSpace(c.GetClubId()),
			NameRU: strings.TrimSpace(c.GetNameRu()),
			NameEN: strings.TrimSpace(c.GetNameEn()),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clubs": clubs,
	})
}
