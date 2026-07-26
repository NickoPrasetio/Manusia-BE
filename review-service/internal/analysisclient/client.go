// Package analysisclient calls the review-analysis-thinking Python
// microservice, which owns the actual Claude API call for moderating
// review appeals.
package analysisclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/manusia/review-service/internal/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type moderateReq struct {
	ReviewRating     int    `json:"reviewRating"`
	ReviewComment    string `json:"reviewComment"`
	AppellantComment string `json:"appellantComment"`
	ReviewerComment  string `json:"reviewerComment"`
}

type moderateResp struct {
	Verdict   string `json:"verdict"`
	Reasoning string `json:"reasoning"`
}

// Moderate implements service.AppealModerator by delegating the actual
// Claude API call to review-analysis-thinking.
func (c *Client) Moderate(ctx context.Context, review model.Review, appellantComment, reviewerComment string) (string, string, error) {
	body := moderateReq{
		ReviewRating:     review.Rating,
		ReviewComment:    review.Comment,
		AppellantComment: appellantComment,
		ReviewerComment:  reviewerComment,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/moderate", bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("review-analysis-thinking request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("review-analysis-thinking returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out moderateResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", "", err
	}
	return out.Verdict, out.Reasoning, nil
}
