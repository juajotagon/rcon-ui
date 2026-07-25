import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import type { Macro } from "../api";

/** A prefill request from elsewhere in the app (the Commands panel's "run…").
 * `nonce` exists so the same text can be requested twice in a row and still
 * be noticed -- an effect keyed on the text alone would see no change.
 */
export interface Prefill {
  text: string;
  nonce: number;
}

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
  commandNames = [],
  prefill,
  onSubmit,
}: {
  disabled: boolean;
  history: string[];
  macros: Macro[];
  // Discovered command names, for Tab-completion -- a prefix match only, not
  // a shell-grade completer; the catalog is flat and rarely long enough to
  // need more.
  commandNames?: string[];
  prefill?: Prefill | null;
  onSubmit: (command: string) => void;
}) {
  const [value, setValue] = useState("");
  const [index, setIndex] = useState(-1); // -1 means "editing a fresh line"
  const draft = useRef("");
  const inputRef = useRef<HTMLInputElement>(null);

  // Jumping to the Console tab with a command name queued up (from "run…" in
  // the Commands panel) is driven by a nonce so re-requesting the same text
  // still lands.
  useEffect(() => {
    if (!prefill) return;
    setValue(prefill.text);
    setIndex(-1);
    inputRef.current?.focus();
  }, [prefill]);

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
      return;
    }

    if (e.key === "Tab") {
      // Only completes the first (command-name) word -- arguments are the
      // server's business, not something we can guess from a name list.
      const [head, ...rest] = value.split(" ");
      if (!head) return;
      const match = commandNames.find((c) => c.startsWith(head) && c !== head);
      if (!match) return;
      e.preventDefault();
      setValue([match, ...rest].join(" "));
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
          placeholder={
            disabled
              ? "Not connected"
              : commandNames.length > 0
                ? "Type a command… (↑ history · Tab completes)"
                : "Type a command, ↑ for history"
          }
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
