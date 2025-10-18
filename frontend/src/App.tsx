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
import MyPromptsPage from "./pages/MyPrompts";
import PromptWorkbenchPage from "./pages/PromptWorkbench";
import SettingsPage from "./pages/Settings";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import VerifyEmailPage from "./pages/VerifyEmail";
import EmailVerificationCallbackPage from "./pages/EmailVerificationCallback";
import HelpPage from "./pages/Help";
import LogsPage from "./pages/Logs";
import IpGuardPage from "./pages/IpGuard";
import ChangelogAdminPage from "./pages/ChangelogAdmin";
import PromptDetailPage from "./pages/PromptDetail";
import { useAuth, useIsAuthenticated } from "./hooks/useAuth";
import { isLocalMode } from "./lib/runtimeMode";

// 占位页面组件：在对应功能尚未实现时保持路由完整。
function Placeholder({ titleKey }: { titleKey: string }) {
    const { t } = useTranslation();

    return (
        <div className="flex h-full flex-col items-center justify-center gap-2 text-slate-500">
            <h2 className="text-xl font-semibold text-slate-600">{t(titleKey)}</h2>
            <p className="text-sm text-slate-400">{t("errors.generic")}</p>
        </div>
    );
}

// 应用根路由容器，统一挂载外壳布局与子页面。
export default function App() {
    const initialize = useAuth((state) => state.initialize);
    const isInitialized = useAuth((state) => state.isInitialized);
    const initializing = useAuth((state) => state.initializing);
    const isAuthenticated = useIsAuthenticated();
    const { t } = useTranslation();
    const location = useLocation();
    const localMode = isLocalMode();

    // 首次挂载时触发认证初始化：校验 Token 并尝试拉取用户资料。
    useEffect(() => {
        void initialize();
    }, [initialize]);

    // 初始化期间保持加载态，避免在资料未拉取完成前误触发路由重定向。
    if (!isInitialized || initializing) {
        return (
            <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-[#F8F9FA] via-[#EEF2FF] to-[#E9EDFF]">
                <span className="text-sm text-slate-500">{t("common.loading")}</span>
            </div>
        );
    }

    if (location.pathname.startsWith("/email/verified")) {
        return <EmailVerificationCallbackPage />;
    }

    if (!isAuthenticated) {
        // 未登录用户允许访问登录 / 注册 / 邮箱验证页面，避免验证链接被重定向。
        return (
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                {localMode ? (
                    <>
                        <Route path="/register" element={<Navigate to="/login" replace />} />
                        <Route path="/verify-email" element={<Navigate to="/login" replace />} />
                    </>
                ) : (
                    <>
                        <Route path="/register" element={<RegisterPage />} />
                        <Route path="/verify-email" element={<VerifyEmailPage />} />
                    </>
                )}
                <Route path="/email/verified" element={<EmailVerificationCallbackPage />} />
                <Route path="*" element={<Navigate to="/login" replace />} />
            </Routes>
        );
    }

    return (
        <AppShell>
            <Routes>
                <Route path="/" element={<DashboardPage />} />
                <Route path="/prompts" element={<MyPromptsPage />} />
                <Route path="/prompt-workbench" element={<PromptWorkbenchPage />} />
                <Route path="/prompts/:id" element={<PromptDetailPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/logs" element={<LogsPage />} />
                <Route path="/ip-guard" element={<IpGuardPage />} />
                <Route path="/help" element={<HelpPage />} />
                <Route path="/admin/changelog" element={<ChangelogAdminPage />} />
                <Route path="*" element={<Navigate to="/prompt-workbench" replace />} />
            </Routes>
        </AppShell>
    );
}
