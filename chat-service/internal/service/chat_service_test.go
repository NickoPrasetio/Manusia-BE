package service_test

import (
	"context"
	"testing"

	"github.com/manusia/chat-service/internal/model"
	"github.com/manusia/chat-service/internal/service"
)

// ── stub repository — implements service.ChatRepository ────────────────────────

type stubRepo struct {
	conversations map[string]*model.Conversation
	messages      map[string][]model.Message
	unreadByConv  map[string]int

	createConvErr error
	createMsgErr  error
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		conversations: map[string]*model.Conversation{},
		messages:      map[string][]model.Message{},
		unreadByConv:  map[string]int{},
	}
}

func (s *stubRepo) FindConversationBetween(_ context.Context, userA, userB string) (*model.Conversation, error) {
	for _, c := range s.conversations {
		if (c.User1ID == userA && c.User2ID == userB) || (c.User1ID == userB && c.User2ID == userA) {
			return c, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) FindConversationByID(_ context.Context, id string) (*model.Conversation, error) {
	c, ok := s.conversations[id]
	if !ok {
		return nil, errNotFoundStub
	}
	return c, nil
}

func (s *stubRepo) CreateConversation(_ context.Context, c *model.Conversation) error {
	if s.createConvErr != nil {
		return s.createConvErr
	}
	c.ID = "conv-" + c.User1ID + "-" + c.User2ID
	s.conversations[c.ID] = c
	return nil
}

func (s *stubRepo) ListForUser(_ context.Context, userID string) ([]model.Conversation, map[string]int, error) {
	var result []model.Conversation
	for _, c := range s.conversations {
		if c.User1ID == userID || c.User2ID == userID {
			result = append(result, *c)
		}
	}
	return result, s.unreadByConv, nil
}

func (s *stubRepo) CountUnread(_ context.Context, userID string) (int, error) {
	count := 0
	for convID, c := range s.conversations {
		if c.User1ID == userID || c.User2ID == userID {
			count += s.unreadByConv[convID]
		}
	}
	return count, nil
}

func (s *stubRepo) CreateMessage(_ context.Context, m *model.Message) error {
	if s.createMsgErr != nil {
		return s.createMsgErr
	}
	m.ID = "msg-1"
	s.messages[m.ConversationID] = append(s.messages[m.ConversationID], *m)
	return nil
}

func (s *stubRepo) FindMessages(_ context.Context, conversationID string, page, limit int) ([]model.Message, int, error) {
	all := s.messages[conversationID]
	total := len(all)
	offset := page * limit
	if offset >= total {
		return []model.Message{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (s *stubRepo) MarkRead(_ context.Context, conversationID, _ string) (int64, error) {
	n := int64(s.unreadByConv[conversationID])
	s.unreadByConv[conversationID] = 0
	return n, nil
}

type stubErr struct{ msg string }

func (e *stubErr) Error() string { return e.msg }

var errNotFoundStub = &stubErr{"not found"}

// ── helpers ───────────────────────────────────────────────────────────────────

func addConversation(repo *stubRepo, id, u1, u2 string) {
	repo.conversations[id] = &model.Conversation{ID: id, User1ID: u1, User1Name: "User1", User2ID: u2, User2Name: "User2"}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestChatService_GetOrCreateConversation(t *testing.T) {
	ctx := context.Background()

	t.Run("creates new conversation when none exists", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewChatService(repo)
		conv, err := svc.GetOrCreateConversation(ctx, "u1", "Andi", "", "u2", "Budi", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conv.User1ID != "u1" || conv.User2ID != "u2" {
			t.Errorf("unexpected conversation: %+v", conv)
		}
	})

	t.Run("returns existing conversation instead of duplicating", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		conv, err := svc.GetOrCreateConversation(ctx, "u1", "Andi", "", "u2", "Budi", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conv.ID != "conv-1" {
			t.Errorf("expected existing conv-1, got %s", conv.ID)
		}
	})

	t.Run("returns existing conversation regardless of side order", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u2", "u1")
		svc := service.NewChatService(repo)
		conv, err := svc.GetOrCreateConversation(ctx, "u1", "Andi", "", "u2", "Budi", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conv.ID != "conv-1" {
			t.Errorf("expected existing conv-1, got %s", conv.ID)
		}
	})

	t.Run("rejects chatting with self", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewChatService(repo)
		_, err := svc.GetOrCreateConversation(ctx, "u1", "Andi", "", "u1", "Andi", "")
		if err != service.ErrSelfChat {
			t.Errorf("expected ErrSelfChat, got %v", err)
		}
	})
}

func TestChatService_SendMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		msg, err := svc.SendMessage(ctx, "conv-1", "u1", "halo", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg.Content != "halo" {
			t.Errorf("unexpected content: %s", msg.Content)
		}
	})

	t.Run("allows photo-only message", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		msg, err := svc.SendMessage(ctx, "conv-1", "u1", "", "http://photo.url/x.jpg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg.PhotoURL == "" {
			t.Errorf("expected photo url to be set")
		}
	})

	t.Run("rejects empty message without photo", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		_, err := svc.SendMessage(ctx, "conv-1", "u1", "   ", "")
		if err != service.ErrEmptyMessage {
			t.Errorf("expected ErrEmptyMessage, got %v", err)
		}
	})

	t.Run("rejects sender who is not a participant", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		_, err := svc.SendMessage(ctx, "conv-1", "stranger", "halo", "")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("returns not found for unknown conversation", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewChatService(repo)
		_, err := svc.SendMessage(ctx, "conv-x", "u1", "halo", "")
		if err != service.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestChatService_MarkRead(t *testing.T) {
	ctx := context.Background()

	t.Run("marks read and returns other participant id", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		repo.unreadByConv["conv-1"] = 3
		svc := service.NewChatService(repo)

		otherID, count, err := svc.MarkRead(ctx, "conv-1", "u2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if otherID != "u1" {
			t.Errorf("expected other user u1, got %s", otherID)
		}
		if count != 3 {
			t.Errorf("expected 3 updated, got %d", count)
		}
	})

	t.Run("rejects non-participant", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		_, _, err := svc.MarkRead(ctx, "conv-1", "stranger")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestChatService_ListConversations(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves other-user fields based on caller side", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		repo.unreadByConv["conv-1"] = 2
		svc := service.NewChatService(repo)

		list, err := svc.ListConversations(ctx, "u2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 conversation, got %d", len(list))
		}
		if list[0].OtherUserID != "u1" {
			t.Errorf("expected other user u1, got %s", list[0].OtherUserID)
		}
		if list[0].UnreadCount != 2 {
			t.Errorf("expected unread 2, got %d", list[0].UnreadCount)
		}
	})

	t.Run("empty when user has no conversations", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewChatService(repo)
		list, err := svc.ListConversations(ctx, "u-lonely")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d", len(list))
		}
	})
}

func TestChatService_GetMessages(t *testing.T) {
	ctx := context.Background()

	t.Run("paginates correctly", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		for i := 0; i < 3; i++ {
			_, _ = svc.SendMessage(ctx, "conv-1", "u1", "msg", "")
		}

		page, err := svc.GetMessages(ctx, "conv-1", "u1", 0, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if page.Total != 3 || len(page.Messages) != 2 || page.Last {
			t.Errorf("unexpected page: %+v", page)
		}
	})

	t.Run("rejects non-participant", func(t *testing.T) {
		repo := newStubRepo()
		addConversation(repo, "conv-1", "u1", "u2")
		svc := service.NewChatService(repo)
		_, err := svc.GetMessages(ctx, "conv-1", "stranger", 0, 10)
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}
