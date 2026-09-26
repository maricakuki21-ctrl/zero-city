package repository

import (
	"strings"
	"testing"
)

func TestCreateSharedPoolQueryKeepsInsertColumnsAndValuesAligned(t *testing.T) {
	columnsStart := strings.Index(createSharedPoolQuery, "(")
	columnsEnd := strings.Index(createSharedPoolQuery, ") VALUES (")
	valuesStart := columnsEnd + len(") VALUES (")
	valuesEnd := strings.Index(createSharedPoolQuery[valuesStart:], ")\nRETURNING")
	if columnsStart < 0 || columnsEnd < 0 || valuesEnd < 0 {
		t.Fatalf("unexpected shared pool insert query shape: %s", createSharedPoolQuery)
	}
	valuesEnd += valuesStart

	columns := splitSQLList(createSharedPoolQuery[columnsStart+1 : columnsEnd])
	values := splitSQLList(createSharedPoolQuery[valuesStart:valuesEnd])
	if len(columns) != len(values) {
		t.Fatalf("shared pool insert has %d target columns but %d values", len(columns), len(values))
	}
	if !strings.Contains(createSharedPoolQuery, "verification_exemption_reason, platform_fee_percent, quality_score") {
		t.Fatal("shared pool insert must persist the configured platform fee percent")
	}
	if strings.Contains(createSharedPoolQuery, "verification_exemption_reason, owner_share_percent, quality_score") {
		t.Fatal("shared pool insert must not derive a new pool from a hard-coded owner share")
	}
	if !strings.Contains(createSharedPoolQuery, "$27, 90.0") {
		t.Fatal("shared pool insert must bind platform_fee_percent before the fixed quality score")
	}
	if !strings.Contains(createSharedPoolQuery, "quality_score, native_onboarding_state") || !strings.Contains(createSharedPoolQuery, "90.0, 'legacy_existing'") {
		t.Fatal("legacy shared pool creation must explicitly preserve legacy credential mode")
	}
}

func splitSQLList(value string) []string {
	out := []string{}
	start := 0
	depth := 0
	inSingleQuote := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'':
			if inSingleQuote && i+1 < len(value) && value[i+1] == '\'' {
				i++
				continue
			}
			inSingleQuote = !inSingleQuote
		case '(':
			if !inSingleQuote {
				depth++
			}
		case ')':
			if !inSingleQuote && depth > 0 {
				depth--
			}
		case ',':
			if !inSingleQuote && depth == 0 {
				if part := strings.Join(strings.Fields(value[start:i]), " "); part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}
	if part := strings.Join(strings.Fields(value[start:]), " "); part != "" {
		out = append(out, part)
	}
	return out
}
