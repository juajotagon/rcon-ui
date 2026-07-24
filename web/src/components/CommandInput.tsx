import { useRef, useState, type KeyboardEvent } from "react";
import type { Macro } from "../api";

/** CommandInput is the console prompt, with shell-style history.
 *
 * History is navigated with the arrow keys because that is what anyone who has
 * used a terminal will try first. A draft in progress is preserved when
 * stepping back into history and restored on the way out, so exploring history
 * never destroys what was being typed.
 */
export function CommandInput({
  disabled,
  history,
  macros,
  onSubmit,
}: {
  disabled: boolean;
  history: string[];
  macros: Macro[];
  onSubmit: (command: string) => void;
}) {
  const [value, setValue] = useState("");
  const [index, setIndex] = useState(-1); // -1 means "editing a fresh line"
  const draft = useRef("");
  const inputRef = useRef<HTMLInputElement>(null);

  const submit = () => {
    const cmd = value.trim();
    if (!cmd || disabled) return;
    onSubmit(cmd);
    setValue("");
    setIndex(-1);
    draft.current = "";
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      submit();
      return;
    }

    if (e.key === "ArrowUp") {
      if (history.length === 0) return;
      e.preventDefault();
      if (index === -1) draft.current = value;
      const next = Math.min(index + 1, history.length - 1);
      setIndex(next);
      setValue(history[next]);
      return;
    }

    if (e.key === "ArrowDown") {
      if (index === -1) return;
      e.preventDefault();
      const next = index - 1;
      setIndex(next);
      setValue(next === -1 ? draft.current : history[next]);
    }
  };

  return (
    <div className="border-t" style={{ background: "var(--color-surface-raised)" }}>
      {macros.length > 0 && (
        <div className="flex flex-wrap gap-1.5 border-b px-3 py-1.5">
          {macros.map((m) => (
            <button
              key={m.id}
              disabled={disabled}
              onClick={() => onSubmit(m.command)}
              title={m.command}
              className="rounded-full border px-2.5 py-0.5 text-xs transition-colors hover:bg-[var(--color-surface-sunken)] disabled:opacity-40"
            >
              {m.name}
            </button>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2 px-3 py-2">
        <span className="font-mono text-sm" style={{ color: "var(--color-accent)" }}>
          ›
        </span>
        <input
          ref={inputRef}
          value={value}
          disabled={disabled}
          onChange={(e) => {
            setValue(e.target.value);
            setIndex(-1);
          }}
          onKeyDown={onKeyDown}
          placeholder={disabled ? "Not connected" : "Type a command, ↑ for history"}
          spellCheck={false}
          autoComplete="off"
          className="flex-1 bg-transparent font-mono text-sm outline-none disabled:opacity-50"
        />
        <button onClick={submit} disabled={disabled || !value.trim()} className="btn btn-primary text-xs">
          Send
        </button>
      </div>
    </div>
  );
}
