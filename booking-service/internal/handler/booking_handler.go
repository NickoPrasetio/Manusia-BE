package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/service"
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

func (h *BookingHandler) Create(c *gin.Context) {
	customerID := c.GetString("userID")
	var req model.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b, err := h.svc.Create(c.Request.Context(), customerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *BookingHandler) GetMy(c *gin.Context) {
	customerID := c.GetString("userID")
	bookings, err := h.svc.GetByCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) GetMyOrders(c *gin.Context) {
	workerID := c.GetString("userID")
	bookings, err := h.svc.GetByWorker(c.Request.Context(), workerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	userID := c.GetString("userID")
	b, err := h.svc.GetByID(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Confirm(c *gin.Context) {
	userID := c.GetString("userID")
	b, err := h.svc.Confirm(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Complete(c *gin.Context) {
	userID := c.GetString("userID")
	b, err := h.svc.Complete(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	userID := c.GetString("userID")
	b, err := h.svc.Cancel(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

// GetOpenNearby returns PENDING bookings near the given lat/lon.
// Query params: lat, lon (required), radius (km, default 25)
func (h *BookingHandler) GetOpenNearby(c *gin.Context) {
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lon, err2 := strconv.ParseFloat(c.Query("lon"), 64)
	if err1 != nil || err2 != nil || lat == 0 || lon == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param lat dan lon wajib"})
		return
	}
	radius := 25.0
	if rv := c.Query("radius"); rv != "" {
		if v, err := strconv.ParseFloat(rv, 64); err == nil && v > 0 {
			radius = v
		}
	}
	bookings, err := h.svc.GetOpenNearby(c.Request.Context(), lat, lon, radius)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) ServerTime(c *gin.Context) {
	now := time.Now()
	c.JSON(http.StatusOK, gin.H{
		"date":     now.Format("2006-01-02"),
		"dateTime": now.Format(time.RFC3339),
	})
}

func writeBookingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNotPending), errors.Is(err, service.ErrNotConfirmed), errors.Is(err, service.ErrAlreadyFinalized):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
