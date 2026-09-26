package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
)

// schemaMigrationsTableDDL 定义迁移记录表的 DDL。
// 该表用于跟踪已应用的迁移文件及其校验和。
// - filename: 迁移文件名，作为主键唯一标识每个迁移
// - checksum: 文件内容的 SHA256 哈希值，用于检测迁移文件是否被篡改
// - applied_at: 迁移应用时间戳
const schemaMigrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	checksum   TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const atlasSchemaRevisionsTableDDL = `
CREATE TABLE IF NOT EXISTS atlas_schema_revisions (
	version TEXT PRIMARY KEY,
	description TEXT NOT NULL,
	type INTEGER NOT NULL,
	applied INTEGER NOT NULL DEFAULT 0,
	total INTEGER NOT NULL DEFAULT 0,
	executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	execution_time BIGINT NOT NULL DEFAULT 0,
	error TEXT NULL,
	error_stmt TEXT NULL,
	hash TEXT NOT NULL DEFAULT '',
	partial_hashes TEXT[] NULL,
	operator_version TEXT NULL
);
`

// migrationsAdvisoryLockID 是用于序列化迁移操作的 PostgreSQL Advisory Lock ID。
// 在多实例部署场景下，该锁确保同一时间只有一个实例执行迁移。
// 任何稳定的 int64 值都可以，只要不与同一数据库中的其他锁冲突即可。
const migrationsAdvisoryLockID int64 = 694208311321144027
const migrationsLockRetryInterval = 500 * time.Millisecond
const migrationsSessionCleanupTimeout = 5 * time.Second
const migrationsSessionSetLockTimeoutSQL = "SET lock_timeout = '5s'"
const migrationsSessionResetLockTimeoutSQL = "RESET lock_timeout"
const nonTransactionalMigrationSuffix = "_notx.sql"
const paymentOrdersOutTradeNoUniqueMigration = "120_enforce_payment_orders_out_trade_no_unique_notx.sql"
const paymentOrdersOutTradeNoUniqueIndex = "paymentorder_out_trade_no_unique"
const schedulerOutboxPendingDedupKeyMigration = "153_scheduler_outbox_pending_dedup_key_index_notx.sql"
const schedulerOutboxPendingDedupKeyIndex = "idx_scheduler_outbox_pending_dedup_key"
const latestAPIKeyIPIndexMigration = "174_add_usage_logs_api_key_latest_ip_index_notx.sql"
const latestAPIKeyIPIndex = "idx_usage_logs_api_key_latest_ip"
const sharedPoolPriceVersionIndexesMigration = "224_shared_pool_price_version_indexes_notx.sql"
const sharedPoolRoutingHotPathIndexesMigration = "228_shared_pool_routing_hot_path_indexes_notx.sql"
const sharedPoolCompactCapabilityIndexMigration = "228a_shared_pool_compact_capability_index_notx.sql"
const sharedPoolCompactCapabilityIndex = "idx_shared_pool_probe_jobs_compact_capability"

var sharedPoolPriceVersionIndexes = []string{
	"idx_shared_pool_owner_earnings_price_version",
	"idx_shared_pool_balance_ledger_price_version",
	"idx_shared_pool_usage_reservations_price_version",
}

var sharedPoolRoutingHotPathIndexes = []string{
	"idx_shared_pool_models_aliases_open_gin",
	"idx_shared_pool_accounts_schedulable_route",
}

type migrationExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type migrationSession interface {
	migrationExecutor
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type migrationChecksumCompatibilityRule struct {
	fileChecksum       string
	acceptedDBChecksum map[string]struct{}
	acceptedChecksums  map[string]struct{}
}

// migrationChecksumCompatibilityRules 仅用于兼容历史上误修改过的迁移文件 checksum。
// 规则必须同时匹配「迁移名 + 数据库 checksum + 当前文件 checksum」且两者都落在该迁移的已知版本集合内才会放行，
// 避免放宽全局校验，也允许将误改的历史 migration 回滚为已发布版本而不要求人工修 checksum。
var migrationChecksumCompatibilityRules = map[string]migrationChecksumCompatibilityRule{
	"054_drop_legacy_cache_columns.sql":                       newMigrationChecksumCompatibilityRule("82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d", "182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4"),
	"061_add_usage_log_request_type.sql":                      newMigrationChecksumCompatibilityRule("66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c", "08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0", "222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3"),
	"109_auth_identity_compat_backfill.sql":                   newMigrationChecksumCompatibilityRule("0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace", "551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee"),
	"110_pending_auth_and_provider_default_grants.sql":        newMigrationChecksumCompatibilityRule("32cf87ee787b1bb36b5c691367c96eee37518fa3eed6f3322cf68795e3745279", "e3d1f433be2b564cfbdc549adf98fce13c5c7b363ebc20fd05b765d0563b0925"),
	"112_add_payment_order_provider_key_snapshot.sql":         newMigrationChecksumCompatibilityRule("b75f8f56d39455682787696a3d92ad25b055444ca328fb7fca9a460a15d68d99", "ffd3e8a2c9295fa9cbefefd629a78268877e5b51bc970a82d9b3f46ec4ebd15e"),
	"115_auth_identity_legacy_external_backfill.sql":          newMigrationChecksumCompatibilityRule("022aadd97bb53e755f0cf7a3a957e0cb1a1353b0c39ec4de3234acd2871fd04f", "4cf39e508be9fd1a5aa41610cbbebeb80385c9adda45bf78a706de9db4f1385f"),
	"116_auth_identity_legacy_external_safety_reports.sql":    newMigrationChecksumCompatibilityRule("07edb09fa8d04ffb172b0621e3c22f4d1757d20a24ae267b3b36b087ab72d488", "f7757bd929ac67ffb08ce69fa4cf20fad39dbff9d5a5085fb2adabb7607e5877"),
	"118_wechat_dual_mode_and_auth_source_defaults.sql":       newMigrationChecksumCompatibilityRule("b54194d7a3e4fbf710e0a3590d22a2fe7966804c487052a356e0b55f53ef96b0", "e0cdf835d6c688d64100f483d31bc02ac9ebad414bf1837af239a84bf75b8227", "a38243ca0a72c3a01c0a92b7986423054d6133c0399441f853b99802852720fb"),
	"119_enforce_payment_orders_out_trade_no_unique.sql":      newMigrationChecksumCompatibilityRule("0bbe809ae48a9d811dabda1ba1c74955bd71c4a9cc610f9128816818dfa6c11e", "ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34"),
	"120_enforce_payment_orders_out_trade_no_unique_notx.sql": newMigrationChecksumCompatibilityRule("34aadc0db59a4e390f92a12b73bd74642d9724f33124f73638ae00089ea5e074", "e77921f79d539bc24575cb9c16cbe566d2b23ce816190343d0a7568f6a3fcf61", "707431450603e70a43ce9fbd61e0c12fa67da4875158ccefabacea069587ab22", "04b082b5a239c525154fe9185d324ee2b05ff90da9297e10dba19f9be79aa59a"),
	"123_fix_legacy_auth_source_grant_on_signup_defaults.sql": newMigrationChecksumCompatibilityRule("2ce43c2cd89e9f9e1febd34a407ed9e84d177386c5544b6f02c1f58a21129f57", "6cd33422f215dcd1f486ab6f35c0ea5805d9ca69bb25906d94bc649156657145"),
	"151_bizdecipher_asset_layer.sql":                         newMigrationChecksumCompatibilityRule("c3e2d30a5ffb619b2eb114d30d4de5e052d8194794790dd98d64218c0a49f84e", "252f70be9b00a16ac305226096d8496b53d9c2991c863c7921309bb7132e84ea"),
	"152_bizdecipher_launch_settings.sql":                     newMigrationChecksumCompatibilityRule("f120aebd8d4d139e22e187e6f94719498f9e07c49a6febf2592dd714fe375213", "dde6b0b3dc48697c00c910cb5dc03162317da4ff59b6c6777c61ef895dfd9fd9"),
	"159_promo_campaigns.sql":                                 newMigrationChecksumCompatibilityRule("c8584aab2e5dead3018db34a1047eac7cccf99608d1dd9f73ab6c30550a937bf", "3215014066feccb6f69f33e7527fa2cb52b0a7608b922e11117fc6ad1537428e"),
	"160_shared_pools.sql":                                    newMigrationChecksumCompatibilityRule("ad875526bb6608f8ab1ac5b3a3e5ab014fa3be34b587655b7771ce4e52628080", "0e328d623855d0ef6cb7d04035e0a6de1aff5105a731442bc78c58b6ad4d18fa"),
	"161_pool_seats.sql":                                      newMigrationChecksumCompatibilityRule("9d4f5af31029540c885fd9aa40d31bbe250b3333e5ab4cbe163988ea970b4aea", "28e5baa913bd00c7434caa2745a03adbf27e330450b5147b199bb8032381fcc2"),
	"162_shared_pool_access_keys.sql":                         newMigrationChecksumCompatibilityRule("4c31793351f15c91ec8433515c2da8e49531eabd1660a95e60ce0405249fe84d", "93913f8677f2e1d412e9ba33148340cc924efab937f48b9e37734941eac05d49"),
	"163_disable_seeded_register_promo.sql":                   newMigrationChecksumCompatibilityRule("cc306abca89ba30efbd43c988d970ecb6a7a5ffc095d563f0dafe6c8f3038fff", "0438497da66245799b53af72d1514003b27305c6571c2d5fcb4aa6d527e53be0"),
	"164_bizdecipher_brand_defaults.sql":                      newMigrationChecksumCompatibilityRule("1dc0222eecdbab1d523820adfc1086e2bda73a473901c9ac16312397e3185659", "7b3a56d5cfc3dcf2c393a035d5f595f0d75b7bcccd73838e224dfeac258a8434"),
	"165_invite_reward_config_defaults.sql":                   newMigrationChecksumCompatibilityRule("bac778ce2facde481bee1d0377b1f68015a60c425eeb89478b3553ddaec0ec5b", "c76e55ae2ce0a925100d746b41c793f8380f8ab6279bfcd514dbd2192f9ecb09"),
	"167_account_square_model_catalog.sql":                    newMigrationChecksumCompatibilityRule("b1053310148f1050be80b097621da9a2a05c0e7adfafd258946a228c9c8e9d3e", "5d4fb16fefc204089408c4a414b21bf091e92a82bb36ec3f3c68d8aef2f036c0"),
	"168_shared_pool_publish_system.sql":                      newMigrationChecksumCompatibilityRule("37e5291a623e7498f4008f0a82aac7b5dc44e4deb4a92ce43e1166f837deb72e", "af12006fffef8ec28f01e9d448e827b55a598a195a632d468c7fed02bc5d9891"),
	"169_shared_pool_model_usage_windows.sql":                 newMigrationChecksumCompatibilityRule("b770e23cd760de04ce656329c7dc17bee11b63d7dddfb01b0d317488359512a1", "af73d0e32cd65fc38faca3c4bde1bec763badb92f030a6668c358a14a2c2212d"),
	"170_balance_credit_dual_track.sql":                       newMigrationChecksumCompatibilityRule("6f7205319d2c531a3f88966264b63dfd6cecac8a0c507467b7230487de4f49d8", "3dcf24559f699783488f27a0dd2afd5c08540407653aa76c82da68af8bf43754"),
	"171_signup_email_starter_credit.sql":                     newMigrationChecksumCompatibilityRule("6396aaeb99a86835e6685cd477e8532febba9935d2a06d3f6b1e34db6e3dc4dd", "94ce4bf0e6f2bc07e58ad3946d3aa0d98ec05c9eb45d9998dc2186b3e4b0dc41"),
	"172_daily_checkin_blind_box.sql":                         newMigrationChecksumCompatibilityRule("fb7a7947cfc855d4f276837fddafc4157ee657f2748a49b7ea03ef808d9a558f", "5ef130064def26e8fc5df1db8bc4e09c8e7d274f337bd94a7c6d79144b6d3af5"),
	"174_api_key_starter_credit_trigger.sql":                  newMigrationChecksumCompatibilityRule("dadcd675465bbbde37f606d854eb54b7a20996080f4f82cf1646ee21e96affd7", "b325ff957f08976ff876cbd5324ec4ea2af38a9ba9d4f528fa8b59187b749b93"),
	"177_daily_fortune_credit_balance_checkin.sql":            newMigrationChecksumCompatibilityRule("6734ab51c477b7bffd7a583061658878ed384d1176d5b51810b4e20799bc4a65", "8179caa7ed8fbd8a0c9b4023d4d1dee35873ebbf8e5dcf65e4fef63ed5f62ea8"),
	"178_daily_checkin_multi_credit_balance.sql":              newMigrationChecksumCompatibilityRule("230ad46e11c3dbb9d1d67919338b3ac1d36f11a39a9c91833b78354f1f5fb3a8", "f20ec3ab245fb804fa8cf0246099f85b8c92fc666af16eedc444e844bfc6be0f"),
	"182_daily_checkin_collectible_cards.sql":                 newMigrationChecksumCompatibilityRule("9813612cdf6fb8b6be5dc8c8f64354b41500e444ddac5d0ff42ae7a618de9d6a", "8b5ae305fb2ccfd3c96714a6ab444637053e97332a2d87ad6074afa9301310b6"),
	"192_shared_pool_seat_fee_waiver.sql":                     newMigrationChecksumCompatibilityRule("41c71c2eaa9e3c508ad3027d7606ab166068dfffef5ccc64d7f14d334c2ff48a", "bf38e242bf9325b7cbe53d4cb9ebb4b88a12141f39988ffd399daebaab7cb511"),
	"159_batch_image_foundation.sql":                          newMigrationChecksumCompatibilityRule("d902b70982025ec519749faf058aab7631e82c3f48167b9a4ae4db718eb72cce", "82da85b5d98e67a0507647b873a40373e84538e4adafdeed6767c0ac8b6570b2"),
	"161_batch_image_pricing_snapshot.sql":                    newMigrationChecksumCompatibilityRule("4012af3e43636cb6af22e0176d59d1fcc70615c0f310194329461ae462c4fbd6", "96d915c9b7a6941ae99039e0ff3f1a61481eb9bddd933d11c6fadb2274554e87"),
}

// ApplyMigrations 将嵌入的 SQL 迁移文件应用到指定的数据库。
//
// 该函数可以在每次应用启动时安全调用：
// - 已应用的迁移会被自动跳过（通过校验 filename 判断）
// - 如果迁移文件内容被修改（checksum 不匹配），会返回错误
// - 使用 PostgreSQL Advisory Lock 确保多实例并发安全
//
// 参数：
//   - ctx: 上下文，用于超时控制和取消
//   - db: 数据库连接
//
// 返回：
//   - error: 迁移过程中的任何错误
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("nil sql db")
	}
	return applyMigrationsFS(ctx, db, migrations.FS)
}

