import { lazy, startTransition, Suspense, useEffect, useState } from "react";
import { getAlgorithms, getFixture, getFixtures, runMerge } from "./api";
import { MergeDiagnostics } from "./components/MergeDiagnostics";
import type { Algorithm, AlgorithmID, FixtureDetail, FixtureSummary, MergeResponse } from "./types";

const emptyInputs = { previousBase: "", userOverride: "", targetBase: "" };
const EditorPanel = lazy(() => import("./components/EditorPanel").then((module) => ({ default: module.EditorPanel })));
const FinalDiff = lazy(() => import("./components/FinalDiff").then((module) => ({ default: module.FinalDiff })));

export default function App() {
  const [algorithms, setAlgorithms] = useState<Algorithm[]>([]);
  const [algorithm, setAlgorithm] = useState<AlgorithmID>("intent");
  const [fixtures, setFixtures] = useState<FixtureSummary[]>([]);
  const [fixtureTotal, setFixtureTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [selectedFixture, setSelectedFixture] = useState<FixtureDetail | null>(null);
  const [isCustom, setIsCustom] = useState(false);
  const [inputs, setInputs] = useState(emptyInputs);
  const [result, setResult] = useState<MergeResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([getAlgorithms(), getFixtures("")])
      .then(([algorithmItems, fixturePage]) => {
        setAlgorithms(algorithmItems);
        setFixtures(fixturePage.items);
        setFixtureTotal(fixturePage.total);
      })
      .catch((reason: Error) => setError(reason.message));
  }, []);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      getFixtures(query)
        .then((page) => startTransition(() => {
          setFixtures(page.items);
          setFixtureTotal(page.total);
        }))
        .catch((reason: Error) => setError(reason.message));
    }, 250);
    return () => window.clearTimeout(timeout);
  }, [query]);

  const executeMerge = async (nextAlgorithm = algorithm, includeExpected = !isCustom) => {
    if (!inputs.previousBase.trim() || !inputs.targetBase.trim()) {
      setError("Base YAML 和 New Base YAML 不能为空");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await runMerge({
        algorithm: nextAlgorithm,
        previous_base: inputs.previousBase,
        user_override: inputs.userOverride,
        target_base: inputs.targetBase,
        ...(includeExpected && selectedFixture ? { expected: selectedFixture.expected } : {}),
      });
      setResult(response);
    } catch (reason) {
      setResult(null);
      setError(reason instanceof Error ? reason.message : "合并失败");
    } finally {
      setLoading(false);
    }
  };

  const chooseFixture = async (id: string) => {
    if (!id) return;
    setLoading(true);
    setError("");
    try {
      const fixture = await getFixture(id);
      const nextInputs = {
        previousBase: fixture.previous_base,
        userOverride: fixture.user_override,
        targetBase: fixture.target_base,
      };
      setSelectedFixture(fixture);
      setInputs(nextInputs);
      setIsCustom(false);
      const response = await runMerge({
        algorithm,
        previous_base: nextInputs.previousBase,
        user_override: nextInputs.userOverride,
        target_base: nextInputs.targetBase,
        expected: fixture.expected,
      });
      setResult(response);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "加载 fixture 失败");
    } finally {
      setLoading(false);
    }
  };

  const updateInput = (key: keyof typeof inputs, value: string) => {
    setInputs((current) => ({ ...current, [key]: value }));
    setIsCustom(true);
    setResult(null);
  };

  const changeAlgorithm = (next: AlgorithmID) => {
    setAlgorithm(next);
    if (inputs.previousBase.trim() && inputs.targetBase.trim()) void executeMerge(next, !isCustom);
  };

  const reset = () => {
    if (selectedFixture) {
      setInputs({
        previousBase: selectedFixture.previous_base,
        userOverride: selectedFixture.user_override,
        targetBase: selectedFixture.target_base,
      });
      setIsCustom(false);
      setResult(null);
      return;
    }
    setInputs(emptyInputs);
    setResult(null);
  };

  const activeAlgorithm = algorithms.find((item) => item.id === algorithm);

  return (
    <main>
      <header className="page-header">
        <div>
          <p className="eyebrow">COMPOSE YAML LAB</p>
          <h1>三方合并可视化</h1>
          <p className="subtitle">观察用户覆盖层如何从旧基线迁移到新基线，并比较升级前后的有效配置。</p>
        </div>
        <div className={`mode-badge ${isCustom ? "custom" : ""}`}>{isCustom ? "自定义输入" : selectedFixture ? "Fixture" : "未选择"}</div>
      </header>

      <section className="toolbar">
        <label>
          <span>算法</span>
          <select aria-label="算法" value={algorithm} onChange={(event) => changeAlgorithm(event.target.value as AlgorithmID)}>
            {(algorithms.length ? algorithms : [{ id: "intent", name: "Intent SMD" } as Algorithm]).map((item) => (
              <option key={item.id} value={item.id}>{item.name}</option>
            ))}
          </select>
        </label>
        <label className="fixture-search">
          <span>Fixture 搜索</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="字段、场景或用例 ID" />
        </label>
        <label className="fixture-select">
          <span>Fixture（{fixtureTotal.toLocaleString()}）</span>
          <select aria-label="Fixture" value={selectedFixture?.metadata.id ?? ""} onChange={(event) => void chooseFixture(event.target.value)}>
            <option value="">选择演示用例</option>
            {fixtures.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}
          </select>
        </label>
        <button className="primary-button" type="button" disabled={loading} onClick={() => void executeMerge()}>{loading ? "处理中..." : "运行合并"}</button>
        <button type="button" disabled={loading} onClick={reset}>重置</button>
      </section>

      <section className="context-bar">
        <span>{activeAlgorithm?.description ?? "加载算法信息..."}</span>
        {selectedFixture && <span><b>{selectedFixture.metadata.field}</b> · {selectedFixture.metadata.scenario}</span>}
        {result?.matches_expected !== undefined && (
          <span className={result.matches_expected ? "match" : "mismatch"}>{result.matches_expected ? "匹配 Expected" : "不匹配 Expected"}</span>
        )}
      </section>

      {selectedFixture && !isCustom && <p className="fixture-description">{selectedFixture.metadata.description}</p>}
      {error && <div className="error-box" role="alert">{error}</div>}
      {result?.diagnostics && <MergeDiagnostics value={result.diagnostics} />}

      <Suspense fallback={<div className="editor-loading">正在加载代码编辑器...</div>}>
        <div className="editor-grid">
          <EditorPanel title="Base YAML" detail="旧版仓库基线" value={inputs.previousBase} onChange={(value) => updateInput("previousBase", value)} />
          <EditorPanel title="User Override" detail="相对于旧基线的用户覆盖层" value={inputs.userOverride} onChange={(value) => updateInput("userOverride", value)} />
          <EditorPanel title="New Base YAML" detail="待升级的新仓库基线" value={inputs.targetBase} onChange={(value) => updateInput("targetBase", value)} />
        </div>

        <div className="flow-label"><span>算法处理结果</span></div>

        <div className="editor-grid output-grid">
          <EditorPanel title="Previous Effective" detail="Base + User Override" value={result?.previous_effective ?? ""} readOnly />
          <EditorPanel title="New User Override" detail={algorithm === "rebase" ? "算法原生生成" : "从算法结果反推生成"} value={result?.new_override ?? ""} readOnly />
          <EditorPanel title="New Effective" detail="New Base + New User Override" value={result?.new_effective ?? ""} readOnly />
        </div>

        <FinalDiff previous={result?.previous_effective ?? ""} next={result?.new_effective ?? ""} />
      </Suspense>
    </main>
  );
}
