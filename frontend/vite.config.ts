import path from "path";
import tailwindcss from "@tailwindcss/vite";
import { viteStaticCopy } from "vite-plugin-static-copy";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  base: "./",
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), tailwindcss(), wails("./bindings"), viteStaticCopy({
    targets: ["cmaps", "standard_fonts", "wasm"].map((directory) => ({
      src: `node_modules/pdfjs-dist/${directory}/*`,
      dest: `pdfjs/${directory}`,
      rename: { stripBase: true },
    })),
  })],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
