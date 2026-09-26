package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type tokenRewardRepository struct{ db *sql.DB }

func NewTokenRewardRepository(db *sql.DB) service.TokenRewardRepository {
	return &tokenRewardRepository{db}
}
func rewardConflict(message string) error {
	return infraerrors.Conflict("TOKEN_REWARD_CONFLICT", message)
}
func (r *tokenRewardRepository) ValidateSponsor(ctx context.Context, id int64) (string, error) {
	var key string
	err := r.db.QueryRowContext(ctx, `SELECT k.key FROM api_keys k JOIN users u ON u.id=k.user_id JOIN groups g ON g.id=k.group_id
	WHERE k.id=$1 AND k.status='active' AND k.deleted_at IS NULL
	AND (k.expires_at IS NULL OR k.expires_at>NOW()) AND (k.quota=0 OR k.quota_used<k.quota)
	AND u.role='admin' AND u.status='active' AND u.deleted_at IS NULL
	AND g.status='active' AND g.deleted_at IS NULL
	AND NOT EXISTS(SELECT 1 FROM shared_pool_access_keys s WHERE s.api_key_id=k.id)`, id).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", rewardConflict("活动资源必须使用有效的运营官方密钥")
	}
	return key, err
}

const packetColumns = `id,COALESCE(message_id,0),sender_id,resource_id,model,total_tokens,portions,claimed,mode,blessing,opens_at,closes_at,use_hours,NOW()`

