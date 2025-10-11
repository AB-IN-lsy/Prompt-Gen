/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:45
 * @FilePath: \electron-go-app\frontend\src\components\layout\AppShell.tsx
 * @LastEditTime: 2025-10-11 23:07:56
 */
import { NavLink, useNavigate } from "react-router-dom";
import { ReactNode, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "../../lib/utils";
import {
    Search,
    Settings,
    HelpCircle,
    LayoutDashboard,
    Sparkles,
    LogOut,
    ListChecks,
    FileClock
} from "lucide-react";
import { Button } from "../ui/button";
import { useAuth } from "../../hooks/useAuth";

// 侧边导航配置：labelKey 与翻译 key 对应，icon 控制导航图标。
const navItems = [
    { labelKey: "nav.dashboard", icon: LayoutDashboard, to: "/" },
    { labelKey: "nav.myPrompts", icon: ListChecks, to: "/prompts" },
    { labelKey: "nav.workbench", icon: Sparkles, to: "/prompt-workbench" },
    { labelKey: "nav.settings", icon: Settings, to: "/settings" },
    { labelKey: "nav.logs", icon: FileClock, to: "/logs" },
    { labelKey: "nav.help", icon: HelpCircle, to: "/help" }
];

interface AppShellProps {
    children: ReactNode;
    rightSlot?: ReactNode;
}

export function AppShell({ children, rightSlot }: AppShellProps) {
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

    return (
        <div className="flex h-screen w-screen bg-[var(--bg)] text-[var(--fg)] transition-colors">
            <aside className="hidden w-64 shrink-0 border-r border-white/40 bg-white/70 px-4 py-6 backdrop-blur-2xl transition-colors dark:border-slate-800/70 dark:bg-slate-900/60 md:flex md:flex-col">
                {/* 顶部品牌区：展示应用名称与图标 */}
                <div className="mb-8 flex items-center gap-3">
                    <Sparkles className="h-6 w-6 text-primary" />
                    <span className="text-lg font-semibold text-primary">{t("app.name")}</span>
                </div>
                {/* 导航列表：动态渲染侧边导航按钮 */}
                <nav className="flex flex-1 flex-col gap-2">
                    {navItems.map(({ labelKey, icon: Icon, to }) => (
                        <NavLink
                            key={to}
                            to={to}
                            className={({ isActive }) =>
                                cn(
                                    "flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition",
                                    isActive
                                        ? "bg-primary text-white shadow-glow"
                                        : "text-slate-600 hover:bg-white/60 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"
                                )
                            }
                        >
                            <Icon className="h-4 w-4" />
                            {t(labelKey)}
                        </NavLink>
                    ))}
                </nav>
                {/* 预留退出登录操作 */}
                <Button
                    variant="ghost"
                    className="mt-auto flex items-center justify-start gap-2 text-sm text-slate-500 dark:text-slate-300"
                    onClick={handleLogout}
                >
                    <LogOut className="h-4 w-4" /> {t("appShell.logout")}
                </Button>
            </aside>
            <main className="flex flex-1 flex-col overflow-hidden">
                {/* 顶部工具栏：包含搜索框、全局操作按钮和头像占位 */}
                <header className="glass sticky top-0 z-10 flex h-16 items-center justify-between border-b border-white/60 bg-white/70 px-6 backdrop-blur-lg transition-colors dark:border-slate-800/70 dark:bg-slate-900/60">
                    <div className="flex items-center gap-3 rounded-full border border-white/60 bg-white/80 px-4 py-2 transition-colors dark:border-slate-800 dark:bg-slate-900/60">
                        <Search className="h-4 w-4 text-slate-500 dark:text-slate-400" />
                        <input
                            placeholder={t("appShell.searchPlaceholder")}
                            className="w-64 bg-transparent text-sm text-slate-700 outline-none dark:text-slate-200"
                            type="search"
                        />
                    </div>
                    <div className="flex items-center gap-3">
                        {rightSlot}
                        <Button size="sm" variant="secondary" className="dark:shadow-none">
                            {t("appShell.syncNow")}
                        </Button>
                        <div className="flex h-9 items-center gap-3 rounded-full border border-white/60 bg-white/80 px-3 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 dark:shadow-none">
                            {profile?.user.avatar_url ? (
                                <img
                                    src={profile.user.avatar_url}
                                    alt={profile.user.username}
                                    className="h-7 w-7 rounded-full object-cover"
                                />
                            ) : (
                                <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                                    {avatarInitial}
                                </div>
                            )}
                            <span className="text-sm text-slate-600 dark:text-slate-300">{profile?.user.username}</span>
                        </div>
                    </div>
                </header>
                {/* 主内容区域，通过 children 注入各业务页面 */}
                <div className="flex-1 overflow-y-auto bg-[var(--bg)] px-6 py-6 transition-colors">
                    <div className="flex min-h-full flex-col gap-8">
                        {children}
                        <footer className="border-t border-white/60 pt-4 text-xs text-slate-400 transition-colors dark:border-slate-800/70 dark:text-slate-500">
                            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                                <span>Powered by AB-IN · 鲁ICP备2021035431号-1</span>
                                <a
                                    href="https://ab-in.blog.csdn.net/"
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-slate-500 underline-offset-4 hover:text-primary hover:underline dark:text-slate-400"
                                >
                                    AB-IN CSDN 博客 · ab-in.blog.csdn.net
                                </a>
                            </div>
                        </footer>
                    </div>
                </div>
            </main>
        </div>
    );
}
