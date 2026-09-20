import { YamlEditor } from "./YamlEditor";

interface Props {
  title: string;
  detail: string;
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
}

export function EditorPanel({ title, detail, value, onChange, readOnly }: Props) {
  const copy = async () => navigator.clipboard?.writeText(value);
  return (
    <section className="editor-panel">
      <header className="panel-header">
        <div>
          <h2>{title}</h2>
          <p>{detail}</p>
        </div>
        <button type="button" className="text-button" onClick={copy} disabled={!value}>复制</button>
      </header>
      <YamlEditor value={value} onChange={onChange} readOnly={readOnly} ariaLabel={title} />
    </section>
  );
}
