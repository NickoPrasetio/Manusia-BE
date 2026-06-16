package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/manusia/chat-service/internal/config"
	"github.com/manusia/chat-service/internal/service"
	"github.com/manusia/chat-service/internal/ws"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ChatHandler struct {
	svc      *service.ChatService
	hub      *ws.Hub
	cfg      *config.Config
	minio    *minio.Client
	upgrader websocket.Upgrader
}

func NewChatHandler(svc *service.ChatService, hub *ws.Hub, cfg *config.Config) (*ChatHandler, error) {
	mc, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccess, cfg.MinioSecret, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, _ := mc.BucketExists(ctx, cfg.MinioBucket)
	if !exists {
		_ = mc.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, cfg.MinioBucket)
		_ = mc.SetBucketPolicy(ctx, cfg.MinioBucket, policy)
	}

	return &ChatHandler{
		svc: svc, hub: hub, cfg: cfg, minio: mc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}, nil
}

// ── REST: conversations ───────────────────────────────────────────────────────

// StartConversation handles POST /api/chats/conversations
func (h *ChatHandler) StartConversation(c *gin.Context) {
	myID := c.GetString("userID")
	myName, _ := c.Get("userName")
	myNameStr, _ := myName.(string)

	var body struct {
		TargetUserID     string `json:"targetUserId"`
		TargetUserName   string `json:"targetUserName"`
		TargetUserAvatar string `json:"targetUserAvatar"`
		MyAvatar         string `json:"myAvatar"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetUserId wajib diisi"})
		return
	}

	conv, err := h.svc.GetOrCreateConversation(
		c.Request.Context(), myID, myNameStr, body.MyAvatar,
		body.TargetUserID, body.TargetUserName, body.TargetUserAvatar,
	)
	if err != nil {
		if errors.Is(err, service.ErrSelfChat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

// ListConversations handles GET /api/chats/conversations
func (h *ChatHandler) ListConversations(c *gin.Context) {
	userID := c.GetString("userID")
	list, err := h.svc.ListConversations(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetMessages handles GET /api/chats/conversations/:id/messages?page=&limit=
func (h *ChatHandler) GetMessages(c *gin.Context) {
	userID := c.GetString("userID")
	conversationID := c.Param("id")

	page, limit := 0, 20
	fmt.Sscanf(c.DefaultQuery("page", "0"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("limit", "20"), "%d", &limit)
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if page < 0 {
		page = 0
	}

	result, err := h.svc.GetMessages(c.Request.Context(), conversationID, userID, page, limit)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// MarkRead handles POST /api/chats/conversations/:id/read
func (h *ChatHandler) MarkRead(c *gin.Context) {
	userID := c.GetString("userID")
	conversationID := c.Param("id")

	otherUserID, count, err := h.svc.MarkRead(c.Request.Context(), conversationID, userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	if count > 0 {
		payload, _ := json.Marshal(map[string]string{
			"type": "read_receipt", "conversationId": conversationID, "readBy": userID,
		})
		h.hub.SendToUser(otherUserID, payload)
	}
	c.JSON(http.StatusOK, gin.H{"updated": count})
}

// UnreadCount handles GET /api/chats/unread-count
func (h *ChatHandler) UnreadCount(c *gin.Context) {
	userID := c.GetString("userID")
	count, err := h.svc.UnreadCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// UploadPhoto handles POST /api/chats/upload (multipart, field "photo")
func (h *ChatHandler) UploadPhoto(c *gin.Context) {
	userID := c.GetString("userID")

	fh, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file foto wajib diisi"})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuka file"})
		return
	}
	defer f.Close()
	data, _ := io.ReadAll(f)

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	objName := fmt.Sprintf("chats/%s-%d%s", userID, time.Now().UnixNano(), ext)
	reader := bytes.NewReader(data)
	_, err = h.minio.PutObject(c.Request.Context(), h.cfg.MinioBucket, objName, reader, int64(len(data)),
		minio.PutObjectOptions{ContentType: fh.Header.Get("Content-Type")})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal upload foto"})
		return
	}

	url := fmt.Sprintf("%s/%s/%s", h.cfg.MinioPublicURL, h.cfg.MinioBucket, objName)
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEmptyMessage):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// ── WebSocket ────────────────────────────────────────────────────────────────

type inboundFrame struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	Content        string `json:"content"`
	PhotoURL       string `json:"photoUrl"`
}

// ServeWS handles GET /ws/chat?token=... — upgrades to a WebSocket connection.
func (h *ChatHandler) ServeWS(c *gin.Context) {
	userID := c.GetString("userID")

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := ws.NewClient(h.hub, conn, userID)
	h.hub.Register(userID, client)

	go client.WritePump()
	go client.ReadPump(h.handleInbound)
}

func (h *ChatHandler) handleInbound(senderID string, raw []byte) {
	var frame inboundFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return
	}
	ctx := context.Background()

	switch frame.Type {
	case "message":
		msg, err := h.svc.SendMessage(ctx, frame.ConversationID, senderID, frame.Content, frame.PhotoURL)
		if err != nil {
			h.sendError(senderID, err)
			return
		}
		conv, err := h.svc.GetConversation(ctx, frame.ConversationID)
		if err != nil {
			return
		}
		otherID := conv.User1ID
		if conv.User1ID == senderID {
			otherID = conv.User2ID
		}

		payload, _ := json.Marshal(map[string]interface{}{"type": "message", "message": msg})
		h.hub.SendToUser(senderID, payload)
		h.hub.SendToUser(otherID, payload)

	case "read":
		otherUserID, count, err := h.svc.MarkRead(ctx, frame.ConversationID, senderID)
		if err != nil {
			h.sendError(senderID, err)
			return
		}
		if count > 0 {
			payload, _ := json.Marshal(map[string]string{
				"type": "read_receipt", "conversationId": frame.ConversationID, "readBy": senderID,
			})
			h.hub.SendToUser(otherUserID, payload)
		}
	}
}

func (h *ChatHandler) sendError(userID string, err error) {
	payload, _ := json.Marshal(map[string]string{"type": "error", "error": err.Error()})
	h.hub.SendToUser(userID, payload)
}
