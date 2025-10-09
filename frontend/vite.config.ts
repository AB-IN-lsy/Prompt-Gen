/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:42:32
 * @FilePath: \electron-go-app\frontend\vite.config.ts
 * @LastEditTime: 2025-10-09 22:42:36
 */
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true
      }
    }
  },
  preview: {
    port: 4173
  }
});
