package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/review-service/internal/analysisclient"
	"github.com/manusia/review-service/internal/chatclient"
	"github.com/manusia/review-service/internal/config"
	"github.com/manusia/review-service/internal/handler"
	"github.com/manusia/review-service/internal/middleware"
	"github.com/manusia/review-service/internal/repository"
	"github.com/manusia/review-service/internal/service"
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

	svc := service.NewReviewService(repo, cfg.UserServiceURL)

	h, err := handler.NewReviewHandler(svc, cfg)
	if err != nil {
		log.Fatalf("handler init: %v", err)
	}

	appealRepo := repository.NewAppealRepository(db)
	if err := appealRepo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate appeals: %v", err)
	}
	notifier := chatclient.NewClient(cfg.ChatServiceURL, cfg.FrontendURL)
	moderator := analysisclient.NewClient(cfg.ReviewAnalysisServiceURL)
	appealSvc := service.NewAppealService(appealRepo, repo, notifier, moderator, time.Duration(cfg.AppealDeadlineHours)*time.Hour)
	appealHandler := handler.NewAppealHandler(appealSvc)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))
	r.Use(metricsMiddleware("review-service"))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "review-service"})
	})

	api := r.Group("/api/reviews")
	{
		// Public
		api.GET("/worker/:workerId", h.GetByWorker)
		api.GET("/worker/:workerId/page", h.GetByWorkerPage)
		api.GET("/given/:userId/page", h.GetGivenByUser)

		// Protected
		protected := api.Group("")
		protected.Use(middleware.JWTAuth([]byte(cfg.JWTSecret)))
		{
			protected.POST("/with-photos", h.CreateWithPhotos)
			protected.PATCH("/:id", h.EditReview)
			protected.POST("/:id/appeal", appealHandler.CreateAppeal)
			protected.GET("/appeals/:appealId", appealHandler.GetAppeal)
			protected.POST("/appeals/:appealId/respond", appealHandler.RespondAppeal)
		}
	}

	go func() {
		interval := time.Duration(cfg.ModerationIntervalSeconds) * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := appealSvc.RunModerationCycle(context.Background()); err != nil {
				log.Printf("moderation cycle error: %v", err)
			}
		}
	}()

	log.Printf("review-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
