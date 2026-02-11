package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/api-gateway/internal/clients/airfare"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type AirfareHandler struct {
	log     *zap.Logger
	client  *airfare.Client
	timeout time.Duration
}

func NewAirfareHandler(log *zap.Logger, client *airfare.Client, timeout time.Duration) *AirfareHandler {
	return &AirfareHandler{
		log:     log,
		client:  client,
		timeout: timeout,
	}
}

func (h *AirfareHandler) GetAirfareByMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	matchID, ok := parseAirfareMatchIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path, expected /v1/matches/{id}/airfare")
		return
	}

	originIATA := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("origin_iata")))
	if originIATA == "" {
		writeError(w, http.StatusBadRequest, "origin_iata is required")
		return
	}

	resp, err := h.client.GetAirfareByMatch(r.Context(), matchID, originIATA)
	if err != nil {
		writeError(w, mapHTTPStatus(err), mapGRPCError(err))
		return
	}

	data, err := protojson.MarshalOptions{
		EmitUnpopulated: true,
	}.Marshal(resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func parseAirfareMatchIDFromPath(path string) (int64, bool) {
	const prefix = "/v1/matches/"
	const suffix = "/airfare"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}

	idPart := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	idPart = strings.Trim(idPart, "/")
	if idPart == "" {
		return 0, false
	}

	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func mapHTTPStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusBadGateway
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

func mapGRPCError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return "upstream error"
	}
	return st.Message()
}
