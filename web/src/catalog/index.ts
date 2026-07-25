import type { GameTemplate } from "./types";
import { zomboidTemplate } from "./zomboid";
import { minecraftTemplate } from "./minecraft";
import { cs2Template } from "./cs2";
import { rustTemplate } from "./rust";
import { valheimTemplate } from "./valheim";
import { palworldTemplate } from "./palworld";
import { arkTemplate } from "./ark";
import { sevenDaysToDieTemplate } from "./sevendaystodie";
import { squadTemplate } from "./squad";
import { conanExilesTemplate } from "./conanexiles";
import { factorioTemplate } from "./factorio";

export type { GameTemplate, CatalogOption, CatalogCommand } from "./types";

/** The catalog's game order: the two fully-discoverable games first (they
 * double as the enrichment source for live Settings, see enrich/index.ts),
 * then everything else roughly by how much of the surface is documented.
 */
export const games: GameTemplate[] = [
  zomboidTemplate,
  minecraftTemplate,
  cs2Template,
  rustTemplate,
  valheimTemplate,
  palworldTemplate,
  arkTemplate,
  sevenDaysToDieTemplate,
  squadTemplate,
  conanExilesTemplate,
  factorioTemplate,
];

export function templateById(id: string): GameTemplate | undefined {
  return games.find((g) => g.id === id);
}
