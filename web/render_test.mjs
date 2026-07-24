// Loads the built bundle in a real DOM and reports what happens.
// Any exception React throws during mount surfaces here instead of vanishing
// inside a webview with no devtools.
import { JSDOM, VirtualConsole } from "jsdom";
import fs from "node:fs";

const DIST = "/home/juanjo/projects/rcon-ui/rcon-ui/internal/webui/dist";
const bundle = fs.readdirSync(`${DIST}/assets`).find((f) => f.endsWith(".js"));

const virtualConsole = new VirtualConsole();
virtualConsole.on("jsdomError", (e) => {
  console.log("!! jsdomError:", e.message);
  if (e.detail) console.log(String(e.detail).split("\n").slice(0, 12).join("\n"));
});
for (const level of ["error", "warn", "log"]) {
  virtualConsole.on(level, (...args) => console.log(`[console.${level}]`, ...args.map(String)));
}

const dom = new JSDOM(
  `<!doctype html><html><head></head><body><div id="root"></div></body></html>`,
  {
    runScripts: "dangerously",
    url: "http://127.0.0.1:8479/?access_token=demo",
    pretendToBeVisual: true,
    virtualConsole,
  },
);

const { window } = dom;

// Minimal stubs for what the app touches and jsdom lacks.
window.EventSource = class {
  constructor(url) {
    console.log("[EventSource] opened:", url);
  }
  addEventListener() {}
  close() {}
};
window.fetch = async (url, init) => {
  console.log("[fetch]", (init && init.method) || "GET", String(url));
  const body = String(url).includes("/api/servers") ? "[]" : "[]";
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => JSON.parse(body),
  };
};

window.addEventListener("error", (e) => console.log("!! window error:", e.message));
window.addEventListener("unhandledrejection", (e) => console.log("!! rejection:", String(e.reason)));

const code = fs.readFileSync(`${DIST}/assets/${bundle}`, "utf8");
console.log(`loading ${bundle} (${code.length} bytes)\n`);

try {
  const script = new window.Function(code);
  script.call(window);
} catch (e) {
  console.log("!! threw during evaluation:", e.constructor.name, e.message);
  console.log(String(e.stack).split("\n").slice(0, 8).join("\n"));
}

setTimeout(() => {
  const root = window.document.getElementById("root");
  console.log("\n=== #root after mount ===");
  const html = root.innerHTML;
  console.log(html.length === 0 ? "(EMPTY — React did not render)" : html.slice(0, 600));
  process.exit(0);
}, 1500);
