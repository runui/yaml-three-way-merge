import Editor from "@monaco-editor/react";
import { useFocusedEditorScroll } from "./useFocusedEditorScroll";

interface Props {
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  ariaLabel: string;
}

export function YamlEditor({ value, onChange, readOnly = false, ariaLabel }: Props) {
  const { focused, focusHandlers } = useFocusedEditorScroll();

  return (
    <div className="editor-shell" aria-label={ariaLabel} {...focusHandlers}>
      <Editor
        height="100%"
        language="yaml"
        theme="vs"
        value={value}
        onChange={(next) => onChange?.(next ?? "")}
        options={{
          readOnly,
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily: "SFMono-Regular, Consolas, Liberation Mono, monospace",
          lineNumbersMinChars: 3,
          scrollbar: { handleMouseWheel: focused, alwaysConsumeMouseWheel: focused },
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: "on",
          folding: true,
          renderWhitespace: "selection",
          padding: { top: 10, bottom: 10 },
        }}
      />
    </div>
  );
}
