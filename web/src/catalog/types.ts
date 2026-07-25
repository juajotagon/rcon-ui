import type { OptionMeta } from "../enrich/types";

/** A curated option: everything `OptionMeta` already carries (label,
 * description, section, min/max, requiresRestart, choice list), plus the
 * two fields the catalog needs that live discovery never has to supply --
 * the key itself, and a plausible example value to display since there is
 * no server to read a real one from.
 */
export interface CatalogOption extends OptionMeta {
  key: string;
  example: string;
}

export interface CatalogCommand {
  name: string;
  description: string;
  usage?: string;
}

/** One game's RCON surface, as documentation rather than as a live report --
 * the shape deliberately mirrors `DiscoveryReport` (capabilities, options,
 * commands) so the same rendering ideas from Settings/Commands panels apply,
 * but nothing here is fetched: it is baked into the app and reviewed by hand.
 */
export interface GameTemplate {
  id: string;
  name: string;
  defaultPort?: number;
  // Honest limitations of this game's RCON surface -- what it can't do, or
  // what to double check, so the catalog never implies more than the
  // protocol actually offers.
  caveats: string[];
  capabilities: { options: boolean; optionWrite: boolean; commands: boolean };
  writeTemplate?: string;
  options: CatalogOption[];
  commands: CatalogCommand[];
}
