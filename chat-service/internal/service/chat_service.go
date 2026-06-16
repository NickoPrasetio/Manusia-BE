package service

import (
	"context"
	"errors"
	"strings"

	"github.com/manusia/chat-service/internal/model"
)

var (
	ErrNotFound     = errors.New("percakapan tidak ditemukan")
	ErrForbidden    = errors.New("kamu bukan peserta percakapan ini")
	ErrEmptyMessage = errors.New("pesan tidak boleh kosong")
	ErrSelfChat     = errors.New("tidak bisa memulai percakapan dengan diri sendiri")
)

// ChatRepository is the persistence contract ChatService depends on —
// satisfied implicitly by *repository.ChatRepository.
type ChatRepository interface {
	FindConversationBetween(ctx context.Context, userA, userB string) (*model.Conversation, error)
	FindConversationByID(ctx context.Context, id string) (*model.Conversation, error)
	CreateConversation(ctx context.Context, c *model.Conversation) error
	ListForUser(ctx context.Context, userID string) ([]model.Conversation, map[string]int, error)
	CountUnread(ctx context.Context, userID string) (int, error)
	CreateMessage(ctx context.Context, m *model.Message) error
	FindMessages(ctx context.Context, conversationID string, page, limit int) ([]model.Message, int, error)
	MarkRead(ctx context.Context, conversationID, readerID string) (int64, error)
}

type ChatService struct {
	repo ChatRepository
}

func NewChatService(repo ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) GetOrCreateConversation(
	ctx context.Context,
	myID, myName, myAvatar, targetID, targetName, targetAvatar string,
) (*model.Conversation, error) {
	if myID == targetID {
		return nil, ErrSelfChat
	}

	existing, err := s.repo.FindConversationBetween(ctx, myID, targetID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	conv := &model.Conversation{
		User1ID: myID, User1Name: myName, User1Avatar: myAvatar,
		User2ID: targetID, User2Name: targetName, User2Avatar: targetAvatar,
	}
	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *ChatService) ListConversations(ctx context.Context, userID string) ([]model.ConversationSummary, error) {
	convs, unread, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	summaries := make([]model.ConversationSummary, 0, len(convs))
	for _, c := range convs {
		summary := model.ConversationSummary{
			ConversationID: c.ID,
			LastMessage:    c.LastMessage,
			LastMessageAt:  c.LastMessageAt,
			UnreadCount:    unread[c.ID],
		}
		if c.User1ID == userID {
			summary.OtherUserID = c.User2ID
			summary.OtherUserName = c.User2Name
			summary.OtherAvatar = c.User2Avatar
		} else {
			summary.OtherUserID = c.User1ID
			summary.OtherUserName = c.User1Name
			summary.OtherAvatar = c.User1Avatar
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *ChatService) assertParticipant(ctx context.Context, conversationID, userID string) (*model.Conversation, error) {
	conv, err := s.repo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return nil, ErrNotFound
	}
	if conv.User1ID != userID && conv.User2ID != userID {
		return nil, ErrForbidden
	}
	return conv, nil
}

func (s *ChatService) GetMessages(ctx context.Context, conversationID, userID string, page, limit int) (*model.MessagePage, error) {
	if _, err := s.assertParticipant(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	messages, total, err := s.repo.FindMessages(ctx, conversationID, page, limit)
	if err != nil {
		return nil, err
	}
	if messages == nil {
		messages = []model.Message{}
	}
	return &model.MessagePage{
		Messages: messages,
		Total:    total,
		Page:     page,
		Limit:    limit,
		Last:     (page+1)*limit >= total,
	}, nil
}

func (s *ChatService) SendMessage(ctx context.Context, conversationID, senderID, content, photoURL string) (*model.Message, error) {
	if _, err := s.assertParticipant(ctx, conversationID, senderID); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" && photoURL == "" {
		return nil, ErrEmptyMessage
	}

	msg := &model.Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        content,
		PhotoURL:       photoURL,
	}
	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// MarkRead marks all unread messages from the other party as read and returns
// the other participant's user ID (so the caller can push a read-receipt over WS).
func (s *ChatService) MarkRead(ctx context.Context, conversationID, readerID string) (otherUserID string, count int64, err error) {
	conv, err := s.assertParticipant(ctx, conversationID, readerID)
	if err != nil {
		return "", 0, err
	}
	count, err = s.repo.MarkRead(ctx, conversationID, readerID)
	if err != nil {
		return "", 0, err
	}
	if conv.User1ID == readerID {
		otherUserID = conv.User2ID
	} else {
		otherUserID = conv.User1ID
	}
	return otherUserID, count, nil
}

func (s *ChatService) UnreadCount(ctx context.Context, userID string) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

// GetConversation returns the raw conversation row (no participant check) —
// used internally after SendMessage/MarkRead already validated access, to
// resolve who the "other" participant is for WS push delivery.
func (s *ChatService) GetConversation(ctx context.Context, id string) (*model.Conversation, error) {
	return s.repo.FindConversationByID(ctx, id)
}
