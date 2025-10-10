import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:45
 * @FilePath: \electron-go-app\frontend\src\components\layout\AppShell.tsx
 * @LastEditTime: 2025-10-09 22:43:50
 */
import { NavLink, useNavigate } from "react-router-dom";
import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "../../lib/utils";
import { Search, Settings, HelpCircle, LayoutDashboard, Sparkles, LogOut, ListChecks } from "lucide-react";
import { Button } from "../ui/button";
import { useAuth } from "../../hooks/useAuth";
// 侧边导航配置：labelKey 与翻译 key 对应，icon 控制导航图标。
const navItems = [
    { labelKey: "nav.dashboard", icon: LayoutDashboard, to: "/" },
    { labelKey: "nav.myPrompts", icon: ListChecks, to: "/prompts" },
    { labelKey: "nav.workbench", icon: Sparkles, to: "/prompt-workbench" },
    { labelKey: "nav.settings", icon: Settings, to: "/settings" },
    { labelKey: "nav.help", icon: HelpCircle, to: "/help" }
];
export function AppShell({ children, rightSlot }) {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const profile = useAuth((state) => state.profile);
    const logout = useAuth((state) => state.logout);
    const handleLogout = useCallback(async () => {
        await logout();
        navigate("/login", { replace: true });
    }, [logout, navigate]);
    const avatarInitial = useMemo(() => {
        if (!profile?.user.username) {
            return "";
        }
        return profile.user.username.charAt(0).toUpperCase();
    }, [profile?.user.username]);
    return (_jsxs("div", { className: "flex h-screen w-screen bg-[var(--bg)] text-[var(--fg)] transition-colors", children: [_jsxs("aside", { className: "hidden w-64 shrink-0 border-r border-white/40 bg-white/70 px-4 py-6 backdrop-blur-2xl transition-colors dark:border-slate-800/70 dark:bg-slate-900/60 md:flex md:flex-col", children: [_jsxs("div", { className: "mb-8 flex items-center gap-3", children: [_jsx(Sparkles, { className: "h-6 w-6 text-primary" }), _jsx("span", { className: "text-lg font-semibold text-primary", children: t("app.name") })] }), _jsx("nav", { className: "flex flex-1 flex-col gap-2", children: navItems.map(({ labelKey, icon: Icon, to }) => (_jsxs(NavLink, { to: to, className: ({ isActive }) => cn("flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition", isActive
                                ? "bg-primary text-white shadow-glow"
                                : "text-slate-600 hover:bg-white/60 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"), children: [_jsx(Icon, { className: "h-4 w-4" }), t(labelKey)] }, to))) }), _jsxs(Button, { variant: "ghost", className: "mt-auto flex items-center justify-start gap-2 text-sm text-slate-500 dark:text-slate-300", onClick: handleLogout, children: [_jsx(LogOut, { className: "h-4 w-4" }), " ", t("appShell.logout")] })] }), _jsxs("main", { className: "flex flex-1 flex-col overflow-hidden", children: [_jsxs("header", { className: "glass sticky top-0 z-10 flex h-16 items-center justify-between border-b border-white/60 bg-white/70 px-6 backdrop-blur-lg transition-colors dark:border-slate-800/70 dark:bg-slate-900/60", children: [_jsxs("div", { className: "flex items-center gap-3 rounded-full border border-white/60 bg-white/80 px-4 py-2 transition-colors dark:border-slate-800 dark:bg-slate-900/60", children: [_jsx(Search, { className: "h-4 w-4 text-slate-500 dark:text-slate-400" }), _jsx("input", { placeholder: t("appShell.searchPlaceholder"), className: "w-64 bg-transparent text-sm text-slate-700 outline-none dark:text-slate-200", type: "search" })] }), _jsxs("div", { className: "flex items-center gap-3", children: [rightSlot, _jsx(Button, { size: "sm", variant: "secondary", className: "dark:shadow-none", children: t("appShell.syncNow") }), _jsxs("div", { className: "flex h-9 items-center gap-3 rounded-full border border-white/60 bg-white/80 px-3 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 dark:shadow-none", children: [profile?.user.avatar_url ? (_jsx("img", { src: profile.user.avatar_url, alt: profile.user.username, className: "h-7 w-7 rounded-full object-cover" })) : (_jsx("div", { className: "flex h-7 w-7 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary", children: avatarInitial })), _jsx("span", { className: "text-sm text-slate-600 dark:text-slate-300", children: profile?.user.username })] })] })] }), _jsx("div", { className: "flex-1 overflow-y-auto bg-[var(--bg)] px-6 py-6 transition-colors", children: children })] })] }));
}
