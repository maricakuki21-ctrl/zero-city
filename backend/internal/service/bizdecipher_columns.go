package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCreatorColumnInvalid     = infraerrors.BadRequest("CREATOR_COLUMN_INVALID", "Invalid column or article input")
	ErrCreatorColumnNotFound    = infraerrors.NotFound("CREATOR_COLUMN_NOT_FOUND", "Column or article not found")
	ErrCreatorColumnForbidden   = infraerrors.Forbidden("CREATOR_COLUMN_FORBIDDEN", "Only the owner of an active column may write")
	ErrCreatorColumnExists      = infraerrors.Conflict("CREATOR_COLUMN_EXISTS", "You already have a column")
	ErrCreatorColumnUnavailable = infraerrors.ServiceUnavailable("CREATOR_COLUMN_UNAVAILABLE", "Columns unavailable")
)

type CreatorColumn struct {
	ID               int64     `json:"id"`
	OwnerUserID      int64     `json:"owner_user_id"`
	AuthorName       string    `json:"author_name"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
	ModerationReason string    `json:"moderation_reason"`
	Mode             string    `json:"mode"`
	Price            string    `json:"price"`
	CanRead          bool      `json:"can_read"`
	ViewerPurchaseID int64     `json:"viewer_purchase_id"`
	ArticleCount     int       `json:"article_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreatorColumnArticle struct {
	ID           int64     `json:"id"`
	ColumnID     int64     `json:"column_id"`
	AuthorUserID int64     `json:"author_user_id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Body         string    `json:"body,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreatorColumnInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}
type CreatorColumnArticleInput struct {
	Title   *string `json:"title"`
	Summary *string `json:"summary"`
	Body    *string `json:"body"`
	Status  *string `json:"status"`
}
type CreatorColumnQuery struct {
	ViewerID int64
	Mine     bool
	Admin    bool
	Cursor   int64
	Limit    int
}

type creatorColumnRepository interface {
	ListCreatorColumns(context.Context, CreatorColumnQuery) ([]CreatorColumn, error)
	GetCreatorColumn(context.Context, int64, CreatorColumnQuery) (*CreatorColumn, error)
	CreateCreatorColumn(context.Context, int64, CreatorColumnInput) (*CreatorColumn, error)
	UpdateCreatorColumn(context.Context, int64, int64, CreatorColumnInput) (*CreatorColumn, error)
	ModerateCreatorColumn(context.Context, int64, string, string) (*CreatorColumn, error)
	ListCreatorColumnArticles(context.Context, int64, CreatorColumnQuery) ([]CreatorColumnArticle, error)
	GetCreatorColumnArticle(context.Context, int64, int64, CreatorColumnQuery) (*CreatorColumnArticle, error)
	SaveCreatorColumnArticle(context.Context, int64, int64, int64, CreatorColumnArticleInput) (*CreatorColumnArticle, error)
}

func (s *BizDecipherService) columnRepository() (creatorColumnRepository, error) {
	if s != nil && s.repo != nil {
		if repo, ok := s.repo.(creatorColumnRepository); ok {
			return repo, nil
		}
	}
	return nil, ErrCreatorColumnUnavailable
}
func validateColumnQuery(q CreatorColumnQuery) (CreatorColumnQuery, error) {
	if q.Cursor < 0 || q.Limit < 0 || q.Limit > 50 || (q.Mine && q.ViewerID <= 0) {
		return q, ErrCreatorColumnInvalid
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	return q, nil
}
func columnText(value *string, max int, required bool) bool {
	if value == nil {
		return !required
	}
	*value = strings.TrimSpace(*value)
	return len([]rune(*value)) <= max && (!required || *value != "")
}
func (s *BizDecipherService) ListCreatorColumns(ctx context.Context, q CreatorColumnQuery) ([]CreatorColumn, error) {
	q, err := validateColumnQuery(q)
	if err != nil {
		return nil, err
	}
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.ListCreatorColumns(ctx, q)
}
func (s *BizDecipherService) GetCreatorColumn(ctx context.Context, id int64, q CreatorColumnQuery) (*CreatorColumn, error) {
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.GetCreatorColumn(ctx, id, q)
}
func (s *BizDecipherService) SaveCreatorColumn(ctx context.Context, id, userID int64, input CreatorColumnInput) (*CreatorColumn, error) {
	if userID <= 0 || !columnText(input.Title, 120, id == 0 || input.Title != nil) || !columnText(input.Description, 2000, false) {
		return nil, ErrCreatorColumnInvalid
	}
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return r.CreateCreatorColumn(ctx, userID, input)
	}
	return r.UpdateCreatorColumn(ctx, id, userID, input)
}
func (s *BizDecipherService) ModerateCreatorColumn(ctx context.Context, id int64, status, reason string) (*CreatorColumn, error) {
	reason = strings.TrimSpace(reason)
	if (status != "active" && status != "suspended") || reason == "" || len([]rune(reason)) > 2000 {
		return nil, ErrCreatorColumnInvalid
	}
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.ModerateCreatorColumn(ctx, id, status, reason)
}
func (s *BizDecipherService) ListCreatorColumnArticles(ctx context.Context, id int64, q CreatorColumnQuery) ([]CreatorColumnArticle, error) {
	q, err := validateColumnQuery(q)
	if err != nil {
		return nil, err
	}
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.ListCreatorColumnArticles(ctx, id, q)
}
func (s *BizDecipherService) GetCreatorColumnArticle(ctx context.Context, id, articleID int64, q CreatorColumnQuery) (*CreatorColumnArticle, error) {
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.GetCreatorColumnArticle(ctx, id, articleID, q)
}
func (s *BizDecipherService) SaveCreatorColumnArticle(ctx context.Context, id, articleID, userID int64, input CreatorColumnArticleInput) (*CreatorColumnArticle, error) {
	if userID <= 0 || !columnText(input.Title, 180, articleID == 0 || input.Title != nil) || !columnText(input.Summary, 2000, false) || !columnText(input.Body, 100000, articleID == 0 || input.Body != nil) {
		return nil, ErrCreatorColumnInvalid
	}
	if input.Status != nil && *input.Status != "draft" && *input.Status != "published" && (*input.Status != "archived" || articleID == 0) {
		return nil, ErrCreatorColumnInvalid
	}
	r, err := s.columnRepository()
	if err != nil {
		return nil, err
	}
	return r.SaveCreatorColumnArticle(ctx, id, articleID, userID, input)
}
