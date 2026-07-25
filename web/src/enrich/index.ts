import type { OptionMeta } from "./types";
import { templateById } from "../catalog";

export type { OptionMeta, EnrichmentTable } from "./types";

/** Looks up curated metadata for one discovered option.
 *
 * The catalog (see ../catalog) is the single source of truth for this data
 * -- it's authored once per game there, with an example value the Settings
 * panel never needs, and this just projects the `OptionMeta` fields back out
 * of it. That means the live Settings panel and the read-only Templates
 * catalog can never drift apart: there is only one table to edit.
 *
 * An unrecognised fingerprint (or "unknown"), or a key the catalog hasn't
 * curated, simply has no match, so every key for it falls through to
 * "Other" -- exactly the behaviour a brand-new game gets on day one, before
 * anyone has curated it.
 */
export function metaFor(fingerprint: string, key: string): OptionMeta | undefined {
  const option = templateById(fingerprint)?.options.find((o) => o.key === key);
  if (!option) return undefined;
  const { label, description, section, min, max, requiresRestart, options } = option;
  return { label, description, section, min, max, requiresRestart, options };
}