// applyMigrationsFS 是迁移执行的核心实现。
// 它从指定的文件系统读取 SQL 迁移文件并按顺序应用。
//
// 迁移执行流程：
//  1. 独占一个 PostgreSQL 会话并设置 5 秒 lock_timeout
//  2. 在该会话获取 Advisory Lock，防止多实例并发迁移
//  3. 确保 schema_migrations 表存在
//  4. 按文件名排序读取所有 .sql 文件
//  5. 对于每个迁移文件：
//     - 计算文件内容的 SHA256 校验和
//     - 检查该迁移是否已应用（通过 filename 查询）
//     - 如果已应用，验证校验和是否匹配
//     - 如果未应用，在事务中执行迁移并记录
//  6. 同会话释放 Advisory Lock、RESET lock_timeout，再将连接归还池
//
// 参数：
//   - ctx: 上下文
//   - db: 数据库连接池
//   - fsys: 包含迁移文件的文件系统（通常是 embed.FS）
func applyMigrationsFS(ctx context.Context, db *sql.DB, fsys fs.FS) (retErr error) {
	if db == nil {
		return errors.New("nil sql db")
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migrations session: %w", err)
	}
	var session migrationSession = conn
	defer func() {
		if err := conn.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close migrations session: %w", err))
		}
	}()

	if _, err := session.ExecContext(ctx, migrationsSessionSetLockTimeoutSQL); err != nil {
		return fmt.Errorf("set migrations lock_timeout: %w", err)
	}

	locked := false
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), migrationsSessionCleanupTimeout)
		defer cancel()

		var cleanupErr error
		if locked {
			cleanupErr = errors.Join(cleanupErr, pgAdvisoryUnlock(cleanupCtx, session))
		}
		if _, err := session.ExecContext(cleanupCtx, migrationsSessionResetLockTimeoutSQL); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("reset migrations lock_timeout: %w", err))
		}
		if cleanupErr != nil {
			// A session whose advisory lock or lock_timeout could not be cleared
			// must never return to the pool. Dropping the physical connection also
			// lets PostgreSQL release any remaining session-level advisory lock.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		retErr = errors.Join(retErr, cleanupErr)
	}()

	// 获取分布式锁，确保多实例部署时只有一个实例执行迁移。
	// 这是 PostgreSQL 特有的 Advisory Lock 机制。
	if err := pgAdvisoryLock(ctx, session); err != nil {
		return err
	}
	locked = true

	// 创建迁移记录表（如果不存在）。
	// 该表记录所有已应用的迁移及其校验和。
	if _, err := session.ExecContext(ctx, schemaMigrationsTableDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// 自动对齐 Atlas 基线（如果检测到 legacy schema_migrations 且缺失 atlas_schema_revisions）。
	if err := ensureAtlasBaselineAligned(ctx, session, fsys); err != nil {
		return err
	}

	// 获取所有 .sql 迁移文件并按文件名排序。
	// 命名规范：使用零填充数字前缀（如 001_init.sql, 002_add_users.sql）。
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files) // 确保按文件名顺序执行迁移

	for _, name := range files {
		// 读取迁移文件内容
		contentBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		content := strings.TrimSpace(string(contentBytes))
		if content == "" {
			continue // 跳过空文件
		}

		// 计算文件内容的 SHA256 校验和，用于检测文件是否被修改。
		// 这是一种防篡改机制：如果有人修改了已应用的迁移文件，系统会拒绝启动。
		sum := sha256.Sum256([]byte(content))
		checksum := hex.EncodeToString(sum[:])

		// 检查该迁移是否已经应用
		var existing string
		rowErr := session.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename = $1", name).Scan(&existing)
		if rowErr == nil {
			// 迁移已应用，验证校验和是否匹配
			if existing != checksum {
				// 兼容已知历史版本和精确的 Up-only 前缀，不改写数据库迁移记录。
				if isMigrationChecksumCompatible(name, existing, checksum) || isMigrationUpChecksumCompatible(content, existing) {
					continue
				}
				// 校验和不匹配意味着迁移文件在应用后被修改，这是危险的。
				// 正确的做法是创建新的迁移文件来进行变更。
				return fmt.Errorf(
					"migration %s checksum mismatch (db=%s file=%s)\n"+
						"This means the migration file was modified after being applied to the database.\n"+
						"Solutions:\n"+
						"  1. Revert to original: git log --oneline -- migrations/%s && git checkout <commit> -- migrations/%s\n"+
						"  2. For new changes, create a new migration file instead of modifying existing ones\n"+
						"Note: Modifying applied migrations breaks the immutability principle and can cause inconsistencies across environments",
					name, existing, checksum, name, name,
				)
			}
			continue // 迁移已应用且校验和匹配，跳过
		}
		if !errors.Is(rowErr, sql.ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", name, rowErr)
		}

		upSQL, err := migrationUpSQL(content)
		if err != nil {
			return fmt.Errorf("parse migration %s: %w", name, err)
		}
		nonTx, err := validateMigrationExecutionMode(name, upSQL)
		if err != nil {
			return fmt.Errorf("validate migration %s: %w", name, err)
		}

		if nonTx {
			if err := prepareNonTransactionalMigration(ctx, session, name); err != nil {
				return fmt.Errorf("prepare migration %s: %w", name, err)
			}

			// *_notx.sql：用于 CREATE/DROP INDEX CONCURRENTLY 场景，必须非事务执行。
			// 逐条语句执行，避免将多条 CONCURRENTLY 语句放入同一个隐式事务块。
			statements := splitSQLStatements(upSQL)
			for i, stmt := range statements {
				trimmed := strings.TrimSpace(stmt)
				if trimmed == "" {
					continue
				}
				if stripSQLLineComment(trimmed) == "" {
					continue
				}
				if _, err := session.ExecContext(ctx, trimmed); err != nil {
					return fmt.Errorf("apply migration %s (non-tx statement %d): %w", name, i+1, err)
				}
			}
			if _, err := session.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
				return fmt.Errorf("record migration %s (non-tx): %w", name, err)
			}
			continue
		}

		// 默认迁移在事务中执行，确保原子性：要么完全成功，要么完全回滚。
		tx, err := session.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		// 执行迁移 SQL
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		// 记录迁移已完成，保存文件名和校验和
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// migrationUpSQL adapts direction annotations only; execution, transactions and
// the checksum of the complete original file remain owned by this runner.
func migrationUpSQL(content string) (string, error) {
	if !strings.Contains(content, "+goose") {
		return content, nil
	}
	up, down, statementBlock, sqlBeforeUp := false, false, false, false
	upEnd := len(content)
	for i := 0; i < len(content); {
		if strings.HasPrefix(content[i:], "--") {
			end := strings.IndexByte(content[i:], '\n')
			if end < 0 {
				end = len(content)
			} else {
				end += i
			}
			comment := strings.TrimSpace(content[i+2 : end])
			lineStart := strings.LastIndexByte(content[:i], '\n') + 1
			if strings.HasPrefix(comment, "+goose") {
				if strings.TrimSpace(content[lineStart:i]) != "" {
					return "", errors.New("goose annotations must be standalone SQL comments")
				}
				fields := strings.Fields(comment)
				if len(fields) != 2 || fields[0] != "+goose" {
					return "", errors.New("invalid or unsupported goose annotation")
				}
				switch strings.ToLower(fields[1]) {
				case "up":
					if up || down || sqlBeforeUp {
						return "", errors.New("goose Up must occur once, before executable SQL")
					}
					up = true
				case "down":
					if !up || down || statementBlock {
						return "", errors.New("goose Down must follow Up and a closed StatementBegin block")
					}
					down, upEnd = true, i
				case "statementbegin":
					if !up || statementBlock {
						return "", errors.New("goose StatementBegin requires a direction and cannot nest")
					}
					statementBlock = true
				case "statementend":
					if !statementBlock {
						return "", errors.New("goose StatementEnd has no matching StatementBegin")
					}
					statementBlock = false
				default:
					return "", fmt.Errorf("unsupported goose annotation %q", fields[1])
				}
			}
			i = end
			continue
		}
		if strings.HasPrefix(content[i:], "/*") {
			depth := 1
			i += 2
			for i < len(content) && depth > 0 {
				switch {
				case strings.HasPrefix(content[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(content[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return "", errors.New("unterminated SQL block comment in annotated migration")
			}
			continue
		}
		switch content[i] {
		case ' ', '\t', '\r', '\n':
			i++
			continue
		case '\'', '"':
			end, err := migrationQuotedSQLEnd(content, i)
			if err != nil {
				return "", err
			}
			i = end
		case '$':
			tag := migrationDollarQuoteTag(content, i)
			if tag == "" {
				i++
				break
			}
			end := strings.Index(content[i+len(tag):], tag)
			if end < 0 {
				return "", errors.New("unterminated SQL dollar quote in annotated migration")
			}
			i += len(tag)*2 + end
		default:
			i++
		}
		if !up {
			sqlBeforeUp = true
		}
	}
	if statementBlock {
		return "", errors.New("goose StatementBegin is missing StatementEnd")
	}
	if !up {
		return content, nil
	}
	return strings.TrimSpace(content[:upEnd]), nil
}

func migrationQuotedSQLEnd(content string, start int) (int, error) {
	quote := content[start]
	escape := quote == '\'' && start > 0 && (content[start-1] == 'e' || content[start-1] == 'E') &&
		(start == 1 || !migrationSQLIdentifierByte(content[start-2]))
	for i := start + 1; i < len(content); i++ {
		if escape && content[i] == '\\' {
			i++
			continue
		}
		if content[i] != quote {
			continue
		}
		if i+1 < len(content) && content[i+1] == quote {
			i++
			continue
		}
		return i + 1, nil
	}
	return 0, errors.New("unterminated SQL quote in annotated migration")
}

func migrationDollarQuoteTag(content string, start int) string {
	if start > 0 && migrationSQLIdentifierByte(content[start-1]) {
		return ""
	}
	for i := start + 1; i < len(content); i++ {
		if content[i] == '$' {
			return content[start : i+1]
		}
		if !migrationSQLIdentifierByte(content[i]) || (i == start+1 && content[i] >= '0' && content[i] <= '9') {
			return ""
		}
	}
	return ""
}

func migrationSQLIdentifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

func isMigrationUpChecksumCompatible(content, existing string) bool {
	upSQL, err := migrationUpSQL(content)
	if err != nil || upSQL == content {
		return false
	}
	// Some deployments stored the exact Up-only prefix of Goose files. Accept
	// only that content hash; never rewrite its record or ignore a changed Up.
	sum := sha256.Sum256([]byte(upSQL))
	return hex.EncodeToString(sum[:]) == existing
}

func prepareNonTransactionalMigration(ctx context.Context, db migrationExecutor, name string) error {
	switch name {
	case paymentOrdersOutTradeNoUniqueMigration:
		return preparePaymentOrdersOutTradeNoUniqueMigration(ctx, db)
	case schedulerOutboxPendingDedupKeyMigration:
		return dropInvalidIndexIfPresent(ctx, db, schedulerOutboxPendingDedupKeyIndex)
	case latestAPIKeyIPIndexMigration:
		return dropInvalidIndexIfPresent(ctx, db, latestAPIKeyIPIndex)
	case sharedPoolPriceVersionIndexesMigration:
		return dropInvalidIndexesIfPresent(ctx, db, sharedPoolPriceVersionIndexes)
	case sharedPoolRoutingHotPathIndexesMigration:
		return dropInvalidIndexesIfPresent(ctx, db, sharedPoolRoutingHotPathIndexes)
	case sharedPoolCompactCapabilityIndexMigration:
		return dropInvalidIndexIfPresent(ctx, db, sharedPoolCompactCapabilityIndex)
	default:
		return nil
	}
}

func dropInvalidIndexesIfPresent(ctx context.Context, db migrationExecutor, indexNames []string) error {
	for _, indexName := range indexNames {
		if err := dropInvalidIndexIfPresent(ctx, db, indexName); err != nil {
			return err
		}
	}
	return nil
}

func preparePaymentOrdersOutTradeNoUniqueMigration(ctx context.Context, db migrationExecutor) error {
	duplicates, err := findDuplicatePaymentOrderOutTradeNos(ctx, db)
	if err != nil {
		return fmt.Errorf("precheck duplicate out_trade_no: %w", err)
	}
	if len(duplicates) > 0 {
		return fmt.Errorf(
			"duplicate out_trade_no values block %s; remediate duplicates before retrying: %s",
			paymentOrdersOutTradeNoUniqueMigration,
			strings.Join(duplicates, ", "),
		)
	}

	return dropInvalidIndexIfPresent(ctx, db, paymentOrdersOutTradeNoUniqueIndex)
}

func dropInvalidIndexIfPresent(ctx context.Context, db migrationExecutor, indexName string) error {
	invalid, err := indexIsInvalid(ctx, db, indexName)
	if err != nil {
		return fmt.Errorf("check invalid index %s: %w", indexName, err)
	}
	if !invalid {
		return nil
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s", indexName)); err != nil {
		return fmt.Errorf("drop invalid index %s: %w", indexName, err)
	}
	return nil
}

func findDuplicatePaymentOrderOutTradeNos(ctx context.Context, db migrationExecutor) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT out_trade_no, COUNT(*) AS duplicate_count
		FROM payment_orders
		WHERE out_trade_no <> ''
		GROUP BY out_trade_no
		HAVING COUNT(*) > 1
		ORDER BY duplicate_count DESC, out_trade_no
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	duplicates := make([]string, 0, 5)
	for rows.Next() {
		var outTradeNo string
		var duplicateCount int
		if err := rows.Scan(&outTradeNo, &duplicateCount); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, fmt.Sprintf("%s (count=%d)", outTradeNo, duplicateCount))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

func indexIsInvalid(ctx context.Context, db migrationExecutor, indexName string) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_class idx
			JOIN pg_namespace ns ON ns.oid = idx.relnamespace
			JOIN pg_index i ON i.indexrelid = idx.oid
			WHERE ns.nspname = 'public'
			  AND idx.relname = $1
			  AND NOT i.indisvalid
		)
	`, indexName).Scan(&invalid)
	return invalid, err
}

func ensureAtlasBaselineAligned(ctx context.Context, db migrationExecutor, fsys fs.FS) error {
	hasLegacy, err := tableExists(ctx, db, "schema_migrations")
	if err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if !hasLegacy {
		return nil
	}

	hasAtlas, err := tableExists(ctx, db, "atlas_schema_revisions")
	if err != nil {
		return fmt.Errorf("check atlas_schema_revisions: %w", err)
	}
	if !hasAtlas {
		if _, err := db.ExecContext(ctx, atlasSchemaRevisionsTableDDL); err != nil {
			return fmt.Errorf("create atlas_schema_revisions: %w", err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM atlas_schema_revisions").Scan(&count); err != nil {
		return fmt.Errorf("count atlas_schema_revisions: %w", err)
	}
	if count > 0 {
		return nil
	}

	version, description, hash, err := latestMigrationBaseline(fsys)
	if err != nil {
		return fmt.Errorf("atlas baseline version: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO atlas_schema_revisions (version, description, type, applied, total, executed_at, execution_time, hash)
		VALUES ($1, $2, $3, 0, 0, NOW(), 0, $4)
	`, version, description, 1, hash); err != nil {
		return fmt.Errorf("insert atlas baseline: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, db migrationExecutor, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func latestMigrationBaseline(fsys fs.FS) (string, string, string, error) {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return "", "", "", err
	}
	if len(files) == 0 {
		return "baseline", "baseline", "", nil
	}
	sort.Strings(files)
	name := files[len(files)-1]
	contentBytes, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", "", "", err
	}
	content := strings.TrimSpace(string(contentBytes))
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	version := strings.TrimSuffix(name, ".sql")
	return version, version, hash, nil
}

func checksumSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func newMigrationChecksumCompatibilityRule(fileChecksum string, acceptedDBChecksums ...string) migrationChecksumCompatibilityRule {
	return migrationChecksumCompatibilityRule{
		fileChecksum:       fileChecksum,
		acceptedDBChecksum: checksumSet(acceptedDBChecksums...),
		acceptedChecksums:  checksumSet(append([]string{fileChecksum}, acceptedDBChecksums...)...),
	}
}

func isMigrationChecksumCompatible(name, dbChecksum, fileChecksum string) bool {
	rule, ok := migrationChecksumCompatibilityRules[name]
	if !ok {
		return false
	}
	_, dbOK := rule.acceptedChecksums[dbChecksum]
	if !dbOK {
		return false
	}
	_, fileOK := rule.acceptedChecksums[fileChecksum]
	return fileOK
}

func validateMigrationExecutionMode(name, content string) (bool, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	upperContent := strings.ToUpper(content)
	nonTx := strings.HasSuffix(normalizedName, nonTransactionalMigrationSuffix)

	if !nonTx {
		if strings.Contains(upperContent, "CONCURRENTLY") {
			return false, errors.New("CONCURRENTLY statements must be placed in *_notx.sql migrations")
		}
		return false, nil
	}

	if strings.Contains(upperContent, "BEGIN") || strings.Contains(upperContent, "COMMIT") || strings.Contains(upperContent, "ROLLBACK") {
		return false, errors.New("*_notx.sql must not contain transaction control statements (BEGIN/COMMIT/ROLLBACK)")
	}

	statements := splitSQLStatements(content)
	for _, stmt := range statements {
		normalizedStmt := strings.ToUpper(stripSQLLineComment(strings.TrimSpace(stmt)))
		if normalizedStmt == "" {
			continue
		}

		if strings.Contains(normalizedStmt, "CONCURRENTLY") {
			isCreateIndex := strings.Contains(normalizedStmt, "CREATE") && strings.Contains(normalizedStmt, "INDEX")
			isDropIndex := strings.Contains(normalizedStmt, "DROP") && strings.Contains(normalizedStmt, "INDEX")
			if !isCreateIndex && !isDropIndex {
				return false, errors.New("*_notx.sql currently only supports CREATE/DROP INDEX CONCURRENTLY statements")
			}
			if isCreateIndex && !strings.Contains(normalizedStmt, "IF NOT EXISTS") {
				return false, errors.New("CREATE INDEX CONCURRENTLY in *_notx.sql must include IF NOT EXISTS for idempotency")
			}
			if isDropIndex && !strings.Contains(normalizedStmt, "IF EXISTS") {
				return false, errors.New("DROP INDEX CONCURRENTLY in *_notx.sql must include IF EXISTS for idempotency")
			}
			continue
		}

		return false, errors.New("*_notx.sql must not mix non-CONCURRENTLY SQL statements")
	}

	return true, nil
}

func splitSQLStatements(content string) []string {
	parts := strings.Split(content, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func stripSQLLineComment(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// pgAdvisoryLock 获取 PostgreSQL Advisory Lock。
// Advisory Lock 是一种轻量级的锁机制，不与任何特定的数据库对象关联。
// 它非常适合用于应用层面的分布式锁场景，如迁移序列化。
func pgAdvisoryLock(ctx context.Context, db migrationExecutor) error {
	ticker := time.NewTicker(migrationsLockRetryInterval)
	defer ticker.Stop()

	for {
		var locked bool
		if err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationsAdvisoryLockID).Scan(&locked); err != nil {
			return fmt.Errorf("acquire migrations lock: %w", err)
		}
		if locked {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire migrations lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// pgAdvisoryUnlock 释放 PostgreSQL Advisory Lock。
// 必须在获取锁后确保释放，否则会阻塞其他实例的迁移操作。
func pgAdvisoryUnlock(ctx context.Context, db migrationExecutor) error {
	_, err := db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationsAdvisoryLockID)
	if err != nil {
		return fmt.Errorf("release migrations lock: %w", err)
	}
	return nil
}
