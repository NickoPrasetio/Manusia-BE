package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/user-service/internal/config"
	"github.com/manusia/user-service/internal/handler"
	"github.com/manusia/user-service/internal/middleware"
	"github.com/manusia/user-service/internal/model"
	"github.com/manusia/user-service/internal/repository"
	"github.com/manusia/user-service/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	repo := repository.NewProfileRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	svc, err := service.NewUserService(repo, cfg)
	if err != nil {
		log.Fatalf("service init: %v", err)
	}

	h := handler.NewUserHandler(svc)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))
	r.Use(metricsMiddleware("user-service"))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "user-service"})
	})

	api := r.Group("/api")
	{
		// Public routes
		api.GET("/users", h.ListAll)
		api.GET("/users/page", h.ListPage)
		api.GET("/users/:id", h.GetByID)
		// Alias "workers" for frontend compatibility
		api.GET("/workers", h.ListAll)
		api.GET("/workers/page", h.ListPage)
		api.GET("/workers/:id", h.GetByID)

		// Internal route (called by review-service)
		api.PATCH("/internal/users/:authId/rating", h.UpdateRating)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth([]byte(cfg.JWTSecret)))
		{
			protected.POST("/users", h.Create)
			protected.PUT("/users/:id", h.Update)
			protected.POST("/users/:id/photo", h.UploadPhoto)
			protected.POST("/workers", h.Create)
			protected.PUT("/workers/:id", h.Update)
			protected.POST("/workers/:id/photo", h.UploadPhoto)
			protected.GET("/users/me/profile", func(c *gin.Context) {
				authID := c.GetString("userID")
				p, err := svc.GetByAuthID(c.Request.Context(), authID)
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, p)
			})
			// PUT /api/users/me/profile — upsert own bio profile
			protected.PUT("/users/me/profile", func(c *gin.Context) {
				authID := c.GetString("userID")
				var req model.UpdateProfileRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				existing, err := svc.GetByAuthID(c.Request.Context(), authID)
				if err != nil {
					// Profile not yet created — create a minimal one first
					createReq := &model.CreateProfileRequest{
						AuthID: authID,
						Name:   req.Name,
					}
					existing, err = svc.Create(c.Request.Context(), createReq)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
				}
				p, err := svc.Update(c.Request.Context(), existing.ID, &req)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, p)
			})
			// POST /api/users/me/photo — upload own avatar
			protected.POST("/users/me/photo", func(c *gin.Context) {
				authID := c.GetString("userID")
				existing, err := svc.GetByAuthID(c.Request.Context(), authID)
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "profil belum dibuat, simpan profil terlebih dahulu"})
					return
				}
				file, header, err := c.Request.FormFile("file")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "file tidak ditemukan"})
					return
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal baca file"})
					return
				}
				p, err := svc.UploadPhotoBytes(c.Request.Context(), existing.ID, data, header.Filename, header.Header.Get("Content-Type"))
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, p)
			})
			protected.PATCH("/users/me/availability", h.UpdateMyAvailability)
		}
	}

	log.Printf("user-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
