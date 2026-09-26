package repository

import (
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAdminGroupSourcePredicate(t *testing.T) {
	for _, source := range []string{"operator", "shared", "dormant", "history"} {
		t.Run(source, func(t *testing.T) {
			table := entsql.Table("groups")
			q := entsql.Dialect(dialect.Postgres).Select(table.C("id")).From(table)
			adminGroupSourcePredicate(source)(q)
			query, args := q.Query()
			if len(args) != 0 || !strings.Contains(query, "shared_pool_sub2_bindings") {
				t.Fatalf("unexpected source predicate: %s", query)
			}
			if strings.Contains(query, "biz-pool-") {
				t.Fatal("must not classify arbitrary user-facing names")
			}
			if source != "operator" && !strings.Contains(query, "p.deleted_at IS NULL") {
				t.Fatal("must exclude deleted pools from current managed groups")
			}
			if source == "history" && !strings.Contains(query, "AND NOT EXISTS") {
				t.Fatal("history must exclude current bindings")
			}
		})
	}
}
