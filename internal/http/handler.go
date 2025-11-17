package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/nurpe/snowops-acts/internal/http/middleware"
	"github.com/nurpe/snowops-acts/internal/service"
)

type Handler struct {
	acts *service.ActService
	log  zerolog.Logger
}

func NewHandler(acts *service.ActService, log zerolog.Logger) *Handler {
	return &Handler{acts: acts, log: log}
}

func (h *Handler) Register(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	protected := router.Group("/")
	protected.Use(authMiddleware)
	protected.POST("/contracts/:contract_id/acts/generate-pdf", h.generateAct)
}

type generateActRequest struct {
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}

func (h *Handler) generateAct(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing principal"})
		return
	}

	contractID, err := uuid.Parse(strings.TrimSpace(c.Param("contract_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contract_id"})
		return
	}

	var req generateActRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start, err := parseDate(req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_start"})
		return
	}
	end, err := parseDate(req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_end"})
		return
	}

	result, err := h.acts.GenerateActPDF(c.Request.Context(), service.GenerateActInput{
		ContractID:  contractID,
		PeriodStart: start,
		PeriodEnd:   end,
		Principal:   principal,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=\""+result.FileName+"\"")
	c.Data(http.StatusOK, "application/pdf", result.Content)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNoTrips):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		h.log.Error().Err(err).Msg("generate act failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, service.ErrInvalidInput
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, service.ErrInvalidInput
}
