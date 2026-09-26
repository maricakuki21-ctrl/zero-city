package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCommunityOwnerStatusOnlyAllowsUserRuntimeStates(t *testing.T) {
	require.Equal(t, "open", normalizeCommunityOwnerStatus("open"))
	require.Equal(t, "answered", normalizeCommunityOwnerStatus("answered"))
	require.Equal(t, "resolved", normalizeCommunityOwnerStatus("resolved"))

	require.Empty(t, normalizeCommunityOwnerStatus("confirmed"))
	require.Empty(t, normalizeCommunityOwnerStatus("hidden"))
	require.Empty(t, normalizeCommunityOwnerStatus("deleted"))
	require.Empty(t, normalizeCommunityOwnerStatus("rejected"))
}

func TestNormalizeCommunityStatusAllowsAdminModerationStates(t *testing.T) {
	for _, status := range []string{"open", "reviewing", "answered", "confirmed", "resolved", "hidden", "deleted", "rejected"} {
		require.Equal(t, status, normalizeCommunityStatus(status))
	}
	require.Empty(t, normalizeCommunityStatus("accepted"))
	require.Empty(t, normalizeCommunityStatus("featured"))
}

func TestNormalizeCommunityCommentStatusAllowsRuntimeCommentStates(t *testing.T) {
	for _, status := range []string{"visible", "accepted", "confirmed", "resolved", "hidden", "deleted"} {
		require.Equal(t, status, normalizeCommunityCommentStatus(status))
	}
	require.Empty(t, normalizeCommunityCommentStatus("rejected"))
	require.Empty(t, normalizeCommunityCommentStatus("pinned"))
}
