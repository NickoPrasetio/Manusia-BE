package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/review-service/internal/config"
	"github.com/manusia/review-service/internal/handler"
	"github.com/manusia/review-service/internal/middleware"
	"github.com/manusia/review-service/internal/repository"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	repo := repository.NewReviewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	h, err := handler.NewReviewHandler(repo, cfg)
	if err != nil {
		log.Fatalf("handler init: %v", err)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "review-service"})
	})

	api := r.Group("/api/reviews")
	{
		// Public
		api.GET("/worker/:workerId", h.GetByWorker)
		api.GET("/worker/:workerId/page", h.GetByWorkerPage)

		// Protected
		protected := api.Group("")
		protected.Use(middleware.JWTAuth([]byte(cfg.JWTSecret)))
		{
			protected.POST("/with-photos", h.CreateWithPhotos)
		}
	}

	log.Printf("review-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
