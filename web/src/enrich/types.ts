/** Per-key enrichment for a discovered option.
 *
 * This is decoration, never a gate: the discovery report is the only source
 * of truth for which options exist and what type they are. A key with no
 * entry here still renders as an editable field (see SettingsPanel's "Other"
 * section) -- curation improves the experience but must never be required
 * for it to exist, or a new/unfamiliar game would show nothing at all.
 */
export interface OptionMeta {
  label?: string;
  description?: string;
  // Groups the row under a heading in Settings. Options with no metadata (and
  // thus no section) fall into a synthesized "Other" section instead.
  section?: string;
  min?: number;
  max?: number;
  // Effective only after the process restarts, not on the next write -- shown
  // as a badge so a live edit doesn't read as if it took effect immediately.
  requiresRestart?: boolean;
  // Fixed choice set for an otherwise free-form string option. Renders a
  // <select> instead of a text field when present.
  options?: string[];
}

export type EnrichmentTable = Record<string, OptionMeta>;
