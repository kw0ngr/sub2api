package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsErrorInsertPreservesProviderCodeInSharedSingleAndBatchArgs(t *testing.T) {
	code := "1113"
	args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{ProviderErrorCode: &code})
	require.Len(t, args, 44)
	require.Equal(t, sql.NullString{String: code, Valid: true}, args[43])
	require.Contains(t, insertOpsErrorLogSQL, "provider_error_code")
	require.Contains(t, insertOpsErrorLogSQL, "$44")
	require.Equal(t, sql.NullString{}, opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{})[43])
}
