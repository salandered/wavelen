package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/storage"
)

const readinessTimeout = time.Second

const healthDependency = "postgres"

type HealthHandler struct {
	Health storage.HealthRepo
}

type HealthResp struct {
	Status     string `json:"status"`
	Dependency string `json:"dependency,omitempty"`
}

func (h *HealthHandler) HandleLive(w http.ResponseWriter, req *http.Request) {
	httputils.WriteJSON(req.Context(), w, http.StatusOK, HealthResp{Status: "ok"})
}

func (h *HealthHandler) HandleReady(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), readinessTimeout)
	defer cancel()

	if err := h.Health.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "readiness check failed",
			"dependency", healthDependency, "error", err)
		httputils.WriteJSON(ctx, w, http.StatusServiceUnavailable, HealthResp{
			Status:     "unavailable",
			Dependency: healthDependency,
		})
		return
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, HealthResp{Status: "ok"})
}
