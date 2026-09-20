import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { getAlgorithms, getFixture, getFixtures, runMerge } from "./api";

vi.mock("./api", () => ({
  getAlgorithms: vi.fn(),
  getFixture: vi.fn(),
  getFixtures: vi.fn(),
  runMerge: vi.fn(),
}));

vi.mock("@monaco-editor/react", () => ({
  default: ({ value, onChange, options }: { value: string; onChange?: (value: string) => void; options?: { readOnly?: boolean; scrollbar?: { handleMouseWheel?: boolean; alwaysConsumeMouseWheel?: boolean } } }) => (
    <textarea data-consume-wheel={String(options?.scrollbar?.alwaysConsumeMouseWheel)} aria-label={options?.readOnly ? "read-only-yaml" : "editable-yaml"} value={value} readOnly={options?.readOnly} onChange={(event) => onChange?.(event.target.value)} />
  ),
}));

const fixture = {
  metadata: {
    id: "atomic/service.image/single/12-both-modify-different-user-wins",
    field: "services.app.image",
    class: "atomic",
    scenario: "12-both-modify-different-user-wins",
    description: "User value wins.",
  },
  previous_base: "image: old\n",
  user_override: "image: user\n",
  target_base: "image: new\n",
  expected: "image: user\n",
};

const mergeResult = {
  previous_effective: "image: user-old\n",
  new_override: "image: user\n",
  new_effective: "image: user-new\n",
  matches_expected: true,
};

describe("App", () => {
  afterEach(cleanup);

  beforeEach(() => {
    vi.mocked(getAlgorithms).mockResolvedValue([
      { id: "intent", name: "Intent SMD", description: "intent", output_type: "effective" },
      { id: "direct", name: "Direct SMD", description: "direct", output_type: "effective" },
      { id: "rebase", name: "Project Rebase", description: "rebase", output_type: "override" },
    ]);
    vi.mocked(getFixtures).mockResolvedValue({ items: [fixture.metadata], total: 28630, page: 1, page_size: 30 });
    vi.mocked(getFixture).mockResolvedValue(fixture);
    vi.mocked(runMerge).mockResolvedValue(mergeResult);
  });

  it("loads a fixture, runs it, and displays the final diff", async () => {
    render(<App />);
    const fixtureSelect = await screen.findByLabelText("Fixture");
    fireEvent.change(fixtureSelect, { target: { value: fixture.metadata.id } });

    await waitFor(() => expect(runMerge).toHaveBeenCalledWith(expect.objectContaining({
      algorithm: "intent",
      previous_base: fixture.previous_base,
      expected: fixture.expected,
    })));
    expect(await screen.findByText("匹配 Expected")).toBeInTheDocument();
    const diff = await screen.findByLabelText("最终结果 Diff");
    expect(screen.getByText("previous-effective.yaml → new-effective.yaml")).toBeInTheDocument();
    expect(screen.getByText("+1")).toBeInTheDocument();
    expect(screen.getByText("-1")).toBeInTheDocument();
    expect(screen.getByText("@@ -1,1 +1,1 @@")).toBeInTheDocument();
    const diffTable = screen.getByRole("table", { name: "Previous Effective 到 New Effective 的行级差异" });
    expect(diffTable).toHaveTextContent("-image: user-old");
    expect(diffTable).toHaveTextContent("+image: user-new");
    expect(diff).not.toHaveClass("focused");
    fireEvent.focus(diff);
    expect(diff).toHaveClass("focused");
    fireEvent.blur(diff);
    expect(diff).not.toHaveClass("focused");
  });

  it("marks edited fixture input as custom and reruns with another algorithm", async () => {
    render(<App />);
    fireEvent.change(await screen.findByLabelText("Fixture"), { target: { value: fixture.metadata.id } });
    await waitFor(() => expect(runMerge).toHaveBeenCalledTimes(1));

    const editors = screen.getAllByLabelText("editable-yaml");
    expect(editors[0]).toHaveAttribute("data-consume-wheel", "false");
    fireEvent.focus(editors[0]);
    expect(editors[0]).toHaveAttribute("data-consume-wheel", "true");
    fireEvent.change(editors[0], { target: { value: "image: edited\n" } });
    expect(screen.getByText("自定义输入")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("算法"), { target: { value: "direct" } });
    await waitFor(() => expect(runMerge).toHaveBeenLastCalledWith(expect.objectContaining({
      algorithm: "direct",
      previous_base: "image: edited\n",
    })));
    expect(vi.mocked(runMerge).mock.calls.at(-1)?.[0]).not.toHaveProperty("expected");
  });
});
