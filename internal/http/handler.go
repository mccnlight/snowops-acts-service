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
	"github.com/nurpe/snowops-acts/internal/model"
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
	protected.POST("/acts/export", h.exportActs)
	protected.POST("/acts/export/pdf", h.exportActsPDF)
}

type exportActsRequest struct {
	Mode        string `json:"mode" binding:"required"`
	TargetID    string `json:"target_id" binding:"required"`
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}

func (h *Handler) exportActs(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing principal"})
		return
	}

	var req exportActsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode, err := parseReportMode(req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode"})
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(req.TargetID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
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

	result, err := h.acts.GenerateReport(c.Request.Context(), service.GenerateReportInput{
		Mode:        mode,
		TargetID:    targetID,
		PeriodStart: start,
		PeriodEnd:   end,
		Principal:   principal,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=\""+result.FileName+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", result.Content)
}

func (h *Handler) exportActsPDF(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing principal"})
		return
	}

	var req exportActsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode, err := parseReportMode(req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode"})
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(req.TargetID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
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

	result, err := h.acts.GenerateReportPDF(c.Request.Context(), service.GenerateReportInput{
		Mode:        mode,
		TargetID:    targetID,
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
	default:
		h.log.Error().Err(err).Msg("generate report failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func parseReportMode(raw string) (model.ReportMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "contractor":
		return model.ReportModeContractor, nil
	case "landfill":
		return model.ReportModeLandfill, nil
	default:
		return "", service.ErrInvalidInput
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
