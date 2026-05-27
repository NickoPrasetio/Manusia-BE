package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/repository"
)

type BookingHandler struct {
	repo *repository.BookingRepository
}

func NewBookingHandler(db *pgxpool.Pool) *BookingHandler {
	return &BookingHandler{repo: repository.NewBookingRepository(db)}
}

func (h *BookingHandler) Create(c *gin.Context) {
	customerID := c.GetString("userID")
	var req model.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b := &model.Booking{
		WorkerID:      req.WorkerID,
		WorkerName:    req.WorkerName,
		WorkerAvatar:  req.WorkerAvatar,
		CustomerID:    customerID,
		CustomerName:  req.CustomerName,
		Address:       req.Address,
		City:          req.City,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		BookingDate:   req.BookingDate,
		StartTime:     req.StartTime,
		DurationDays:  req.DurationDays,
		PaymentMethod: req.PaymentMethod,
		Notes:         req.Notes,
		Status:        model.StatusPending,
	}
	if err := h.repo.Create(c.Request.Context(), b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *BookingHandler) GetMy(c *gin.Context) {
	customerID := c.GetString("userID")
	bookings, err := h.repo.FindByCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if bookings == nil {
		bookings = []model.Booking{}
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) GetMyOrders(c *gin.Context) {
	workerID := c.GetString("userID")
	bookings, err := h.repo.FindByWorker(c.Request.Context(), workerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if bookings == nil {
		bookings = []model.Booking{}
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	b, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Confirm(c *gin.Context) {
	id := c.Param("id")
	b, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if b.Status != model.StatusPending {
		c.JSON(http.StatusConflict, gin.H{"error": "hanya booking PENDING yang bisa dikonfirmasi"})
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), id, model.StatusConfirmed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.Status = model.StatusConfirmed
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Complete(c *gin.Context) {
	id := c.Param("id")
	b, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if b.Status != model.StatusConfirmed {
		c.JSON(http.StatusConflict, gin.H{"error": "hanya booking CONFIRMED yang bisa diselesaikan"})
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), id, model.StatusCompleted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.Status = model.StatusCompleted
	c.JSON(http.StatusOK, b)
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	b, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if b.Status == model.StatusCompleted || b.Status == model.StatusCancelled {
		c.JSON(http.StatusConflict, gin.H{"error": "booking tidak bisa dibatalkan"})
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), id, model.StatusCancelled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.Status = model.StatusCancelled
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
	bookings, err := h.repo.FindOpenNearby(c.Request.Context(), lat, lon, radius)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if bookings == nil {
		bookings = []model.Booking{}
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
