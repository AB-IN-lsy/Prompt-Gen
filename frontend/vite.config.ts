/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:42:32
 * @FilePath: \electron-go-app\frontend\vite.config.ts
 * @LastEditTime: 2025-10-10 01:34:13
 */
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig(({ mode }) => {
  const envDir = path.resolve(__dirname, "..");
  const env = loadEnv(mode, envDir, "");

  const viteEnvForProcess = Object.fromEntries(
    Object.entries(env).filter(([key]) => key.startsWith("VITE_"))
  );

  return {
    envDir,
    define: {
      "process.env": viteEnvForProcess
    },
    plugins: [react()],
    base: "./",
    server: {
      port: 5173
    },
    preview: {
      port: 4173
    }
  };
});
