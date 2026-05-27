package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/booking-service/internal/config"
	"github.com/manusia/booking-service/internal/handler"
	"github.com/manusia/booking-service/internal/middleware"
	"github.com/manusia/booking-service/internal/repository"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	repo := repository.NewBookingRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	h := handler.NewBookingHandler(db)
	jwtMiddleware := middleware.JWTAuth([]byte(cfg.JWTSecret))

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "booking-service"})
	})

	api := r.Group("/api/bookings")
	api.Use(jwtMiddleware)
	{
		api.GET("/server-time", func(c *gin.Context) { // public enough — still needs auth
			h.ServerTime(c)
		})
		api.POST("", h.Create)
		api.GET("/my", h.GetMy)
		api.GET("/my-orders", h.GetMyOrders)
		api.GET("/:id", h.GetByID)
		api.PATCH("/:id/confirm", h.Confirm)
		api.PATCH("/:id/complete", h.Complete)
		api.PATCH("/:id/cancel", h.Cancel)
	}

	log.Printf("booking-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

// ensure repo is used
var _ = (*repository.BookingRepository)(nil)
