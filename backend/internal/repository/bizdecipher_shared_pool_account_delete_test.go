package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Regression: soft-delete used UPDATE ... FROM shared_pools with unqualified
// disabled_reason. Both tables have that column, so PostgreSQL returned
// "column reference \"disabled_reason\" is ambiguous" and the API surfaced HTTP 500.
func TestDeleteSharedPoolAccountSQLQualifiesAmbiguousColumns(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(file), "bizdecipher_repo.go")
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read repo source: %v", err)
	}
	src := string(raw)
	marker := "func (r *bizDecipherRepository) DeleteSharedPoolAccount"
	idx := strings.Index(src, marker)
	if idx < 0 {
		t.Fatal("DeleteSharedPoolAccount not found")
	}
	chunk := src[idx:]
	end := strings.Index(chunk, "\nfunc (")
	if end > 0 {
		chunk = chunk[:end]
	}
	if !strings.Contains(chunk, "NULLIF(spa.disabled_reason, '')") {
		t.Fatalf("DeleteSharedPoolAccount must qualify spa.disabled_reason to avoid ambiguous column with shared_pools; got:\n%s", chunk)
	}
	if strings.Contains(chunk, "NULLIF(disabled_reason, '')") {
		t.Fatalf("DeleteSharedPoolAccount still references unqualified disabled_reason:\n%s", chunk)
	}
	if !strings.Contains(chunk, "schedulable = FALSE") {
		t.Fatalf("soft delete should clear schedulable:\n%s", chunk)
	}
}
