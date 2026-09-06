/** 单独打包访客正文组件，产物随 Go 服务端发布。 */
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"

export default defineConfig({
  plugins: [react()],
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  build: {
    outDir: "../internal/publicweb/dist",
    emptyOutDir: true,
    lib: {
      entry: "src/publicweb/markdown.tsx",
      name: "CerviMarkdown",
      formats: ["iife"],
      fileName: () => "markdown.js",
      cssFileName: "markdown",
    },
  },
})
