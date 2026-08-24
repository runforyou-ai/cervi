import path from "path";
import type { Plugin } from "vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

const frontendTargets = ["web", "desktop", "mobile"] as const;

/** 读取并校验当前前端构建目标。 */
function resolveFrontendTarget() {
  const target = process.env.CERVI_FRONTEND_TARGET;
  if (
    target !== "web" &&
    target !== "desktop" &&
    target !== "mobile"
  ) {
    throw new Error(
      `CERVI_FRONTEND_TARGET 必须是 ${frontendTargets.join("、")}，当前值为 ${target ?? "未设置"}`,
    );
  }
  return target;
}

/** 把前端构建目标和模式写入产物清单。 */
function frontendBuildManifest(target: string, mode: string): Plugin {
  return {
    name: "cervi-frontend-build-manifest",
    apply: "build",
    generateBundle() {
      this.emitFile({
        type: "asset",
        fileName: "cervi-platform.json",
        source: `${JSON.stringify({ platform: target, mode })}\n`,
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  const frontendTarget = resolveFrontendTarget();
  return {
    base: "./",
    define: {
      __CERVI_FRONTEND_TARGET__: JSON.stringify(frontendTarget),
    },
    server: {
      host: "127.0.0.1",
      port: Number(process.env.WAILS_VITE_PORT) || 9245,
      strictPort: true,
    },
    plugins: [
      react(),
      tailwindcss(),
      wails("./bindings"),
      frontendBuildManifest(frontendTarget, mode),
    ],
    resolve: {
      alias: {
        "@": path.resolve(import.meta.dirname, "./src"),
      },
    },
  };
});
