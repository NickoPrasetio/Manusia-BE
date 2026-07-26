package model

import "time"

type Appeal struct {
	ID                   string     `db:"id"                     json:"id"`
	ReviewID             string     `db:"review_id"              json:"reviewId"`
	AppellantID          string     `db:"appellant_id"           json:"appellantId"`
	AppellantName        string     `db:"appellant_name"         json:"appellantName"`
	ReviewerID           string     `db:"reviewer_id"            json:"reviewerId"`
	ReviewerName         string     `db:"reviewer_name"          json:"reviewerName"`
	AppellantComment     string     `db:"appellant_comment"      json:"appellantComment"`
	ReviewerComment      string     `db:"reviewer_comment"       json:"reviewerComment"`
	Status               string     `db:"status"                 json:"status"`
	AIVerdict            string     `db:"ai_verdict"             json:"aiVerdict"`
	AIReasoning          string     `db:"ai_reasoning"           json:"aiReasoning"`
	ConversationID       string     `db:"conversation_id"        json:"conversationId"`
	DeadlineAt           time.Time  `db:"deadline_at"            json:"deadlineAt"`
	ReviewerRespondedAt  *time.Time `db:"reviewer_responded_at"  json:"reviewerRespondedAt"`
	ResolvedAt           *time.Time `db:"resolved_at"            json:"resolvedAt"`
	CreatedAt            time.Time  `db:"created_at"             json:"createdAt"`
}

const (
	AppealStatusPending  = "pending"
	AppealStatusResolved = "resolved"
)

const (
	VerdictReviewValid   = "ulasan_valid"
	VerdictReviewUnfair  = "ulasan_tidak_adil"
	VerdictInconclusive  = "tidak_konklusif"
)

// AppealDetail bundles an appeal with the review it disputes, for the
// appeal detail page (both parties need to see the original review).
type AppealDetail struct {
	Appeal
	Review Review `json:"review"`
}
