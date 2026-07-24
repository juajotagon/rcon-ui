import type { EnrichmentTable, OptionMeta } from "./types";
import { zomboidOptions } from "./zomboid";
import { minecraftOptions } from "./minecraft";

export type { OptionMeta, EnrichmentTable } from "./types";

/** One enrichment table per fingerprint the daemon can report. An
 * unrecognised fingerprint (or "unknown") simply has no table, so every key
 * for it falls through to "Other" -- exactly the behaviour a brand-new game
 * gets on day one, before anyone has curated it.
 */
const TABLES: Record<string, EnrichmentTable> = {
  zomboid: zomboidOptions,
  minecraft: minecraftOptions,
};

export function metaFor(fingerprint: string, key: string): OptionMeta | undefined {
  return TABLES[fingerprint]?.[key];
}
