// Package chatclient calls chat-service's internal API to send an
// automated notification message when a review appeal is filed.
package chatclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL     string
	frontendURL string
	httpClient  *http.Client
}

func NewClient(chatServiceURL, frontendURL string) *Client {
	return &Client{
		baseURL:     chatServiceURL,
		frontendURL: frontendURL,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

type systemMessageReq struct {
	SenderID        string `json:"senderId"`
	SenderName      string `json:"senderName"`
	SenderAvatar    string `json:"senderAvatar"`
	RecipientID     string `json:"recipientId"`
	RecipientName   string `json:"recipientName"`
	RecipientAvatar string `json:"recipientAvatar"`
	Content         string `json:"content"`
}

type systemMessageResp struct {
	ConversationID string `json:"conversationId"`
}

// NotifyAppeal sends the reviewer a chat message (appearing to come from
// the appellant) pointing them to the appeal page.
func (c *Client) NotifyAppeal(ctx context.Context, appellantID, appellantName, reviewerID, reviewerName, appealID string) (string, error) {
	content := fmt.Sprintf(
		"%s mengajukan banding atas ulasan yang kamu berikan. Silakan beri tanggapan di sini: %s/dashboard/appeals/%s",
		appellantName, c.frontendURL, appealID,
	)

	body := systemMessageReq{
		SenderID:      appellantID,
		SenderName:    appellantName,
		RecipientID:   reviewerID,
		RecipientName: reviewerName,
		Content:       content,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := c.baseURL + "/api/internal/chats/system-message"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat-service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out systemMessageResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return out.ConversationID, nil
}
