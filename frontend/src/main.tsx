/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:18
 * @FilePath: \electron-go-app\frontend\src\main.tsx
 * @LastEditTime: 2025-10-10 00:56:11
 */
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, HashRouter } from "react-router-dom";
import { Toaster } from "sonner";

import App from "./App";
import "./index.css";
import "./i18n";

// 全局 QueryClient 实例，供数据请求缓存与失效管理使用。
const queryClient = new QueryClient();

const rootElement = document.getElementById("root");

if (!rootElement) {
    throw new Error("Root element not found");
}

console.info("[frontend] mounting React application");

const shouldUseHashRouter =
    typeof window !== "undefined" && window.location.protocol === "file:";
const RouterComponent = shouldUseHashRouter ? HashRouter : BrowserRouter;

ReactDOM.createRoot(rootElement).render(
    <React.StrictMode>
        <QueryClientProvider client={queryClient}>
            {/* BrowserRouter 负责提供路由上下文 */}
            <RouterComponent>
                <App />
            </RouterComponent>
            {/* Toaster 用于全局提示通知 */}
            <Toaster position="top-right" richColors closeButton />
        </QueryClientProvider>
    </React.StrictMode>
);
