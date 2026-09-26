package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const tavernTicketFields = `id,room_id,script_id,buyer_user_id,author_user_id,operation_id,title,amount::text,status,created_at,released_at,refunded_at,refund_reason`

func scanTavernTicket(row scanner) (*service.TavernTicket, error) {
	t := &service.TavernTicket{}
	err := row.Scan(&t.ID, &t.RoomID, &t.ScriptID, &t.BuyerUserID, &t.AuthorUserID, &t.OperationID, &t.Title, &t.Amount, &t.Status, &t.CreatedAt, &t.ReleasedAt, &t.RefundedAt, &t.RefundReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}
func (r *bizDecipherRepository) GetTavernCommercePolicy(ctx context.Context) (*service.TavernCommercePolicy, error) {
	p := &service.TavernCommercePolicy{Currency: "USD"}
	err := r.db.QueryRowContext(ctx, `SELECT enabled FROM tavern_commerce_policy WHERE id=TRUE`).Scan(&p.Enabled)
	return p, err
}
func (r *bizDecipherRepository) SetTavernCommercePolicy(ctx context.Context, actor int64, enabled bool, reason string) (*service.TavernCommercePolicy, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 1000 {
		return nil, service.ErrTavernCommerceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = columnCommerceAdmin(ctx, tx, actor); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tavern_commerce_policy SET enabled=$1,updated_at=NOW() WHERE id=TRUE`, enabled); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tavern_commerce_audit(actor_user_id,action,detail) VALUES($1,'policy',jsonb_build_object('enabled',$2::boolean,'reason',$3::text))`, actor, enabled, reason); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.TavernCommercePolicy{Enabled: enabled, Currency: "USD"}, nil
}
func (r *bizDecipherRepository) SetTavernScriptPricing(ctx context.Context, id, actor int64, input service.TavernPricingInput) (*service.TavernScript, error) {
	in, err := service.NormalizeTavernPricing(input)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := lockColumnCommerceUsers(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	if !users[actor].active {
		return nil, service.ErrTavernCommerceForbidden
	}
	result, err := tx.ExecContext(ctx, `UPDATE tavern_scripts SET entry_price_usd=$3::numeric,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL AND status NOT IN ('delisted','archived','rejected')`, id, actor, in.Price)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, service.ErrTavernCommerceForbidden
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tavern_commerce_audit(actor_user_id,action,detail) VALUES($1,'pricing',jsonb_build_object('script_id',$2::bigint,'price',$3::text))`, actor, id, in.Price); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTavernScript(ctx, id, true)
}
func (r *bizDecipherRepository) GetTavernTicketQuote(ctx context.Context, room, actor int64) (*service.TavernTicketQuote, error) {
	q := &service.TavernTicketQuote{RoomID: room, Currency: "USD"}
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(ticket_price_usd,0)::text,ticket_author_id,owner_id,status,title FROM tavern_rooms
 WHERE id=$1 AND deleted_at IS NULL AND (visibility='public' OR owner_id=$2 OR EXISTS(SELECT 1 FROM tavern_room_players WHERE room_id=$1 AND user_id=$2))`, room, actor).Scan(&q.Price, &q.AuthorUserID, &q.OwnerID, &q.Status, &q.Title)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrTavernCommerceForbidden
	}
	if err != nil {
		return nil, err
	}
	if err = r.db.QueryRowContext(ctx, `SELECT enabled FROM tavern_commerce_policy WHERE id=TRUE`).Scan(&q.Enabled); err != nil {
		return nil, err
	}
	q.Ticket, err = scanTavernTicket(r.db.QueryRowContext(ctx, `SELECT `+tavernTicketFields+` FROM tavern_tickets WHERE room_id=$1 AND buyer_user_id=$2 ORDER BY id DESC LIMIT 1`, room, actor))
	return q, err
}

