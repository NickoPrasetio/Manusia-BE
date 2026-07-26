package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/manusia/review-service/internal/model"
)

var (
	ErrReviewNotFound   = errors.New("ulasan tidak ditemukan")
	ErrNotAppellant     = errors.New("kamu bukan penerima ulasan ini")
	ErrAlreadyAppealed  = errors.New("ulasan ini sudah pernah dibanding")
	ErrEmptyComment     = errors.New("komentar penjelasan wajib diisi")
	ErrAppealNotFound   = errors.New("banding tidak ditemukan")
	ErrNotParticipant   = errors.New("kamu bukan pihak dalam banding ini")
	ErrNotReviewer      = errors.New("hanya pemberi ulasan yang bisa menanggapi banding ini")
	ErrAlreadyResponded = errors.New("kamu sudah memberi tanggapan untuk banding ini")
	ErrAppealResolved   = errors.New("banding ini sudah selesai dimoderasi")
)

// AppealRepository is the persistence contract AppealService depends on —
// satisfied implicitly by *repository.AppealRepository.
type AppealRepository interface {
	Create(ctx context.Context, a *model.Appeal) error
	GetByID(ctx context.Context, id string) (*model.Appeal, error)
	GetByReviewID(ctx context.Context, reviewID string) (*model.Appeal, error)
	SetConversationID(ctx context.Context, id, conversationID string) error
	SetReviewerComment(ctx context.Context, id, comment string) error
	SetVerdict(ctx context.Context, id, verdict, reasoning string) error
	ListDueForModeration(ctx context.Context, now time.Time) ([]model.Appeal, error)
}

// ReviewReader is the narrow slice of ReviewRepository that AppealService
// needs — reuses *repository.ReviewRepository via duck typing.
type ReviewReader interface {
	FindByID(ctx context.Context, id string) (*model.Review, error)
}

// AppealNotifier sends the reviewer a chat notification when an appeal is
// filed. Implemented by internal/chatclient.Client.
type AppealNotifier interface {
	NotifyAppeal(ctx context.Context, appellantID, appellantName, reviewerID, reviewerName, appealID string) (conversationID string, err error)
}

// AppealModerator asks the AI to render a verdict on a dispute.
type AppealModerator interface {
	Moderate(ctx context.Context, review model.Review, appellantComment, reviewerComment string) (verdict, reasoning string, err error)
}

type AppealService struct {
	repo         AppealRepository
	reviews      ReviewReader
	notifier     AppealNotifier
	moderator    AppealModerator
	deadlineTTL  time.Duration
}

func NewAppealService(repo AppealRepository, reviews ReviewReader, notifier AppealNotifier, moderator AppealModerator, deadlineTTL time.Duration) *AppealService {
	return &AppealService{repo: repo, reviews: reviews, notifier: notifier, moderator: moderator, deadlineTTL: deadlineTTL}
}

func (s *AppealService) CreateAppeal(ctx context.Context, reviewID, appellantID, appellantName, comment string) (*model.Appeal, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, ErrEmptyComment
	}

	review, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	if review.WorkerID != appellantID {
		return nil, ErrNotAppellant
	}

	existing, err := s.repo.GetByReviewID(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyAppealed
	}

	appeal := &model.Appeal{
		ReviewID:         reviewID,
		AppellantID:      appellantID,
		AppellantName:    appellantName,
		ReviewerID:       review.UserID,
		ReviewerName:     review.UserName,
		AppellantComment: comment,
		DeadlineAt:       time.Now().Add(s.deadlineTTL),
	}
	if err := s.repo.Create(ctx, appeal); err != nil {
		return nil, err
	}

	// Best-effort notification — a failure here shouldn't block appeal creation.
	if s.notifier != nil {
		convID, err := s.notifier.NotifyAppeal(ctx, appellantID, appellantName, review.UserID, review.UserName, appeal.ID)
		if err != nil {
			log.Printf("appeal notify failed for appeal %s: %v", appeal.ID, err)
		} else if convID != "" {
			if err := s.repo.SetConversationID(ctx, appeal.ID, convID); err != nil {
				log.Printf("appeal set conversation id failed for appeal %s: %v", appeal.ID, err)
			} else {
				appeal.ConversationID = convID
			}
		}
	}

	return appeal, nil
}

func (s *AppealService) RespondAppeal(ctx context.Context, appealID, reviewerID, comment string) (*model.Appeal, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, ErrEmptyComment
	}

	appeal, err := s.repo.GetByID(ctx, appealID)
	if err != nil {
		return nil, ErrAppealNotFound
	}
	if appeal.ReviewerID != reviewerID {
		return nil, ErrNotReviewer
	}
	if appeal.Status == model.AppealStatusResolved {
		return nil, ErrAppealResolved
	}
	if appeal.ReviewerComment != "" {
		return nil, ErrAlreadyResponded
	}

	if err := s.repo.SetReviewerComment(ctx, appealID, comment); err != nil {
		return nil, err
	}
	appeal.ReviewerComment = comment
	return appeal, nil
}

func (s *AppealService) GetAppeal(ctx context.Context, appealID, requesterID string) (*model.AppealDetail, error) {
	appeal, err := s.repo.GetByID(ctx, appealID)
	if err != nil {
		return nil, ErrAppealNotFound
	}
	if appeal.AppellantID != requesterID && appeal.ReviewerID != requesterID {
		return nil, ErrNotParticipant
	}

	review, err := s.reviews.FindByID(ctx, appeal.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	return &model.AppealDetail{Appeal: *appeal, Review: *review}, nil
}

// RunModerationCycle is invoked periodically by a background ticker: it
// finds appeals ready for a verdict (reviewer responded, or deadline
// passed) and asks the AI moderator to resolve each one.
func (s *AppealService) RunModerationCycle(ctx context.Context) error {
	if s.moderator == nil {
		return nil
	}

	due, err := s.repo.ListDueForModeration(ctx, time.Now())
	if err != nil {
		return err
	}

	for _, appeal := range due {
		review, err := s.reviews.FindByID(ctx, appeal.ReviewID)
		if err != nil {
			log.Printf("moderation: review %s not found for appeal %s: %v", appeal.ReviewID, appeal.ID, err)
			continue
		}

		verdict, reasoning, err := s.moderator.Moderate(ctx, *review, appeal.AppellantComment, appeal.ReviewerComment)
		if err != nil {
			log.Printf("moderation: AI call failed for appeal %s: %v", appeal.ID, err)
			continue
		}

		if err := s.repo.SetVerdict(ctx, appeal.ID, verdict, reasoning); err != nil {
			log.Printf("moderation: set verdict failed for appeal %s: %v", appeal.ID, err)
		}
	}

	return nil
}
