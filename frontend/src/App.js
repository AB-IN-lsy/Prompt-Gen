import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:47:32
 * @FilePath: \electron-go-app\frontend\src\App.tsx
 * @LastEditTime: 2025-10-09 22:51:57
 */
import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useEffect } from "react";
import { AppShell } from "./components/layout/AppShell";
import DashboardPage from "./pages/Dashboard";
import PromptWorkbenchPage from "./pages/PromptWorkbench";
import SettingsPage from "./pages/Settings";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import VerifyEmailPage from "./pages/VerifyEmail";
import EmailVerificationCallbackPage from "./pages/EmailVerificationCallback";
import { useAuth, useIsAuthenticated } from "./hooks/useAuth";
// 占位页面组件：在对应功能尚未实现时保持路由完整。
function Placeholder({ titleKey }) {
    const { t } = useTranslation();
    return (_jsxs("div", { className: "flex h-full flex-col items-center justify-center gap-2 text-slate-500", children: [_jsx("h2", { className: "text-xl font-semibold text-slate-600", children: t(titleKey) }), _jsx("p", { className: "text-sm text-slate-400", children: t("errors.generic") })] }));
}
// 应用根路由容器，统一挂载外壳布局与子页面。
export default function App() {
    const initialize = useAuth((state) => state.initialize);
    const isInitialized = useAuth((state) => state.isInitialized);
    const initializing = useAuth((state) => state.initializing);
    const isAuthenticated = useIsAuthenticated();
    const { t } = useTranslation();
    const location = useLocation();
    // 首次挂载时触发认证初始化：校验 Token 并尝试拉取用户资料。
    useEffect(() => {
        void initialize();
    }, [initialize]);
    // 初始化期间保持加载态，避免在资料未拉取完成前误触发路由重定向。
    if (!isInitialized || initializing) {
        return (_jsx("div", { className: "flex min-h-screen items-center justify-center bg-gradient-to-br from-[#F8F9FA] via-[#EEF2FF] to-[#E9EDFF]", children: _jsx("span", { className: "text-sm text-slate-500", children: t("common.loading") }) }));
    }
    if (location.pathname.startsWith("/email/verified")) {
        return _jsx(EmailVerificationCallbackPage, {});
    }
    if (!isAuthenticated) {
        // 未登录用户允许访问登录 / 注册 / 邮箱验证页面，避免验证链接被重定向。
        return (_jsxs(Routes, { children: [_jsx(Route, { path: "/login", element: _jsx(LoginPage, {}) }), _jsx(Route, { path: "/register", element: _jsx(RegisterPage, {}) }), _jsx(Route, { path: "/verify-email", element: _jsx(VerifyEmailPage, {}) }), _jsx(Route, { path: "/email/verified", element: _jsx(EmailVerificationCallbackPage, {}) }), _jsx(Route, { path: "*", element: _jsx(Navigate, { to: "/login", replace: true }) })] }));
    }
    return (_jsx(AppShell, { children: _jsxs(Routes, { children: [_jsx(Route, { path: "/", element: _jsx(DashboardPage, {}) }), _jsx(Route, { path: "/prompts", element: _jsx(Placeholder, { titleKey: "nav.myPrompts" }) }), _jsx(Route, { path: "/prompt-workbench", element: _jsx(PromptWorkbenchPage, {}) }), _jsx(Route, { path: "/settings", element: _jsx(SettingsPage, {}) }), _jsx(Route, { path: "/help", element: _jsx(Placeholder, { titleKey: "nav.help" }) }), _jsx(Route, { path: "*", element: _jsx(Navigate, { to: "/prompt-workbench", replace: true }) })] }) }));
}
