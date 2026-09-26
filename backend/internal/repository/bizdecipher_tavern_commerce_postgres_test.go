package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTavernCommercePostgres(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"), "disposable database only")
	require.NoError(t, ApplyMigrations(ctx, db))
	r := &bizDecipherRepository{db: db}
	user := func(name string) int64 {
		id := insertMarketplaceTestUser(t, db, fmt.Sprintf("tavern-commerce-%s-%d@example.test", name, time.Now().UnixNano()))
		_, e := db.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, id)
		require.NoError(t, e)
		return id
	}
	author, host, buyer, other, admin := user("author"), user("host"), user("buyer"), user("other"), user("admin")
	_, err = db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin)
	require.NoError(t, err)
	var script, pkg int64
	err = db.QueryRowContext(ctx, `INSERT INTO tavern_scripts(user_id,title,slug,summary,description,status,visibility,player_min,player_max,estimated_minutes,difficulty)
 VALUES($1,'Ticket story',$2,'Summary','Story','listed','public',1,6,60,'normal') RETURNING id`, author, fmt.Sprintf("ticket-story-%d", time.Now().UnixNano())).Scan(&script)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `INSERT INTO tavern_game_packages(script_id,owner_user_id,version,schema_version,runtime_kind,protocol_version,status,manifest)
 VALUES($1,$2,'v1','tavern.package.v1','declarative','2026-09-13.package.v1','published','{}') RETURNING id`, script, author).Scan(&pkg)
	require.NoError(t, err)
	newRoom := func() *service.TavernRoom {
		room, e := r.CreateTavernRoom(ctx, host, service.TavernRoomInput{ScriptID: script, PackageID: pkg, Title: "Ticket room", Visibility: "public", HostMode: "human_host", BillingMode: "free", MaxPlayers: 6, RoomConfig: map[string]any{}})
		require.NoError(t, e)
		room, e = r.OpenTavernRoom(ctx, room.ID, host)
		require.NoError(t, e)
		return room
	}
	legacy := newRoom()
	require.Nil(t, legacy.TicketPriceUSD)
	_, err = r.SetTavernScriptPricing(ctx, script, other, service.TavernPricingInput{Price: "10.12345678", Currency: "USD"})
	require.Error(t, err)
	_, err = r.SetTavernScriptPricing(ctx, script, author, service.TavernPricingInput{Price: "10.12345678", Currency: "USD"})
	require.NoError(t, err)
	room := newRoom()
	require.NotNil(t, room.TicketPriceUSD)
	require.Equal(t, "10.12345678", *room.TicketPriceUSD)
	_, err = r.SetTavernScriptPricing(ctx, script, author, service.TavernPricingInput{Price: "20", Currency: "USD"})
	require.NoError(t, err)
	q, err := r.GetTavernTicketQuote(ctx, room.ID, buyer)
	require.NoError(t, err)
	require.Equal(t, "10.12345678", q.Price)
	require.False(t, q.Enabled)
	_, err = r.JoinTavernRoomTx(ctx, room.ID, buyer)
	require.ErrorIs(t, err, service.ErrTavernCommerceInvalid)
	in := service.TavernTicketInput{ExpectedPrice: "10.12345678", Currency: "USD", OperationID: "tavern_ticket_purchase_001"}
	_, err = r.PurchaseTavernTicket(ctx, room.ID, buyer, in)
	require.ErrorIs(t, err, service.ErrTavernCommerceClosed)
	_, err = r.SetTavernCommercePolicy(ctx, other, true, "not admin")
	require.Error(t, err)
	_, err = r.SetTavernCommercePolicy(ctx, admin, true, "isolated test")
	require.NoError(t, err)
	_, err = r.PurchaseTavernTicket(ctx, room.ID, author, in)
	require.ErrorIs(t, err, service.ErrTavernCommerceForbidden)
	stale := in
	stale.ExpectedPrice = "1"
	_, err = r.PurchaseTavernTicket(ctx, room.ID, buyer, stale)
	require.Error(t, err)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = r.PurchaseTavernTicket(ctx, room.ID, buyer, in) }(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	q, err = r.GetTavernTicketQuote(ctx, room.ID, buyer)
	require.NoError(t, err)
	ticketID := q.Ticket.ID
	var balance string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "89.87654322", balance)
	joined, err := r.IsTavernRoomParticipant(ctx, room.ID, buyer)
	require.NoError(t, err)
	require.True(t, joined)
	var wallets int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM shared_pool_owner_wallets WHERE owner_id=$1`, author).Scan(&wallets))
	require.Zero(t, wallets)
	// A player refund races an owner start: exactly one financially valid result.
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i == 0 {
				_, errs[i] = r.StartTavernRoom(ctx, room.ID, host)
			} else {
				_, errs[i] = r.RefundTavernTicket(ctx, room.ID, buyer, ticketID)
			}
		}(i)
	}
	wg.Wait()
	require.True(t, (errs[0] == nil) != (errs[1] == nil))
	q, err = r.GetTavernTicketQuote(ctx, room.ID, buyer)
	require.NoError(t, err)
	require.Contains(t, []string{"released", "refunded"}, q.Ticket.Status)
	if q.Ticket.Status == "released" {
		var available string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, author).Scan(&available))
		require.Equal(t, "10.123456780000", available)
		_, err = r.RefundTavernTicket(ctx, room.ID, buyer, ticketID)
		require.Error(t, err)
		_, err = r.StartTavernRoom(ctx, room.ID, host)
		require.NoError(t, err)
	} else {
		require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
		require.Equal(t, "100.00000000", balance)
		_, err = r.RefundTavernTicket(ctx, room.ID, buyer, ticketID)
		require.NoError(t, err)
	}
	cancelled := newRoom()
	rejoinRoom := newRoom()
	rejoinBuyer := user("rejoin")
	firstTicket, err := r.PurchaseTavernTicket(ctx, rejoinRoom.ID, rejoinBuyer, service.TavernTicketInput{ExpectedPrice: "20", Currency: "USD", OperationID: "tavern_ticket_rejoin_001"})
	require.NoError(t, err)
	_, err = r.RefundTavernTicket(ctx, rejoinRoom.ID, rejoinBuyer, firstTicket.ID)
	require.NoError(t, err)
	secondTicket, err := r.PurchaseTavernTicket(ctx, rejoinRoom.ID, rejoinBuyer, service.TavernTicketInput{ExpectedPrice: "20", Currency: "USD", OperationID: "tavern_ticket_rejoin_002"})
	require.NoError(t, err)
	require.NotEqual(t, firstTicket.ID, secondTicket.ID)
	_, err = r.RefundTavernTicket(ctx, rejoinRoom.ID, rejoinBuyer, firstTicket.ID)
	require.NoError(t, err)
	q, err = r.GetTavernTicketQuote(ctx, rejoinRoom.ID, rejoinBuyer)
	require.NoError(t, err)
	require.Equal(t, secondTicket.ID, q.Ticket.ID)
	require.Equal(t, "held", q.Ticket.Status)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, rejoinBuyer).Scan(&balance))
	require.Equal(t, "80.00000000", balance)
	_, err = r.CancelTavernRoom(ctx, rejoinRoom.ID, host)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, rejoinBuyer).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	settledRoom := newRoom()
	settledBuyer := user("settled")
	_, err = r.PurchaseTavernTicket(ctx, settledRoom.ID, settledBuyer, service.TavernTicketInput{ExpectedPrice: "20", Currency: "USD", OperationID: "tavern_ticket_settlement_003"})
	require.NoError(t, err)
	_, err = r.StartTavernRoom(ctx, settledRoom.ID, host)
	require.NoError(t, err)
	_, err = r.StartTavernRoom(ctx, settledRoom.ID, host)
	require.NoError(t, err)
	_, err = r.CancelTavernRoom(ctx, settledRoom.ID, host)
	require.Error(t, err)
	q, err = r.GetTavernTicketQuote(ctx, settledRoom.ID, settledBuyer)
	require.NoError(t, err)
	require.Equal(t, "released", q.Ticket.Status)
	var reconciled bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT w.available_amount=(SELECT COALESCE(SUM(amount),0) FROM tavern_tickets WHERE author_user_id=$1 AND status='released') FROM shared_pool_owner_wallets w WHERE w.owner_id=$1`, author).Scan(&reconciled))
	require.True(t, reconciled)
	input2 := service.TavernTicketInput{ExpectedPrice: "20", Currency: "USD", OperationID: "tavern_ticket_purchase_002"}
	_, err = r.PurchaseTavernTicket(ctx, cancelled.ID, other, input2)
	require.NoError(t, err)
	_, err = r.CancelTavernRoom(ctx, cancelled.ID, buyer)
	require.Error(t, err)
	_, err = r.SetTavernCommercePolicy(ctx, admin, false, "disable new tickets")
	require.NoError(t, err)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = r.CancelTavernRoom(ctx, cancelled.ID, host) }(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, other).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	q, err = r.GetTavernTicketQuote(ctx, cancelled.ID, other)
	require.NoError(t, err)
	require.Equal(t, "refunded", q.Ticket.Status)
	joined, err = r.IsTavernRoomParticipant(ctx, cancelled.ID, other)
	require.NoError(t, err)
	require.False(t, joined)
	// Real legacy room remains free after its author changes new-room pricing.
	_, err = r.JoinTavernRoomTx(ctx, legacy.ID, other)
	require.NoError(t, err)
	_, err = r.StartTavernRoom(ctx, legacy.ID, host)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM user_balance_ledger WHERE user_id=$1 AND source_type='tavern_ticket_refund'`, other).Scan(&count))
	require.Equal(t, 1, count)
}
