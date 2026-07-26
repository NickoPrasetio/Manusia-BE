package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/manusia/review-service/internal/service"
)

type AppealHandler struct {
	svc *service.AppealService
}

func NewAppealHandler(svc *service.AppealService) *AppealHandler {
	return &AppealHandler{svc: svc}
}

type appealCommentBody struct {
	Comment string `json:"comment"`
}

// CreateAppeal handles POST /api/reviews/:id/appeal
func (h *AppealHandler) CreateAppeal(c *gin.Context) {
	reviewID := c.Param("id")
	userID := c.GetString("userID")
	userName, _ := c.Get("userName")
	userNameStr, _ := userName.(string)

	var body appealCommentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "komentar wajib diisi"})
		return
	}

	appeal, err := h.svc.CreateAppeal(c.Request.Context(), reviewID, userID, userNameStr, body.Comment)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusCreated, appeal)
}

// GetAppeal handles GET /api/reviews/appeals/:appealId
func (h *AppealHandler) GetAppeal(c *gin.Context) {
	appealID := c.Param("appealId")
	userID := c.GetString("userID")

	detail, err := h.svc.GetAppeal(c.Request.Context(), appealID, userID)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// RespondAppeal handles POST /api/reviews/appeals/:appealId/respond
func (h *AppealHandler) RespondAppeal(c *gin.Context) {
	appealID := c.Param("appealId")
	userID := c.GetString("userID")

	var body appealCommentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "komentar wajib diisi"})
		return
	}

	appeal, err := h.svc.RespondAppeal(c.Request.Context(), appealID, userID, body.Comment)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, appeal)
}

func writeAppealError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrReviewNotFound), errors.Is(err, service.ErrAppealNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNotAppellant), errors.Is(err, service.ErrNotParticipant), errors.Is(err, service.ErrNotReviewer):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEmptyComment):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrAlreadyAppealed), errors.Is(err, service.ErrAlreadyResponded), errors.Is(err, service.ErrAppealResolved):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
