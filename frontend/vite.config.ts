/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:42:32
 * @FilePath: \electron-go-app\frontend\vite.config.ts
 * @LastEditTime: 2025-10-10 01:34:13
 */
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173
  },
  preview: {
    port: 4173
  }
});
