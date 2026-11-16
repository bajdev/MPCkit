import { writeFileSync } from "node:fs";
import { defineConfig, type Plugin } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";

// main.go embeds frontend/dist, so the folder must exist in a fresh clone for
// `go build` and `go test ./...` to work. Git keeps it with dist/.gitkeep;
// Vite empties dist on every build, so put the file back afterwards.
const keepDist: Plugin = {
  name: "mpckit-keep-dist",
  apply: "build",
  closeBundle() {
    writeFileSync(new URL("./dist/.gitkeep", import.meta.url), "");
  },
};

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [svelte(), wails("./bindings"), keepDist],
});
