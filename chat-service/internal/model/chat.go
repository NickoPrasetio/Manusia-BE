package model

import "time"

type Conversation struct {
	ID              string    `db:"id"                json:"id"`
	User1ID         string    `db:"user1_id"           json:"user1Id"`
	User1Name       string    `db:"user1_name"         json:"user1Name"`
	User1Avatar     string    `db:"user1_avatar"       json:"user1Avatar"`
	User2ID         string    `db:"user2_id"           json:"user2Id"`
	User2Name       string    `db:"user2_name"         json:"user2Name"`
	User2Avatar     string    `db:"user2_avatar"       json:"user2Avatar"`
	LastMessage     string    `db:"last_message"       json:"lastMessage"`
	LastMessageAt   time.Time `db:"last_message_at"    json:"lastMessageAt"`
	CreatedAt       time.Time `db:"created_at"         json:"createdAt"`
}

// ConversationSummary is the conversation as seen from one participant's
// point of view — "other" fields are resolved relative to the caller.
type ConversationSummary struct {
	ConversationID string    `json:"conversationId"`
	OtherUserID    string    `json:"otherUserId"`
	OtherUserName  string    `json:"otherUserName"`
	OtherAvatar    string    `json:"otherAvatar"`
	LastMessage    string    `json:"lastMessage"`
	LastMessageAt  time.Time `json:"lastMessageAt"`
	UnreadCount    int       `json:"unreadCount"`
}

type Message struct {
	ID             string     `db:"id"              json:"id"`
	ConversationID string     `db:"conversation_id" json:"conversationId"`
	SenderID       string     `db:"sender_id"        json:"senderId"`
	Content        string     `db:"content"          json:"content"`
	PhotoURL       string     `db:"photo_url"        json:"photoUrl"`
	ReadAt         *time.Time `db:"read_at"          json:"readAt"`
	CreatedAt      time.Time  `db:"created_at"       json:"createdAt"`
}

type MessagePage struct {
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
	Last     bool       `json:"last"`
}
