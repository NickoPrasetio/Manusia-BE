package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/auth-service/internal/config"
	"github.com/manusia/auth-service/internal/handler"
	"github.com/manusia/auth-service/internal/middleware"
	"github.com/manusia/auth-service/internal/repository"
	"github.com/manusia/auth-service/internal/service"
)

func main() {
	cfg := config.Load()

	// DB
	db, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	// Migrate
	repo := repository.NewUserRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Services
	svc := service.NewAuthService(repo, cfg.JWTSecret)
	h, err := handler.NewAuthHandler(svc, cfg)
	if err != nil {
		log.Fatalf("handler init: %v", err)
	}

	// Router
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "auth-service"})
	})

	api := r.Group("/api/auth")
	{
		api.POST("/register", h.Register)
		api.POST("/login", h.Login)

		protected := api.Group("")
		protected.Use(middleware.JWTAuth(svc.JWTSecret()))
		{
			protected.GET("/me", h.GetMe)
			protected.PUT("/me", h.UpdateMe)
			protected.POST("/me/photo", h.UploadAvatar)
		}
	}

	log.Printf("auth-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
