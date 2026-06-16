package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/chat-service/internal/config"
	"github.com/manusia/chat-service/internal/handler"
	"github.com/manusia/chat-service/internal/middleware"
	"github.com/manusia/chat-service/internal/repository"
	"github.com/manusia/chat-service/internal/service"
	"github.com/manusia/chat-service/internal/ws"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	repo := repository.NewChatRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	svc := service.NewChatService(repo)
	hub := ws.NewHub()

	h, err := handler.NewChatHandler(svc, hub, cfg)
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
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "chat-service"})
	})

	jwtAuth := middleware.JWTAuth([]byte(cfg.JWTSecret))

	api := r.Group("/api/chats")
	api.Use(jwtAuth)
	{
		api.POST("/conversations", h.StartConversation)
		api.GET("/conversations", h.ListConversations)
		api.GET("/conversations/:id/messages", h.GetMessages)
		api.POST("/conversations/:id/read", h.MarkRead)
		api.GET("/unread-count", h.UnreadCount)
		api.POST("/upload", h.UploadPhoto)
	}

	r.GET("/ws/chat", middleware.WSAuth([]byte(cfg.JWTSecret)), h.ServeWS)

	log.Printf("chat-service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
