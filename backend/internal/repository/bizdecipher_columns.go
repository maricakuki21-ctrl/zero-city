package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const creatorColumnSelect = `SELECT c.id,c.owner_user_id,
 COALESCE(NULLIF(p.display_name,''),NULLIF(u.username,''),'Creator'),
 c.title,c.description,c.status,c.moderation_reason,
 (SELECT COUNT(*) FROM creator_column_articles a WHERE a.column_id=c.id AND a.status='published'),
 c.created_at,c.updated_at,c.mode,c.price::text,
 (c.mode='free' OR c.owner_user_id=$3 OR EXISTS(SELECT 1 FROM creator_column_purchases p WHERE p.column_id=c.id AND p.buyer_user_id=$3 AND p.status='active')),
 COALESCE((SELECT p.id FROM creator_column_purchases p WHERE p.column_id=c.id AND p.buyer_user_id=$3 AND p.status='active'),0)
 FROM creator_columns c
 JOIN users u ON u.id=c.owner_user_id LEFT JOIN biz_profiles p ON p.user_id=c.owner_user_id`

func scanCreatorColumn(row communityPollScanner) (*service.CreatorColumn, error) {
	c := &service.CreatorColumn{}
	err := row.Scan(&c.ID, &c.OwnerUserID, &c.AuthorName, &c.Title, &c.Description, &c.Status, &c.ModerationReason, &c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.Mode, &c.Price, &c.CanRead, &c.ViewerPurchaseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreatorColumnNotFound
	}
	return c, err
}
func (r *bizDecipherRepository) ListCreatorColumns(ctx context.Context, q service.CreatorColumnQuery) ([]service.CreatorColumn, error) {
	rows, err := r.db.QueryContext(ctx, creatorColumnSelect+`
 WHERE ($1 OR c.status='active' OR ($2 AND c.owner_user_id=$3))
 AND (NOT $2 OR c.owner_user_id=$3) AND ($4::bigint=0 OR c.id<$4) ORDER BY c.id DESC LIMIT $5`,
		q.Admin, q.Mine, q.ViewerID, q.Cursor, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.CreatorColumn, 0)
	for rows.Next() {
		c, err := scanCreatorColumn(rows)
		if err != nil {
			return nil, err
		}
		c.CanRead = c.CanRead || q.Admin
		items = append(items, *c)
	}
	return items, rows.Err()
}
func (r *bizDecipherRepository) GetCreatorColumn(ctx context.Context, id int64, q service.CreatorColumnQuery) (*service.CreatorColumn, error) {
	c, err := scanCreatorColumn(r.db.QueryRowContext(ctx, creatorColumnSelect+` WHERE c.id=$1 AND ($2 OR c.status='active' OR c.owner_user_id=$3)`, id, q.Admin, q.ViewerID))
	if err == nil {
		c.CanRead = c.CanRead || q.Admin
	}
	return c, err
}
func (r *bizDecipherRepository) CreateCreatorColumn(ctx context.Context, userID int64, in service.CreatorColumnInput) (*service.CreatorColumn, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO creator_columns(owner_user_id,title,description) VALUES($1,$2,COALESCE($3,'')) RETURNING id`, userID, in.Title, in.Description).Scan(&id)
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return nil, service.ErrCreatorColumnExists
	}
	if err != nil {
		return nil, err
	}
	return r.GetCreatorColumn(ctx, id, service.CreatorColumnQuery{ViewerID: userID})
}
func (r *bizDecipherRepository) UpdateCreatorColumn(ctx context.Context, id, userID int64, in service.CreatorColumnInput) (*service.CreatorColumn, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE creator_columns SET title=COALESCE($3,title),description=COALESCE($4,description),updated_at=NOW() WHERE id=$1 AND owner_user_id=$2 AND status='active'`, id, userID, in.Title, in.Description)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, service.ErrCreatorColumnForbidden
	}
	return r.GetCreatorColumn(ctx, id, service.CreatorColumnQuery{ViewerID: userID})
}
func (r *bizDecipherRepository) ModerateCreatorColumn(ctx context.Context, id int64, status, reason string) (*service.CreatorColumn, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE creator_columns SET status=$2,moderation_reason=$3,updated_at=NOW() WHERE id=$1`, id, status, reason)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, service.ErrCreatorColumnNotFound
	}
	return r.GetCreatorColumn(ctx, id, service.CreatorColumnQuery{Admin: true})
}

const creatorArticleFields = `a.id,a.column_id,a.author_user_id,a.title,a.summary,a.status,a.created_at,a.updated_at`

func scanCreatorArticle(row communityPollScanner, body bool) (*service.CreatorColumnArticle, error) {
	a := &service.CreatorColumnArticle{}
	args := []any{&a.ID, &a.ColumnID, &a.AuthorUserID, &a.Title, &a.Summary, &a.Status, &a.CreatedAt, &a.UpdatedAt}
	if body {
		args = append(args, &a.Body)
	}
	err := row.Scan(args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreatorColumnNotFound
	}
	return a, err
}
func (r *bizDecipherRepository) ListCreatorColumnArticles(ctx context.Context, id int64, q service.CreatorColumnQuery) ([]service.CreatorColumnArticle, error) {
	c, err := r.GetCreatorColumn(ctx, id, q)
	if err != nil {
		return nil, err
	}
	if q.Mine && c.OwnerUserID != q.ViewerID && !q.Admin {
		return nil, service.ErrCreatorColumnForbidden
	}
	// Repeat visibility in the query so a concurrent takedown cannot leak an article.
	rows, err := r.db.QueryContext(ctx, `SELECT `+creatorArticleFields+` FROM creator_column_articles a JOIN creator_columns c ON c.id=a.column_id
 WHERE c.id=$1 AND ($6 OR (c.status='active' AND a.status='published') OR ($2 AND c.owner_user_id=$3))
 AND ($4::bigint=0 OR a.id<$4) ORDER BY a.id DESC LIMIT $5`, id, q.Mine, q.ViewerID, q.Cursor, q.Limit, q.Admin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.CreatorColumnArticle, 0)
	for rows.Next() {
		a, err := scanCreatorArticle(rows, false)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}
func (r *bizDecipherRepository) GetCreatorColumnArticle(ctx context.Context, id, articleID int64, q service.CreatorColumnQuery) (*service.CreatorColumnArticle, error) {
	return scanCreatorArticle(r.db.QueryRowContext(ctx, `SELECT `+creatorArticleFields+`,a.body FROM creator_column_articles a JOIN creator_columns c ON c.id=a.column_id
 WHERE c.id=$1 AND a.id=$2 AND ($4 OR c.owner_user_id=$3 OR
 (c.status='active' AND a.status='published' AND (c.mode='free' OR EXISTS(
 SELECT 1 FROM creator_column_purchases p WHERE p.column_id=c.id AND p.buyer_user_id=$3 AND p.status='active'))))`, id, articleID, q.ViewerID, q.Admin), true)
}
func (r *bizDecipherRepository) SaveCreatorColumnArticle(ctx context.Context, id, articleID, userID int64, in service.CreatorColumnArticleInput) (*service.CreatorColumnArticle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owner int64
	var status string
	// Serialize owner publication and administrator takedown on the column row.
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id,status FROM creator_columns WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreatorColumnNotFound
	}
	if err != nil {
		return nil, err
	}
	if owner != userID || status != "active" {
		return nil, service.ErrCreatorColumnForbidden
	}
	if articleID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO creator_column_articles(column_id,author_user_id,title,summary,body,status)
 VALUES($1,$2,$3,COALESCE($4,''),$5,COALESCE($6,'draft')) RETURNING id`, id, userID, in.Title, in.Summary, in.Body, in.Status).Scan(&articleID)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE creator_column_articles SET title=COALESCE($4,title),summary=COALESCE($5,summary),
 body=COALESCE($6,body),status=COALESCE($7,status),updated_at=NOW() WHERE column_id=$1 AND id=$2 AND author_user_id=$3`, id, articleID, userID, in.Title, in.Summary, in.Body, in.Status)
		if err == nil {
			var n int64
			n, err = result.RowsAffected()
			if err == nil && n == 0 {
				return nil, service.ErrCreatorColumnNotFound
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCreatorColumnArticle(ctx, id, articleID, service.CreatorColumnQuery{ViewerID: userID})
}
