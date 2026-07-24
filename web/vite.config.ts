import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],

  build: {
    // Emitted straight into the Go package that embeds it.
    //
    // go:embed patterns cannot traverse "..", so the assets have to live under
    // the embedding package's own directory. Writing them here rather than
    // putting a Go file inside web/ also keeps node_modules out of Go's
    // package scan, where a dependency shipping a stray .go file would break
    // `go build ./...`.
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },

  server: {
    // During development the UI runs on Vite's own port and the daemon on
    // 8477, so same-origin requests are proxied across. In production both are
    // served by the Go binary from one origin and no proxy exists.
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8477",
        changeOrigin: true,
      },
    },
  },
});
