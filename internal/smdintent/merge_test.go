package smdintent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeReplaysBothIntents(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n    environment: [A=old]\n"),
		[]byte("services:\n  app:\n    environment: [A=user]\n"),
		[]byte("services:\n  app:\n    image: busybox:new\n    environment: [A=remote, B=remote]\n"),
	)
	require.NoError(t, err)
	assert.True(t, result.Report.UserReplayValid)
	assert.True(t, result.Report.UpstreamReplayValid)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox:new\n    environment: {A: user, B: remote}\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestMergeKeepsIndependentStableItems(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    volumes:\n      - {type: bind, source: /old-a, target: /a}\n      - {type: bind, source: /old-b, target: /b}\n"),
		[]byte("services:\n  app:\n    volumes: !reset []\n---\nservices:\n  app:\n    volumes:\n      - {type: bind, source: /user-a, target: /a}\n      - {type: bind, source: /old-b, target: /b}\n"),
		[]byte("services:\n  app:\n    volumes:\n      - {type: bind, source: /old-a, target: /a}\n      - {type: bind, source: /remote-b, target: /b}\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    volumes:\n      - {type: bind, source: /user-a, target: /a}\n      - {type: bind, source: /remote-b, target: /b}\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
	assert.Greater(t, result.Report.IndependentUser, 0)
	assert.Greater(t, result.Report.IndependentUpstream, 0)
}

func TestMergeKeepsDifferentConcurrentAddsWithoutOldOrigin(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n"),
		[]byte("services:\n  app:\n    cap_add: [SYS_ADMIN]\n"),
		[]byte("services:\n  app:\n    image: busybox\n    cap_add: [CHOWN]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox\n    cap_add: [CHOWN, SYS_ADMIN]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestMergeTreatsOrderedListAsAtomic(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    env_file: [base-a.env, base-b.env]\n"),
		[]byte("services:\n  app:\n    env_file: !reset []\n---\nservices:\n  app:\n    env_file: [user-a.env, base-b.env]\n"),
		[]byte("services:\n  app:\n    env_file: [base-a.env, remote-b.env]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    env_file: [user-a.env, base-b.env]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
	assert.Greater(t, result.Report.Conflicts.WriteWrite, 0)
}

func TestReportDistinguishesUpstreamModifyFromAdd(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    cap_add: [NET_ADMIN]\n"),
		[]byte("{}\n"),
		[]byte("services:\n  app:\n    cap_add: [SYS_ADMIN, CHOWN]\n"),
	)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Report.Upstream.Modified)
	assert.Equal(t, 1, result.Report.Upstream.Added)
	assert.Equal(t, 0, result.Report.Upstream.Removed)
	assert.Equal(t, 0, result.Report.User.Added)
	assert.Equal(t, 0, result.Report.User.Modified)
	assert.Equal(t, 0, result.Report.User.Removed)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    cap_add: [SYS_ADMIN, CHOWN]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestUserDeleteKeepsIndependentUpstreamMapSibling(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n    build:\n      args: [B=old]\n"),
		[]byte("services:\n  app:\n    build: !reset {}\n"),
		[]byte("services:\n  app:\n    image: busybox\n    build:\n      args: [A=remote, B=old]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox\n    build:\n      args: {A: remote}\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestUserClearOldListKeepsIndependentUpstreamItem(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n    volumes:\n      - {type: bind, source: /old, target: /old}\n"),
		[]byte("services:\n  app:\n    volumes: !reset []\n"),
		[]byte("services:\n  app:\n    image: busybox\n    volumes:\n      - {type: bind, source: /old, target: /old}\n      - {type: bind, source: /remote, target: /remote}\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox\n    volumes:\n      - {type: bind, source: /remote, target: /remote}\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestNamedOriginBothModifyUserWins(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    depends_on: [base-dependency-a]\n"),
		[]byte("services:\n  app:\n    depends_on: !reset []\n---\nservices:\n  app:\n    depends_on: [user-dependency-a]\n"),
		[]byte("services:\n  app:\n    depends_on: [remote-dependency-a]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    depends_on: [user-dependency-a]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
	assert.Equal(t, 1, result.Report.OriginRewrites)
}

func TestNamedOriginUserDeleteWinsRemoteRename(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n    networks: [base-net-a]\n"),
		[]byte("services:\n  app:\n    networks: !reset []\n"),
		[]byte("services:\n  app:\n    image: busybox\n    networks: [remote-net-a]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestNamedOriginConcurrentAddsOnSameOriginUserWins(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app: {}\n"),
		[]byte("services:\n  app:\n    networks: [user-net-a]\n"),
		[]byte("services:\n  app:\n    networks: [remote-net-a]\n"),
	)
	require.NoError(t, err)
	// Both additions carry the same stable origin token, so they identify the
	// same logical item and the user's version wins.
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    networks: [user-net-a]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
	assert.Equal(t, 1, result.Report.OriginRewrites)
}

func TestMergeKeepsDifferentConcurrentAddsWithDistinctOrigins(t *testing.T) {
	result, err := Merge(
		[]byte("services:\n  app: {}\n"),
		[]byte("services:\n  app:\n    networks: [user-net-a]\n"),
		[]byte("services:\n  app:\n    networks: [remote-net-b]\n"),
	)
	require.NoError(t, err)
	// Distinct origin tokens identify different logical items and are both kept.
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    networks: [remote-net-b, user-net-a]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}
