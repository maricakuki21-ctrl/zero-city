package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceRepositoryUpdateListing_whenAdminTookListingDown(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &marketplaceRepository{db: db}
	mock.ExpectQuery(regexp.QuoteMeta("WITH target AS (")).
		WithArgs(int64(4), int64(9), "service", nil, "Title", "Summary", "general", "", "", "[]").
		WillReturnRows(sqlmock.NewRows([]string{"id", "outcome"}).AddRow(4, "locked"))

	// When
	_, err = repo.UpdateListing(context.Background(), 4, 9, service.MarketplaceListingInput{
		Kind: "service", Title: "Title", Summary: "Summary", Category: "general",
	})

	// Then
	require.ErrorIs(t, err, service.ErrMarketplaceListingLocked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceRepositoryListMessages_whenUserIsNotParticipant(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &marketplaceRepository{db: db}
	mock.ExpectQuery(`SELECT TRUE FROM marketplace_inquiries i\s+WHERE i.id = \$1 AND \(\$2 = i.initiator_user_id OR \$2 = i.listing_owner_user_id\)`).
		WithArgs(int64(8), int64(99)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}))

	// When
	_, err = repo.ListMessages(context.Background(), 8, 99, service.MarketplacePageQuery{Limit: 20})

	// Then
	require.ErrorIs(t, err, service.ErrMarketplaceInquiryForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceRepositoryPostMessage_whenRetryBodyDiffers(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &marketplaceRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT TRUE FROM marketplace_inquiries i[\s\S]+FOR UPDATE`).
		WithArgs(int64(8), int64(31)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
	mock.ExpectQuery(`SELECT id, inquiry_id, sender_user_id, client_message_id, body, created_at[\s\S]+FROM marketplace_inquiry_messages`).
		WithArgs(int64(8), int64(31), "client-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "inquiry_id", "sender_user_id", "client_message_id", "body", "created_at"}).
			AddRow(51, 8, 31, "client-1", "original", time.Now()))
	mock.ExpectRollback()

	// When
	_, err = repo.PostMessage(context.Background(), 8, 31, service.MarketplaceMessageInput{
		ClientMessageID: "client-1", Body: "different",
	})

	// Then
	require.ErrorIs(t, err, service.ErrMarketplaceMessageIdempotencyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
