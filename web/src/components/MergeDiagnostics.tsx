import type { MergeDiagnostics as Diagnostics } from "../types";

export function MergeDiagnostics({ value }: { value: Diagnostics }) {
  const conflictCount = Object.values(value.conflicts).reduce((total, count) => total + count, 0);
  return (
    <section className="diagnostics" aria-label="Intent diagnostics">
      <strong>Intent 诊断</strong>
      <span>用户 +{value.user.added} / ~{value.user.modified} / -{value.user.removed}</span>
      <span>上游 +{value.upstream.added} / ~{value.upstream.modified} / -{value.upstream.removed}</span>
      <span>冲突 {conflictCount}</span>
      <span>独立变更 用户 {value.independent_user} / 上游 {value.independent_upstream}</span>
      <span>Replay {value.user_replay_valid && value.upstream_replay_valid ? "通过" : "失败"}</span>
    </section>
  );
}
