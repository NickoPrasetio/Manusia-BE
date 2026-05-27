package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/repository"
)

type JobHandler struct {
	repo *repository.JobRepository
}

func NewJobHandler(db *pgxpool.Pool) *JobHandler {
	return &JobHandler{repo: repository.NewJobRepository(db)}
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

	j := &model.Job{
		CustomerID:   customerID,
		CustomerName: req.CustomerName,
		Title:        req.Title,
		Description:  req.Description,
		BudgetPerDay: req.BudgetPerDay,
		TodoList:     req.TodoList,
		DurationDays: req.DurationDays,
		City:         req.City,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		Category:     req.Category,
		Status:       model.JobStatusOpen,
	}
	if j.TodoList == nil {
		j.TodoList = []string{}
	}

	if err := h.repo.Create(c.Request.Context(), j); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, j)
}

// GET /api/jobs/my
// Customer melihat daftar job posting miliknya.
func (h *JobHandler) GetMy(c *gin.Context) {
	customerID := c.GetString("userID")
	jobs, err := h.repo.FindByCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []model.Job{}
	}
	c.JSON(http.StatusOK, jobs)
}

// GET /api/jobs/:id
func (h *JobHandler) GetByID(c *gin.Context) {
	j, err := h.repo.FindByID(c.Request.Context(), c.Param("id"))
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
	j, err := h.repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if j.CustomerID != customerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "bukan pemilik job ini"})
		return
	}
	if j.Status == model.JobStatusClosed {
		c.JSON(http.StatusConflict, gin.H{"error": "job sudah ditutup"})
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), j.ID, model.JobStatusClosed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	j.Status = model.JobStatusClosed
	c.JSON(http.StatusOK, j)
}

// GET /api/jobs/nearby?lat=&lon=&radius=&category=
// Worker melihat job terbuka di sekitar lokasinya.
// Query params:
//   lat, lon  — wajib (koordinat worker)
//   radius    — opsional, km (default 50)
//   category  — opsional: TASK | PROJECT | EVENT (kosong = semua)
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

	category := c.Query("category") // e.g. "TASK", "PROJECT", "EVENT", or ""

	jobs, err := h.repo.FindNearby(c.Request.Context(), lat, lon, radius, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []model.Job{}
	}
	c.JSON(http.StatusOK, jobs)
}