// The room advisory lock serializes ticket membership before discovering and
// sorting all users. Every ticket/state path takes this before users -> wallet.
func lockTavernTicketRoom(ctx context.Context, tx *sql.Tx, room int64) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(267,hashtext($1::text))`, strconv.FormatInt(room, 10))
	return err
}

type tavernTicketRoom struct {
	owner, author, script            int64
	price, status, title, visibility string
	players, max                     int
}

func readTavernTicketRoom(ctx context.Context, tx *sql.Tx, room int64) (tavernTicketRoom, error) {
	var v tavernTicketRoom
	err := tx.QueryRowContext(ctx, `SELECT owner_id,COALESCE(ticket_author_id,0),script_id,COALESCE(ticket_price_usd,0)::text,status,title,visibility,current_players,max_players
 FROM tavern_rooms WHERE id=$1 AND deleted_at IS NULL`, room).Scan(&v.owner, &v.author, &v.script, &v.price, &v.status, &v.title, &v.visibility, &v.players, &v.max)
	return v, err
}
func (r *bizDecipherRepository) PurchaseTavernTicket(ctx context.Context, room, actor int64, input service.TavernTicketInput) (*service.TavernTicket, error) {
	in, err := service.NormalizeTavernTicket(input)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockTavernTicketRoom(ctx, tx, room); err != nil {
		return nil, err
	}
	v, err := readTavernTicketRoom(ctx, tx, room)
	if err != nil {
		return nil, err
	}
	if v.author == 0 || actor == v.owner || actor == v.author || v.visibility != "public" {
		return nil, service.ErrTavernCommerceForbidden
	}
	users, err := lockColumnCommerceUsers(ctx, tx, actor, v.author)
	if err != nil {
		return nil, err
	}
	if !users[actor].active || !users[v.author].active {
		return nil, service.ErrTavernCommerceForbidden
	}
	t, err := scanTavernTicket(tx.QueryRowContext(ctx, `SELECT `+tavernTicketFields+` FROM tavern_tickets WHERE buyer_user_id=$1 AND operation_id=$2`, actor, in.OperationID))
	if err != nil {
		return nil, err
	}
	if t != nil {
		if t.RoomID != room || t.Amount != in.ExpectedPrice {
			return nil, service.ErrTavernCommerceInvalid
		}
		return t, tx.Commit()
	}
	if v.status != "lobby" || v.players >= v.max || v.price != in.ExpectedPrice {
		return nil, service.ErrTavernCommerceInvalid
	}
	var enabled, available, exists bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM tavern_commerce_policy WHERE id=TRUE FOR SHARE`).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, service.ErrTavernCommerceClosed
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tavern_rooms r JOIN tavern_scripts s ON s.id=r.script_id JOIN tavern_game_packages p ON p.id=r.package_id
 WHERE r.id=$1 AND s.status='listed' AND s.deleted_at IS NULL AND p.status='published')`, room).Scan(&available); err != nil {
		return nil, err
	}
	if !available {
		return nil, service.ErrTavernCommerceForbidden
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tavern_tickets WHERE room_id=$1 AND buyer_user_id=$2 AND status<>'refunded') OR EXISTS(SELECT 1 FROM tavern_room_players WHERE room_id=$1 AND user_id=$2 AND status='joined')`, room, actor).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, service.ErrTavernCommerceInvalid
	}
	var balance string
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$2::numeric,updated_at=NOW() WHERE id=$1 AND balance>=$2::numeric RETURNING balance::text`, actor, v.price).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrTavernCommerceFunds
	}
	if err != nil {
		return nil, err
	}
	t, err = scanTavernTicket(tx.QueryRowContext(ctx, `INSERT INTO tavern_tickets(room_id,script_id,buyer_user_id,author_user_id,operation_id,title,amount) VALUES($1,$2,$3,$4,$5,$6,$7::numeric) RETURNING `+tavernTicketFields, room, v.script, actor, v.author, in.OperationID, v.title, v.price))
	if err != nil {
		return nil, err
	}
	if err = tavernTicketBalanceLedger(ctx, tx, t, actor, false, balance); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tavern_room_players(room_id,user_id,status,joined_at) VALUES($1,$2,'joined',NOW()) ON CONFLICT(room_id,user_id) DO UPDATE SET status='joined',joined_at=NOW(),left_at=NULL`, room, actor); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tavern_rooms SET current_players=current_players+1,updated_at=NOW() WHERE id=$1`, room); err != nil {
		return nil, err
	}
	return t, tx.Commit()
}
func tavernTicketBalanceLedger(ctx context.Context, tx *sql.Tx, t *service.TavernTicket, actor int64, refund bool, balance string) error {
	source, amount := "tavern_ticket_purchase", "-"+t.Amount
	if refund {
		source, amount = "tavern_ticket_refund", t.Amount
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO user_balance_ledger(user_id,source_type,source_id,amount,balance_after,status,note,created_by,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,'posted',$6,$7,NOW())`, t.BuyerUserID, source, source+"_"+strconv.FormatInt(t.ID, 10), amount, balance, t.Title, actor)
	return err
}
func refundTavernTicketTx(ctx context.Context, tx *sql.Tx, t *service.TavernTicket, actor int64, reason string) error {
	var balance string
	if err := tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1 RETURNING balance::text`, t.BuyerUserID, t.Amount).Scan(&balance); err != nil {
		return err
	}
	if err := tavernTicketBalanceLedger(ctx, tx, t, actor, true, balance); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tavern_tickets SET status='refunded',refunded_at=NOW(),refund_reason=$2 WHERE id=$1 AND status='held'`, t.ID, reason); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tavern_room_players SET status='left',left_at=NOW() WHERE room_id=$1 AND user_id=$2 AND status='joined'`, t.RoomID, t.BuyerUserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tavern_runtime_sessions SET status='revoked',updated_at=NOW() WHERE room_id=$1 AND user_id=$2 AND status='active'`, t.RoomID, t.BuyerUserID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE tavern_rooms SET current_players=GREATEST(current_players-1,0),updated_at=NOW() WHERE id=$1`, t.RoomID)
	return err
}
func (r *bizDecipherRepository) RefundTavernTicket(ctx context.Context, room, actor, ticketID int64) (*service.TavernTicket, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockTavernTicketRoom(ctx, tx, room); err != nil {
		return nil, err
	}
	v, err := readTavernTicketRoom(ctx, tx, room)
	if err != nil {
		return nil, err
	}
	users, err := lockColumnCommerceUsers(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	if !users[actor].active {
		return nil, service.ErrTavernCommerceForbidden
	}
	t, err := scanTavernTicket(tx.QueryRowContext(ctx, `SELECT `+tavernTicketFields+` FROM tavern_tickets WHERE room_id=$1 AND buyer_user_id=$2 AND id=$3 FOR UPDATE`, room, actor, ticketID))
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, service.ErrTavernCommerceForbidden
	}
	if t.Status == "refunded" {
		return t, tx.Commit()
	}
	if t.Status != "held" || (v.status != "draft" && v.status != "lobby") {
		return nil, service.ErrTavernCommerceInvalid
	}
	if err = refundTavernTicketTx(ctx, tx, t, actor, "Player left before start"); err != nil {
		return nil, err
	}
	t, err = scanTavernTicket(tx.QueryRowContext(ctx, `SELECT `+tavernTicketFields+` FROM tavern_tickets WHERE id=$1`, t.ID))
	if err != nil {
		return nil, err
	}
	return t, tx.Commit()
}
func (r *bizDecipherRepository) ListTavernTicketBuyers(ctx context.Context, room, actor int64) ([]int64, error) {
	var owner int64
	if err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM tavern_rooms WHERE id=$1 AND deleted_at IS NULL`, room).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != actor {
		return nil, service.ErrTavernCommerceForbidden
	}
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT buyer_user_id FROM tavern_tickets WHERE room_id=$1 ORDER BY buyer_user_id`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// All room transitions use this transaction, including free legacy rooms, so
// ticket joins cannot race owner start/cancel.
func (r *bizDecipherRepository) transitionTavernRoom(ctx context.Context, room, actor int64, allowed []string, next string, open bool) (*service.TavernRoom, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockTavernTicketRoom(ctx, tx, room); err != nil {
		return nil, err
	}
	v, err := readTavernTicketRoom(ctx, tx, room)
	if err != nil {
		return nil, err
	}
	if actor != v.owner {
		return nil, service.ErrTavernRoomForbidden
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+tavernTicketFields+` FROM tavern_tickets WHERE room_id=$1 AND status='held' ORDER BY id`, room)
	if err != nil {
		return nil, err
	}
	tickets := []*service.TavernTicket{}
	ids := []int64{actor}
	for rows.Next() {
		t, e := scanTavernTicket(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		tickets = append(tickets, t)
		ids = append(ids, t.BuyerUserID, t.AuthorUserID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	users, err := lockColumnCommerceUsers(ctx, tx, ids...)
	if err != nil {
		return nil, err
	}
	if !users[actor].active {
		return nil, service.ErrTavernRoomForbidden
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM tavern_rooms WHERE id=$1 FOR UPDATE`, room).Scan(&status); err != nil {
		return nil, err
	}
	if status == next && (next == "running" || next == "cancelled") {
		return r.getTavernRoomAfterCommit(ctx, tx, room)
	}
	if !stringInSlice(status, allowed) || (next == "running" && v.players <= 0) {
		return nil, service.ErrTavernRoomState
	}
	if next == "running" {
		if v.author > 0 {
			var playable bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tavern_rooms r JOIN tavern_scripts s ON s.id=r.script_id JOIN tavern_game_packages p ON p.id=r.package_id
 WHERE r.id=$1 AND s.status='listed' AND s.deleted_at IS NULL AND p.status='published')`, room).Scan(&playable); err != nil {
				return nil, err
			}
			if !playable {
				return nil, service.ErrTavernCommerceForbidden
			}
		}
		for _, t := range tickets {
			if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_wallets(owner_id) VALUES($1) ON CONFLICT(owner_id) DO NOTHING`, t.AuthorUserID); err != nil {
				return nil, err
			}
			var available string
			if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount+$2::numeric,version=version+1,updated_at=NOW() WHERE owner_id=$1 RETURNING available_amount::text`, t.AuthorUserID, t.Amount).Scan(&available); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_earnings_ledger(owner_id,event_type,operation_id,gross_amount,platform_fee_amount,net_amount,wallet_delta,available_after,status,available_at,metadata,posted_at)
 VALUES($1,'earning',$2,$3::numeric,0,$3::numeric,$3::numeric,$4::numeric,'available',NOW(),jsonb_build_object('source_type','tavern_ticket_settlement','room_id',$5::bigint,'script_id',$6::bigint,'ticket_id',$7::bigint,'title',$8::text),NOW())`, t.AuthorUserID, fmt.Sprintf("tavern_ticket_settlement_%d", t.ID), t.Amount, available, room, t.ScriptID, t.ID, t.Title); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE tavern_tickets SET status='released',released_at=NOW() WHERE id=$1`, t.ID); err != nil {
				return nil, err
			}
		}
	} else if next == "cancelled" {
		for _, t := range tickets {
			if err = refundTavernTicketTx(ctx, tx, t, actor, "Room cancelled before start"); err != nil {
				return nil, err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE tavern_rooms SET status=$2::varchar,current_phase=CASE WHEN $2::varchar IN ('completed','cancelled') THEN 'ended' ELSE $2::varchar END,
 visibility=CASE WHEN $3 THEN 'public' ELSE visibility END,started_at=CASE WHEN $2::varchar='running' THEN COALESCE(started_at,NOW()) ELSE started_at END,
 ended_at=CASE WHEN $2::varchar IN ('completed','cancelled') THEN COALESCE(ended_at,NOW()) ELSE ended_at END,updated_at=NOW() WHERE id=$1`, room, next, open)
	if err != nil {
		return nil, err
	}
	return r.getTavernRoomAfterCommit(ctx, tx, room)
}
func (r *bizDecipherRepository) getTavernRoomAfterCommit(ctx context.Context, tx *sql.Tx, room int64) (*service.TavernRoom, error) {
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTavernRoom(ctx, room)
}
