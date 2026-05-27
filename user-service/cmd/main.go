package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/user-service/internal/config"
	"github.com/manusia/user-service/internal/handler"
	"github.com/manusia/user-service/internal/middleware"
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
			protected.PATCH("/users/me/availability", h.UpdateMyAvailability)
		}
	}

	log.Printf("user-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