func scanTokenPacket(row scanner) (*service.TokenPacket, error) {
	p := new(service.TokenPacket)
	err := row.Scan(&p.ID, &p.MessageID, &p.SenderID, &p.ResourceID, &p.Model, &p.Total, &p.Portions, &p.Claimed, &p.Mode, &p.Blessing, &p.OpensAt, &p.ClosesAt, &p.UseHours, &p.ServerTime)
	return p, err
}
func (r *tokenRewardRepository) CreateTokenPacket(ctx context.Context, slug string, user int64, in service.TokenPacketInput, res service.TokenRewardResource, shares []int64, hash string) (*service.TokenPacket, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var admin bool
	if err = tx.QueryRowContext(ctx, `SELECT role='admin' AND status='active' AND deleted_at IS NULL FROM users WHERE id=$1`, user).Scan(&admin); err != nil {
		return nil, err
	}
	if !admin {
		return nil, infraerrors.Forbidden("TOKEN_PACKET_ADMIN_ONLY", "仅运营方可以发红包")
	}
	// Serialize creation retries before inserting either the packet or its card.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("token-packet:%d:%s", user, in.ClientID)); err != nil {
		return nil, err
	}
	var existing int64
	var stored string
	var originalSlug string
	err = tx.QueryRowContext(ctx, `SELECT p.id,p.request_hash,c.slug FROM biz_token_packets p JOIN biz_chat_channels c ON c.id=p.channel_id WHERE sender_id=$1 AND client_id=$2`, user, in.ClientID).Scan(&existing, &stored, &originalSlug)
	if err == nil {
		if stored != hash || originalSlug != slug {
			return nil, rewardConflict("重复请求的红包规则发生变化")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetTokenPacket(ctx, existing, user)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var channel int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM biz_chat_channels WHERE slug=$1 AND kind='public'`, slug).Scan(&channel); err != nil {
		return nil, service.ErrChatChannelNotFound
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO biz_token_packets(channel_id,sender_id,client_id,request_hash,resource_id,model,sponsor_key_id,total_tokens,portions,allocations,mode,blessing,opens_at,closes_at,use_hours)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW()+$13*INTERVAL '1 second',NOW()+$13*INTERVAL '1 second'+$14*INTERVAL '1 hour',$15) RETURNING id`,
		channel, user, in.ClientID, hash, res.ID, res.Model, res.KeyID, in.Total, in.Portions, pq.Array(shares), in.Mode, in.Blessing, in.DelaySeconds, in.ClaimHours, in.UseHours).Scan(&id)
	if err != nil {
		return nil, err
	}
	var messageID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO biz_chat_messages(channel_id,sender_user_id,client_message_id,body) VALUES($1,$2,$3,$4) RETURNING id`,
		channel, user, fmt.Sprintf("token-packet-%d", id), in.Blessing).Scan(&messageID)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE biz_token_packets SET message_id=$2 WHERE id=$1`, id, messageID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTokenPacket(ctx, id, user)
}
func (r *tokenRewardRepository) GetTokenPacket(ctx context.Context, id, user int64) (*service.TokenPacket, error) {
	p, err := scanTokenPacket(r.db.QueryRowContext(ctx, `SELECT `+packetColumns+` FROM biz_token_packets WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.NotFound("TOKEN_PACKET_NOT_FOUND", "红包不存在")
	}
	if err != nil {
		return nil, err
	}
	g, err := scanTokenGrant(r.db.QueryRowContext(ctx, grantSelect+` WHERE g.packet_id=$1 AND g.user_id=$2`, id, user))
	if err == nil {
		p.Mine = g
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return p, nil
}

const grantSelect = `SELECT g.id,g.packet_id,p.resource_id,p.model,g.tokens,g.used,g.reserved,g.expires_at FROM biz_token_grants g JOIN biz_token_packets p ON p.id=g.packet_id`

func scanTokenGrant(row scanner) (*service.TokenGrant, error) {
	g := new(service.TokenGrant)
	err := row.Scan(&g.ID, &g.PacketID, &g.ResourceID, &g.Model, &g.Tokens, &g.Used, &g.Reserved, &g.ExpiresAt)
	return g, err
}
func (r *tokenRewardRepository) ClaimTokenPacket(ctx context.Context, id, user int64) (*service.TokenGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := scanTokenPacket(tx.QueryRowContext(ctx, `SELECT `+packetColumns+` FROM biz_token_packets WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.NotFound("TOKEN_PACKET_NOT_FOUND", "红包不存在")
	}
	if err != nil {
		return nil, err
	}
	g, err := scanTokenGrant(tx.QueryRowContext(ctx, grantSelect+` WHERE g.packet_id=$1 AND g.user_id=$2`, id, user))
	if err == nil {
		return g, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if user == p.SenderID {
		return nil, rewardConflict("不能领取自己发出的运营红包")
	}
	if p.ServerTime.Before(p.OpensAt) {
		return nil, rewardConflict("还没到开抢时间")
	}
	if !p.ServerTime.Before(p.ClosesAt) {
		return nil, rewardConflict("红包已过期")
	}
	if p.Claimed >= p.Portions {
		return nil, rewardConflict("红包已抢完")
	}
	// Check the recipient even when called outside HTTP authentication.
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT status='active' AND deleted_at IS NULL FROM users WHERE id=$1`, user).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, infraerrors.Forbidden("TOKEN_REWARD_ACCOUNT", "账号当前不可领取")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO biz_token_grants(packet_id,user_id,tokens,expires_at)
	SELECT id,$2,allocations[claimed+1],NOW()+use_hours*INTERVAL '1 hour' FROM biz_token_packets WHERE id=$1`, id, user)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE biz_token_packets SET claimed=claimed+1 WHERE id=$1`, id); err != nil {
		return nil, err
	}
	g, err = scanTokenGrant(tx.QueryRowContext(ctx, grantSelect+` WHERE g.packet_id=$1 AND g.user_id=$2`, id, user))
	if err != nil {
		return nil, err
	}
	return g, tx.Commit()
}

const runColumns = `id,client_id,resource_id,model,reserved,COALESCE(actual,0),COALESCE(covered,0),status,result,note,created_at`

func scanTokenRun(row scanner) (*service.TokenRewardRun, error) {
	v := new(service.TokenRewardRun)
	err := row.Scan(&v.ID, &v.ClientID, &v.ResourceID, &v.Model, &v.Reserved, &v.Actual, &v.Covered, &v.Status, &v.Result, &v.Note, &v.CreatedAt)
	return v, err
}
func (r *tokenRewardRepository) TokenRewardWallet(ctx context.Context, user int64) (*service.TokenRewardWallet, error) {
	w := &service.TokenRewardWallet{Grants: []service.TokenGrant{}, Runs: []service.TokenRewardRun{}}
	if err := r.db.QueryRowContext(ctx, `SELECT NOW()`).Scan(&w.ServerTime); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, grantSelect+` WHERE g.user_id=$1 ORDER BY g.expires_at,g.id`, user)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		g, e := scanTokenGrant(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		w.Grants = append(w.Grants, *g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT `+runColumns+` FROM biz_token_runs WHERE user_id=$1 ORDER BY id DESC LIMIT 30`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanTokenRun(rows)
		if e != nil {
			return nil, e
		}
		if v.Status == "pending" && time.Since(v.CreatedAt) > 3*time.Minute {
			v.Note = "调用结果待核对，不会自动重试或扣充值余额"
		}
		w.Runs = append(w.Runs, *v)
	}
	return w, rows.Err()
}
func (r *tokenRewardRepository) ReserveTokenRun(ctx context.Context, user int64, in service.TokenRewardRunInput, res service.TokenRewardResource, bound int64, hash string) (*service.TokenRewardRun, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("token-user:%d", user)); err != nil {
		return nil, false, err
	}
	var oldHash string
	err = tx.QueryRowContext(ctx, `SELECT request_hash FROM biz_token_runs WHERE user_id=$1 AND client_id=$2`, user, in.ClientID).Scan(&oldHash)
	if err == nil {
		if oldHash != hash {
			return nil, false, rewardConflict("同一调用编号不能修改内容")
		}
		v, e := scanTokenRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM biz_token_runs WHERE user_id=$1 AND client_id=$2`, user, in.ClientID))
		if e != nil {
			return nil, false, e
		}
		return v, false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM biz_token_runs WHERE user_id=$1 AND status IN ('pending','review')`, user).Scan(&pending); err != nil {
		return nil, false, err
	}
	if pending >= 2 {
		return nil, false, rewardConflict("已有两次调用处理中或待核对，请先等待结果")
	}
	rows, err := tx.QueryContext(ctx, `SELECT g.id,g.tokens-g.used-g.reserved FROM biz_token_grants g JOIN biz_token_packets p ON p.id=g.packet_id
	WHERE g.user_id=$1 AND p.resource_id=$2 AND p.model=$3 AND p.sponsor_key_id=$4 AND g.expires_at>NOW() AND g.tokens>g.used+g.reserved
	ORDER BY g.expires_at,g.id FOR UPDATE OF g`, user, res.ID, res.Model, res.KeyID)
	if err != nil {
		return nil, false, err
	}
	type share struct{ id, n int64 }
	var shares []share
	remaining := bound
	for rows.Next() {
		var id, n int64
		if err = rows.Scan(&id, &n); err != nil {
			rows.Close()
			return nil, false, err
		}
		if remaining <= 0 {
			continue
		}
		if n > remaining {
			n = remaining
		}
		shares = append(shares, share{id, n})
		remaining -= n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	if remaining > 0 {
		return nil, false, rewardConflict("奖励额度不足以预留本次调用，请缩短输入或输出；不会扣充值余额")
	}
	run, err := scanTokenRun(tx.QueryRowContext(ctx, `INSERT INTO biz_token_runs(user_id,client_id,request_hash,resource_id,sponsor_key_id,model,reserved,status)
	VALUES($1,$2,$3,$4,$5,$6,$7,'pending') RETURNING `+runColumns, user, in.ClientID, hash, res.ID, res.KeyID, res.Model, bound))
	if err != nil {
		return nil, false, err
	}
	for _, v := range shares {
		if _, err = tx.ExecContext(ctx, `UPDATE biz_token_grants SET reserved=reserved+$2 WHERE id=$1`, v.id, v.n); err != nil {
			return nil, false, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO biz_token_run_grants(run_id,grant_id,reserved) VALUES($1,$2,$3)`, run.ID, v.id, v.n); err != nil {
			return nil, false, err
		}
	}
	return run, true, tx.Commit()
}
func (r *tokenRewardRepository) FinishTokenRun(ctx context.Context, id int64, status string, actual int64, result, note string) (*service.TokenRewardRun, error) {
	if (status != "succeeded" && status != "failed" && status != "review") || actual < 0 || actual > 2e9 {
		return nil, rewardConflict("结算状态不正确")
	}
	if status == "failed" && actual != 0 {
		return nil, rewardConflict("失败释放必须确认零用量")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	run, err := finishTokenRunTx(ctx, tx, id, status, actual, result, note)
	if err != nil {
		return nil, err
	}
	return run, tx.Commit()
}
func finishTokenRunTx(ctx context.Context, tx *sql.Tx, id int64, status string, actual int64, result, note string) (*service.TokenRewardRun, error) {
	run, err := scanTokenRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM biz_token_runs WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if run.Status == "succeeded" || run.Status == "failed" {
		return run, nil
	}
	covered := actual
	if covered > run.Reserved {
		covered = run.Reserved
	}
	if status != "review" {
		rows, e := tx.QueryContext(ctx, `SELECT rg.grant_id,rg.reserved FROM biz_token_run_grants rg JOIN biz_token_grants g ON g.id=rg.grant_id WHERE run_id=$1 ORDER BY g.expires_at,g.id FOR UPDATE OF g`, id)
		if e != nil {
			return nil, e
		}
		type allocation struct{ id, n int64 }
		var entries []allocation
		for rows.Next() {
			var v allocation
			if e = rows.Scan(&v.id, &v.n); e != nil {
				rows.Close()
				return nil, e
			}
			entries = append(entries, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		left := covered
		for _, v := range entries {
			use := v.n
			if use > left {
				use = left
			}
			left -= use
			if _, err = tx.ExecContext(ctx, `UPDATE biz_token_grants SET reserved=reserved-$2,used=used+$3 WHERE id=$1`, v.id, v.n, use); err != nil {
				return nil, err
			}
		}
	}
	run, err = scanTokenRun(tx.QueryRowContext(ctx, `UPDATE biz_token_runs SET status=$2,actual=$3,covered=$4,result=$5,note=$6,updated_at=NOW() WHERE id=$1 RETURNING `+runColumns, id, status, actual, covered, result, note))
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *tokenRewardRepository) LinkTokenRunRequest(ctx context.Context, id int64, requestID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE biz_token_runs SET gateway_request_id=$2 WHERE id=$1 AND gateway_request_id=''`, id, requestID)
	return err
}
func (r *tokenRewardRepository) ReviewTokenRuns(ctx context.Context) ([]service.TokenRewardReview, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+runColumns+`,user_id,gateway_request_id FROM biz_token_runs WHERE status='review' OR (status='pending' AND created_at<NOW()-INTERVAL '5 minutes') ORDER BY id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.TokenRewardReview{}
	for rows.Next() {
		var v service.TokenRewardReview
		err = rows.Scan(&v.ID, &v.ClientID, &v.ResourceID, &v.Model, &v.Reserved, &v.Actual, &v.Covered, &v.Status, &v.Result, &v.Note, &v.CreatedAt, &v.UserID, &v.GatewayRequestID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *tokenRewardRepository) ResolveTokenRun(ctx context.Context, id, operator, actual int64, evidence string) (*service.TokenRewardRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var admin bool
	if err = tx.QueryRowContext(ctx, `SELECT role='admin' AND status='active' AND deleted_at IS NULL FROM users WHERE id=$1`, operator).Scan(&admin); err != nil {
		return nil, err
	}
	if !admin {
		return nil, infraerrors.Forbidden("TOKEN_REWARD_ADMIN_ONLY", "仅运营方可核对异常调用")
	}
	run, err := scanTokenRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM biz_token_runs WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	var oldActual int64
	var oldEvidence string
	err = tx.QueryRowContext(ctx, `SELECT actual,evidence FROM biz_token_run_resolutions WHERE run_id=$1`, id).Scan(&oldActual, &oldEvidence)
	if err == nil {
		if oldActual != actual || oldEvidence != evidence {
			return nil, rewardConflict("该调用已按其他核对结果结算")
		}
		return run, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var stale bool
	if err = tx.QueryRowContext(ctx, `SELECT created_at<NOW()-INTERVAL '5 minutes' FROM biz_token_runs WHERE id=$1`, id).Scan(&stale); err != nil {
		return nil, err
	}
	if (run.Status != "review" && run.Status != "pending") || !stale {
		return nil, rewardConflict("只能核对已等待至少五分钟的未决调用")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO biz_token_run_resolutions(run_id,operator_id,actual,evidence) VALUES($1,$2,$3,$4)`, id, operator, actual, evidence)
	if err != nil {
		return nil, err
	}
	status := "succeeded"
	if actual == 0 {
		status = "failed"
	}
	run, err = finishTokenRunTx(ctx, tx, id, status, actual, run.Result, "运营已核对并结算；核对依据已留档")
	if err != nil {
		return nil, err
	}
	return run, tx.Commit()
}
