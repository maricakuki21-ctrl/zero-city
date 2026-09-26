package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMigrationChecksumCompatible(t *testing.T) {
	t.Run("054历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.True(t, ok)
	})

	t.Run("054在未知文件checksum下不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"0000000000000000000000000000000000000000000000000000000000000000",
		)
		require.False(t, ok)
	})

	t.Run("061历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("061第二个历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("非白名单迁移不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"001_init.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.False(t, ok)
	})

	t.Run("109历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
		)
		require.True(t, ok)
	})

	t.Run("109当前checksum可兼容历史checksum", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
		)
		require.True(t, ok)
	})

	t.Run("109回滚到历史文件后仍兼容已应用的新checksum", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
		)
		require.True(t, ok)
	})

	t.Run("110历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"110_pending_auth_and_provider_default_grants.sql",
			"e3d1f433be2b564cfbdc549adf98fce13c5c7b363ebc20fd05b765d0563b0925",
			"32cf87ee787b1bb36b5c691367c96eee37518fa3eed6f3322cf68795e3745279",
		)
		require.True(t, ok)
	})

	t.Run("112历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"112_add_payment_order_provider_key_snapshot.sql",
			"ffd3e8a2c9295fa9cbefefd629a78268877e5b51bc970a82d9b3f46ec4ebd15e",
			"b75f8f56d39455682787696a3d92ad25b055444ca328fb7fca9a460a15d68d99",
		)
		require.True(t, ok)
	})

	t.Run("115历史checksum可兼容修复后的legacy external backfill", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"115_auth_identity_legacy_external_backfill.sql",
			"4cf39e508be9fd1a5aa41610cbbebeb80385c9adda45bf78a706de9db4f1385f",
			"022aadd97bb53e755f0cf7a3a957e0cb1a1353b0c39ec4de3234acd2871fd04f",
		)
		require.True(t, ok)
	})

	t.Run("116历史checksum可兼容修复后的legacy external safety reports", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"116_auth_identity_legacy_external_safety_reports.sql",
			"f7757bd929ac67ffb08ce69fa4cf20fad39dbff9d5a5085fb2adabb7607e5877",
			"07edb09fa8d04ffb172b0621e3c22f4d1757d20a24ae267b3b36b087ab72d488",
		)
		require.True(t, ok)
	})

	t.Run("119历史checksum可兼容占位文件", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"119_enforce_payment_orders_out_trade_no_unique.sql",
			"ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34",
			"0bbe809ae48a9d811dabda1ba1c74955bd71c4a9cc610f9128816818dfa6c11e",
		)
		require.True(t, ok)
	})

	t.Run("118多个历史checksum都可兼容当前版本", func(t *testing.T) {
		for _, dbChecksum := range []string{
			"a38243ca0a72c3a01c0a92b7986423054d6133c0399441f853b99802852720fb",
			"e0cdf835d6c688d64100f483d31bc02ac9ebad414bf1837af239a84bf75b8227",
		} {
			ok := isMigrationChecksumCompatible(
				"118_wechat_dual_mode_and_auth_source_defaults.sql",
				dbChecksum,
				"b54194d7a3e4fbf710e0a3590d22a2fe7966804c487052a356e0b55f53ef96b0",
			)
			require.True(t, ok)
		}
	})

	t.Run("120多个历史checksum都可兼容新的notx修复版本", func(t *testing.T) {
		for _, dbChecksum := range []string{
			"e77921f79d539bc24575cb9c16cbe566d2b23ce816190343d0a7568f6a3fcf61",
			"707431450603e70a43ce9fbd61e0c12fa67da4875158ccefabacea069587ab22",
			"04b082b5a239c525154fe9185d324ee2b05ff90da9297e10dba19f9be79aa59a",
		} {
			ok := isMigrationChecksumCompatible(
				"120_enforce_payment_orders_out_trade_no_unique_notx.sql",
				dbChecksum,
				"34aadc0db59a4e390f92a12b73bd74642d9724f33124f73638ae00089ea5e074",
			)
			require.True(t, ok)
		}
	})

	t.Run("119未知checksum不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"119_enforce_payment_orders_out_trade_no_unique.sql",
			"ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34",
			"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		)
		require.False(t, ok)
	})

	t.Run("151测试服历史checksum兼容当前资产层迁移", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"151_bizdecipher_asset_layer.sql",
			"252f70be9b00a16ac305226096d8496b53d9c2991c863c7921309bb7132e84ea",
			"c3e2d30a5ffb619b2eb114d30d4de5e052d8194794790dd98d64218c0a49f84e",
		)
		require.True(t, ok)
	})

	t.Run("151未知checksum仍拒绝兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"151_bizdecipher_asset_layer.sql",
			"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			"c3e2d30a5ffb619b2eb114d30d4de5e052d8194794790dd98d64218c0a49f84e",
		)
		require.False(t, ok)
	})

	t.Run("测试服已知历史迁移checksum兼容当前release", func(t *testing.T) {
		cases := []struct {
			name         string
			dbChecksum   string
			fileChecksum string
		}{
			{"152_bizdecipher_launch_settings.sql", "dde6b0b3dc48697c00c910cb5dc03162317da4ff59b6c6777c61ef895dfd9fd9", "f120aebd8d4d139e22e187e6f94719498f9e07c49a6febf2592dd714fe375213"},
			{"159_promo_campaigns.sql", "3215014066feccb6f69f33e7527fa2cb52b0a7608b922e11117fc6ad1537428e", "c8584aab2e5dead3018db34a1047eac7cccf99608d1dd9f73ab6c30550a937bf"},
			{"160_shared_pools.sql", "0e328d623855d0ef6cb7d04035e0a6de1aff5105a731442bc78c58b6ad4d18fa", "ad875526bb6608f8ab1ac5b3a3e5ab014fa3be34b587655b7771ce4e52628080"},
			{"161_pool_seats.sql", "28e5baa913bd00c7434caa2745a03adbf27e330450b5147b199bb8032381fcc2", "9d4f5af31029540c885fd9aa40d31bbe250b3333e5ab4cbe163988ea970b4aea"},
			{"162_shared_pool_access_keys.sql", "93913f8677f2e1d412e9ba33148340cc924efab937f48b9e37734941eac05d49", "4c31793351f15c91ec8433515c2da8e49531eabd1660a95e60ce0405249fe84d"},
			{"163_disable_seeded_register_promo.sql", "0438497da66245799b53af72d1514003b27305c6571c2d5fcb4aa6d527e53be0", "cc306abca89ba30efbd43c988d970ecb6a7a5ffc095d563f0dafe6c8f3038fff"},
			{"164_bizdecipher_brand_defaults.sql", "7b3a56d5cfc3dcf2c393a035d5f595f0d75b7bcccd73838e224dfeac258a8434", "1dc0222eecdbab1d523820adfc1086e2bda73a473901c9ac16312397e3185659"},
			{"165_invite_reward_config_defaults.sql", "c76e55ae2ce0a925100d746b41c793f8380f8ab6279bfcd514dbd2192f9ecb09", "bac778ce2facde481bee1d0377b1f68015a60c425eeb89478b3553ddaec0ec5b"},
			{"167_account_square_model_catalog.sql", "5d4fb16fefc204089408c4a414b21bf091e92a82bb36ec3f3c68d8aef2f036c0", "b1053310148f1050be80b097621da9a2a05c0e7adfafd258946a228c9c8e9d3e"},
			{"168_shared_pool_publish_system.sql", "af12006fffef8ec28f01e9d448e827b55a598a195a632d468c7fed02bc5d9891", "37e5291a623e7498f4008f0a82aac7b5dc44e4deb4a92ce43e1166f837deb72e"},
			{"169_shared_pool_model_usage_windows.sql", "af73d0e32cd65fc38faca3c4bde1bec763badb92f030a6668c358a14a2c2212d", "b770e23cd760de04ce656329c7dc17bee11b63d7dddfb01b0d317488359512a1"},
			{"170_balance_credit_dual_track.sql", "3dcf24559f699783488f27a0dd2afd5c08540407653aa76c82da68af8bf43754", "6f7205319d2c531a3f88966264b63dfd6cecac8a0c507467b7230487de4f49d8"},
			{"171_signup_email_starter_credit.sql", "94ce4bf0e6f2bc07e58ad3946d3aa0d98ec05c9eb45d9998dc2186b3e4b0dc41", "6396aaeb99a86835e6685cd477e8532febba9935d2a06d3f6b1e34db6e3dc4dd"},
			{"172_daily_checkin_blind_box.sql", "5ef130064def26e8fc5df1db8bc4e09c8e7d274f337bd94a7c6d79144b6d3af5", "fb7a7947cfc855d4f276837fddafc4157ee657f2748a49b7ea03ef808d9a558f"},
			{"174_api_key_starter_credit_trigger.sql", "b325ff957f08976ff876cbd5324ec4ea2af38a9ba9d4f528fa8b59187b749b93", "dadcd675465bbbde37f606d854eb54b7a20996080f4f82cf1646ee21e96affd7"},
			{"177_daily_fortune_credit_balance_checkin.sql", "8179caa7ed8fbd8a0c9b4023d4d1dee35873ebbf8e5dcf65e4fef63ed5f62ea8", "6734ab51c477b7bffd7a583061658878ed384d1176d5b51810b4e20799bc4a65"},
			{"178_daily_checkin_multi_credit_balance.sql", "f20ec3ab245fb804fa8cf0246099f85b8c92fc666af16eedc444e844bfc6be0f", "230ad46e11c3dbb9d1d67919338b3ac1d36f11a39a9c91833b78354f1f5fb3a8"},
			{"182_daily_checkin_collectible_cards.sql", "8b5ae305fb2ccfd3c96714a6ab444637053e97332a2d87ad6074afa9301310b6", "9813612cdf6fb8b6be5dc8c8f64354b41500e444ddac5d0ff42ae7a618de9d6a"},
			{"192_shared_pool_seat_fee_waiver.sql", "bf38e242bf9325b7cbe53d4cb9ebb4b88a12141f39988ffd399daebaab7cb511", "41c71c2eaa9e3c508ad3027d7606ab166068dfffef5ccc64d7f14d334c2ff48a"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				require.True(t, isMigrationChecksumCompatible(tc.name, tc.dbChecksum, tc.fileChecksum))
				require.False(t, isMigrationChecksumCompatible(tc.name, tc.dbChecksum, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
			})
		}
	})
}
