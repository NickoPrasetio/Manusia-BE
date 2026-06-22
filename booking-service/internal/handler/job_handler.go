package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/service"
)

type JobHandler struct {
	svc *service.JobService
}

func NewJobHandler(svc *service.JobService) *JobHandler {
	return &JobHandler{svc: svc}
}

// POST /api/jobs
// Customer membuat job posting baru.
func (h *JobHandler) Create(c *gin.Context) {
	customerID := c.GetString("userID")
	var req model.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	j, err := h.svc.Create(c.Request.Context(), customerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, j)
}

// GET /api/jobs/my
// Customer melihat daftar job posting miliknya.
func (h *JobHandler) GetMy(c *gin.Context) {
	customerID := c.GetString("userID")
	jobs, err := h.svc.GetByCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// GET /api/jobs/:id
func (h *JobHandler) GetByID(c *gin.Context) {
	j, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, j)
}

// PATCH /api/jobs/:id/close
// Customer menutup job posting (tidak menerima pelamar baru).
func (h *JobHandler) Close(c *gin.Context) {
	customerID := c.GetString("userID")
	j, err := h.svc.Close(c.Request.Context(), c.Param("id"), customerID)
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, j)
}

// GET /api/jobs/nearby?lat=&lon=&radius=&category=&page=&limit=
// Worker melihat job terbuka di sekitar lokasinya (paginated).
func (h *JobHandler) GetNearby(c *gin.Context) {
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lon, err2 := strconv.ParseFloat(c.Query("lon"), 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param lat dan lon wajib"})
		return
	}

	radius := 50.0
	if rv := c.Query("radius"); rv != "" {
		if v, err := strconv.ParseFloat(rv, 64); err == nil && v > 0 {
			radius = v
		}
	}

	category := c.Query("category")

	page := 1
	if pv := c.Query("page"); pv != "" {
		if v, err := strconv.Atoi(pv); err == nil && v > 0 {
			page = v
		}
	}

	limit := 10
	if lv := c.Query("limit"); lv != "" {
		if v, err := strconv.Atoi(lv); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	offset := (page - 1) * limit

	jobs, total, err := h.svc.GetNearby(c.Request.Context(), lat, lon, radius, category, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.NearbyJobsResponse{
		Jobs:    jobs,
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: int64(offset+len(jobs)) < total,
	})
}

// POST /api/jobs/:id/apply
// Worker menawarkan diri untuk job posting.
func (h *JobHandler) ApplyToJob(c *gin.Context) {
	workerID := c.GetString("userID")
	jobID := c.Param("id")

	var req struct {
		WorkerName   string `json:"workerName"`
		WorkerAvatar string `json:"workerAvatar"`
		Notes        string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)

	b, err := h.svc.ApplyToJob(c.Request.Context(), service.ApplyToJobInput{
		JobID:        jobID,
		WorkerID:     workerID,
		WorkerName:   req.WorkerName,
		WorkerAvatar: req.WorkerAvatar,
		Notes:        req.Notes,
	})
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusCreated, b)
}

func writeJobError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrJobNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrJobForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrJobAlreadyClosed):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrJobNotOpen), errors.Is(err, service.ErrJobSelfApply):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
