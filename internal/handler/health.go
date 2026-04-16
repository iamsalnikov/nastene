package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Health struct {
	DB Pinger
}

func (h *Health) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := "ok"
	code := http.StatusOK
	var dbErr string
	if err := h.DB.Ping(ctx); err != nil {
		status = "degraded"
		code = http.StatusServiceUnavailable
		dbErr = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"db":     dbErr,
	})
}
