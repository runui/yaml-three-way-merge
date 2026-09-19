package merge

import "github.com/runui/yaml-three-way-merge/internal/smdintent"

// ChangeSummary counts logical paths, not serialized YAML lines.
type ChangeSummary struct {
	Added    int      `json:"added"`
	Modified int      `json:"modified"`
	Removed  int      `json:"removed"`
	Paths    []string `json:"paths,omitempty"`
}

// ConflictSummary counts overlapping field-path operations on the two sides.
// Counts are diagnostic; they are not necessarily distinct resource counts.
type ConflictSummary struct {
	WriteWrite  int `json:"write_write"`
	WriteRemove int `json:"write_remove"`
	RemoveWrite int `json:"remove_write"`
	BothRemove  int `json:"both_remove"`
}

// Report describes extracted intent and replay validation.
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

// Result contains effective Compose YAML and its diagnostic report.
type Result struct {
	YAML   []byte
	Report Report
}

// Explicit conversion keeps the public contract independent of engine types.
func publicReport(r smdintent.Report) Report {
	return Report{
		User:            ChangeSummary{r.User.Added, r.User.Modified, r.User.Removed, append([]string(nil), r.User.Paths...)},
		Upstream:        ChangeSummary{r.Upstream.Added, r.Upstream.Modified, r.Upstream.Removed, append([]string(nil), r.Upstream.Paths...)},
		Conflicts:       ConflictSummary{r.Conflicts.WriteWrite, r.Conflicts.WriteRemove, r.Conflicts.RemoveWrite, r.Conflicts.BothRemove},
		IndependentUser: r.IndependentUser, IndependentUpstream: r.IndependentUpstream,
		OriginRewrites: r.OriginRewrites, UserReplayValid: r.UserReplayValid, UpstreamReplayValid: r.UpstreamReplayValid,
	}
}
