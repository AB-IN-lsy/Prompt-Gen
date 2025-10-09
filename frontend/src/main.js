import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:18
 * @FilePath: \electron-go-app\frontend\src\main.tsx
 * @LastEditTime: 2025-10-09 22:43:22
 */
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";
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
ReactDOM.createRoot(rootElement).render(_jsx(React.StrictMode, { children: _jsxs(QueryClientProvider, { client: queryClient, children: [_jsx(BrowserRouter, { children: _jsx(App, {}) }), _jsx(Toaster, { position: "top-right", richColors: true, closeButton: true })] }) }));
