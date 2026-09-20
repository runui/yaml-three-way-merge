import { diffLines } from "diff";
import { useFocusedEditorScroll } from "./useFocusedEditorScroll";

interface Props {
  previous: string;
  next: string;
}

interface DiffRow {
  key: string;
  kind: "context" | "added" | "removed";
  oldLine?: number;
  newLine?: number;
  content: string;
}

function splitLines(value: string) {
  const lines = value.split("\n");
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

function buildRows(previous: string, next: string) {
  let oldLine = 1;
  let newLine = 1;
  let additions = 0;
  let deletions = 0;
  const rows: DiffRow[] = [];

  for (const [changeIndex, change] of diffLines(previous, next).entries()) {
    const kind = change.added ? "added" : change.removed ? "removed" : "context";
    for (const [lineIndex, content] of splitLines(change.value).entries()) {
      const row: DiffRow = { key: `${changeIndex}-${lineIndex}`, kind, content };
      if (kind !== "added") row.oldLine = oldLine++;
      if (kind !== "removed") row.newLine = newLine++;
      if (kind === "added") additions++;
      if (kind === "removed") deletions++;
      rows.push(row);
    }
  }
  return { rows, additions, deletions };
}

export function FinalDiff({ previous, next }: Props) {
  const { focused, focusHandlers } = useFocusedEditorScroll();
  const { rows, additions, deletions } = buildRows(previous, next);
  const hasInput = Boolean(previous || next);
  const hasChanges = additions > 0 || deletions > 0;

  return (
    <section className="diff-panel">
      <header className="panel-header">
        <div>
          <h2>最终结果 Diff</h2>
          <p>GitHub 风格 unified diff · Previous Effective → New Effective</p>
        </div>
      </header>
      <div
        className={`diff-shell${focused ? " focused" : ""}`}
        aria-label="最终结果 Diff"
        tabIndex={0}
        {...focusHandlers}
      >
        <div className="diff-file-header">
          <span className="diff-file-icon" aria-hidden="true">YML</span>
          <strong>previous-effective.yaml → new-effective.yaml</strong>
          {hasChanges && <span className="diff-stats"><b className="added">+{additions}</b><b className="removed">-{deletions}</b></span>}
        </div>
        {!hasInput && <div className="diff-empty">运行合并后显示最终结果差异</div>}
        {hasInput && !hasChanges && <div className="diff-empty">两个最终结果没有差异</div>}
        {hasChanges && (
          <table className="diff-table" aria-label="Previous Effective 到 New Effective 的行级差异">
            <tbody>
              <tr className="diff-hunk">
                <td />
                <td />
                <td>@@ -1,{splitLines(previous).length} +1,{splitLines(next).length} @@</td>
              </tr>
              {rows.map((row) => (
                <tr className={`diff-row ${row.kind}`} key={row.key}>
                  <td className="diff-line-number">{row.oldLine ?? ""}</td>
                  <td className="diff-line-number">{row.newLine ?? ""}</td>
                  <td><code><span className="diff-marker">{row.kind === "added" ? "+" : row.kind === "removed" ? "-" : " "}</span>{row.content || " "}</code></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}
