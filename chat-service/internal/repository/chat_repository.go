package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/chat-service/internal/model"
)

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS conversations (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user1_id         TEXT NOT NULL,
			user1_name       TEXT NOT NULL,
			user1_avatar     TEXT NOT NULL DEFAULT '',
			user2_id         TEXT NOT NULL,
			user2_name       TEXT NOT NULL,
			user2_avatar     TEXT NOT NULL DEFAULT '',
			last_message     TEXT NOT NULL DEFAULT '',
			last_message_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS messages (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id       TEXT NOT NULL,
			content         TEXT NOT NULL DEFAULT '',
			photo_url       TEXT NOT NULL DEFAULT '',
			read_at         TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_conversations_user1 ON conversations(user1_id);
		CREATE INDEX IF NOT EXISTS idx_conversations_user2 ON conversations(user2_id);
	`)
	return err
}

func (r *ChatRepository) FindConversationBetween(ctx context.Context, userA, userB string) (*model.Conversation, error) {
	var c model.Conversation
	err := r.db.QueryRow(ctx, `
		SELECT id, user1_id, user1_name, user1_avatar, user2_id, user2_name, user2_avatar,
		       last_message, last_message_at, created_at
		FROM conversations
		WHERE (user1_id=$1 AND user2_id=$2) OR (user1_id=$2 AND user2_id=$1)
		LIMIT 1
	`, userA, userB).Scan(
		&c.ID, &c.User1ID, &c.User1Name, &c.User1Avatar, &c.User2ID, &c.User2Name, &c.User2Avatar,
		&c.LastMessage, &c.LastMessageAt, &c.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) FindConversationByID(ctx context.Context, id string) (*model.Conversation, error) {
	var c model.Conversation
	err := r.db.QueryRow(ctx, `
		SELECT id, user1_id, user1_name, user1_avatar, user2_id, user2_name, user2_avatar,
		       last_message, last_message_at, created_at
		FROM conversations WHERE id=$1
	`, id).Scan(
		&c.ID, &c.User1ID, &c.User1Name, &c.User1Avatar, &c.User2ID, &c.User2Name, &c.User2Avatar,
		&c.LastMessage, &c.LastMessageAt, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) CreateConversation(ctx context.Context, c *model.Conversation) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO conversations (user1_id, user1_name, user1_avatar, user2_id, user2_name, user2_avatar)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, last_message, last_message_at, created_at
	`, c.User1ID, c.User1Name, c.User1Avatar, c.User2ID, c.User2Name, c.User2Avatar).
		Scan(&c.ID, &c.LastMessage, &c.LastMessageAt, &c.CreatedAt)
}

// ListForUser returns every conversation the user participates in, newest first,
// together with the unread count (messages sent by the other party, not yet read).
func (r *ChatRepository) ListForUser(ctx context.Context, userID string) ([]model.Conversation, map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user1_id, user1_name, user1_avatar, user2_id, user2_name, user2_avatar,
		       last_message, last_message_at, created_at
		FROM conversations
		WHERE user1_id=$1 OR user2_id=$1
		ORDER BY last_message_at DESC
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var convs []model.Conversation
	for rows.Next() {
		var c model.Conversation
		if err := rows.Scan(
			&c.ID, &c.User1ID, &c.User1Name, &c.User1Avatar, &c.User2ID, &c.User2Name, &c.User2Avatar,
			&c.LastMessage, &c.LastMessageAt, &c.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	unreadRows, err := r.db.Query(ctx, `
		SELECT m.conversation_id, COUNT(*)
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE (c.user1_id=$1 OR c.user2_id=$1) AND m.sender_id != $1 AND m.read_at IS NULL
		GROUP BY m.conversation_id
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer unreadRows.Close()

	unread := map[string]int{}
	for unreadRows.Next() {
		var convID string
		var cnt int
		if err := unreadRows.Scan(&convID, &cnt); err != nil {
			return nil, nil, err
		}
		unread[convID] = cnt
	}
	return convs, unread, unreadRows.Err()
}

func (r *ChatRepository) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE (c.user1_id=$1 OR c.user2_id=$1) AND m.sender_id != $1 AND m.read_at IS NULL
	`, userID).Scan(&count)
	return count, err
}

func (r *ChatRepository) CreateMessage(ctx context.Context, m *model.Message) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, content, photo_url)
		VALUES ($1,$2,$3,$4)
		RETURNING id, created_at
	`, m.ConversationID, m.SenderID, m.Content, m.PhotoURL).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return err
	}

	preview := m.Content
	if preview == "" && m.PhotoURL != "" {
		preview = "📷 Foto"
	}
	_, err = r.db.Exec(ctx, `
		UPDATE conversations SET last_message=$1, last_message_at=$2 WHERE id=$3
	`, preview, m.CreatedAt, m.ConversationID)
	return err
}

func (r *ChatRepository) FindMessages(ctx context.Context, conversationID string, page, limit int) ([]model.Message, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id=$1`, conversationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := page * limit
	rows, err := r.db.Query(ctx, `
		SELECT id, conversation_id, sender_id, content, photo_url, read_at, created_at
		FROM messages WHERE conversation_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, conversationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.PhotoURL, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, total, rows.Err()
}

// MarkRead marks all messages in the conversation sent by someone other than
// readerID as read. Returns the number of rows updated.
func (r *ChatRepository) MarkRead(ctx context.Context, conversationID, readerID string) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE messages SET read_at = NOW()
		WHERE conversation_id=$1 AND sender_id != $2 AND read_at IS NULL
	`, conversationID, readerID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
