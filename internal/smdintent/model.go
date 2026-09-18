package smdintent

type ChangeSummary struct {
	Added    int      `json:"added"`
	Modified int      `json:"modified"`
	Removed  int      `json:"removed"`
	Paths    []string `json:"paths,omitempty"`
}

type ConflictSummary struct {
	WriteWrite  int `json:"write_write"`
	WriteRemove int `json:"write_remove"`
	RemoveWrite int `json:"remove_write"`
	BothRemove  int `json:"both_remove"`
}

type Report struct {
	User                ChangeSummary   `json:"user"`
	Upstream            ChangeSummary   `json:"upstream"`
	Conflicts           ConflictSummary `json:"conflicts"`
	IndependentUser     int             `json:"independent_user"`
	IndependentUpstream int             `json:"independent_upstream"`
	OriginRewrites      int             `json:"origin_rewrites"`
	UserReplayValid     bool            `json:"user_replay_valid"`
	UpstreamReplayValid bool            `json:"upstream_replay_valid"`
}

type Result struct {
	YAML   []byte
	Report Report
}
