/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:45
 * @FilePath: \electron-go-app\frontend\src\components\layout\AppShell.tsx
 * @LastEditTime: 2025-10-12 02:29:45
 */
import { NavLink, useLocation, useNavigate } from "react-router-dom";
import {
    ReactNode,
    useCallback,
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
    type ChangeEvent,
    type KeyboardEvent
} from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { cn, resolveAssetUrl } from "../../lib/utils";
import { isLocalMode } from "../../lib/runtimeMode";
import {
    Settings,
    HelpCircle,
    LayoutDashboard,
    Sparkles,
    LogOut,
    ListChecks,
    FileClock,
    Rocket,
    ShieldAlert,
    ScrollText,
    Library,
    ClipboardCheck,
    Menu,
    X,
    PanelLeftClose,
    PanelRightOpen
} from "lucide-react";
import { Button } from "../ui/button";
import { useAuth } from "../../hooks/useAuth";
import { TitleBar } from "./TitleBar";
import { DesktopContextMenu } from "../system/DesktopContextMenu";
import { AuroraBackdrop } from "../visuals/AuroraBackdrop";
import { SpotlightSearch } from "../ui/spotlight-search";

// 侧边导航基础配置：labelKey 与翻译 key 对应，icon 控制导航图标。
const baseNavItems = [
    { labelKey: "nav.dashboard", icon: LayoutDashboard, to: "/" },
    { labelKey: "nav.myPrompts", icon: ListChecks, to: "/prompts" },
    { labelKey: "nav.publicPrompts", icon: Library, to: "/public-prompts" },
    { labelKey: "nav.workbench", icon: Sparkles, to: "/prompt-workbench" },
    { labelKey: "nav.logs", icon: FileClock, to: "/logs" },
    { labelKey: "nav.help", icon: HelpCircle, to: "/help" },
    { labelKey: "nav.settings", icon: Settings, to: "/settings" }
];

// 仅管理员显示的额外导航项。
const adminNavItems = [
    { labelKey: "nav.ipGuard", icon: ShieldAlert, to: "/ip-guard" },
    { labelKey: "nav.publicPromptReview", icon: ClipboardCheck, to: "/admin/public-prompts" },
    { labelKey: "nav.changelogAdmin", icon: ScrollText, to: "/admin/changelog" }
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
    const location = useLocation();
    const scrollContainerRef = useRef<HTMLDivElement>(null);
    const localMode = isLocalMode();
    const primaryNavItems = baseNavItems;
    const adminNavList = useMemo(() => (localMode ? [] : adminNavItems), [localMode]);
    const [showScrollTop, setShowScrollTop] = useState(false);
    const [fading, setFading] = useState(false);
    const [globalSearchValue, setGlobalSearchValue] = useState("");
    const [mobileNavOpen, setMobileNavOpen] = useState(false);
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

    const handleLogout = useCallback(async () => {
        await logout();
        navigate("/login", { replace: true });
    }, [logout, navigate]);

    const toggleMobileNav = useCallback(() => {
        setMobileNavOpen((prev) => !prev);
    }, []);

    const closeMobileNav = useCallback(() => {
        setMobileNavOpen(false);
    }, []);

    const toggleSidebarCollapsed = useCallback(() => {
        setSidebarCollapsed((prev) => !prev);
    }, []);

    const avatarInitial = useMemo(() => {
        if (!profile?.user.username) {
            return "";
        }
        return profile.user.username.charAt(0).toUpperCase();
    }, [profile?.user.username]);

    const avatarSrc = useMemo(
        () => resolveAssetUrl(profile?.user.avatar_url ?? null),
        [profile?.user.avatar_url]
    );

    useEffect(() => {
        const node = scrollContainerRef.current;
        if (!node) {
            return;
        }
        const handleScroll = () => {
            setShowScrollTop(node.scrollTop > 240);
        };
        handleScroll();
        node.addEventListener("scroll", handleScroll);
        return () => {
            node.removeEventListener("scroll", handleScroll);
        };
    }, []);

    useLayoutEffect(() => {
        const node = scrollContainerRef.current;
        if (!node) {
            return;
        }
        node.scrollTo({ top: 0, behavior: "auto" });
        setFading(true);
        let frame = 0;
        let frame2 = 0;
        frame = window.requestAnimationFrame(() => {
            frame2 = window.requestAnimationFrame(() => setFading(false));
        });
        return () => {
            window.cancelAnimationFrame(frame);
            window.cancelAnimationFrame(frame2);
        };
    }, [location.pathname]);

    useEffect(() => {
        if (!mobileNavOpen) {
            return;
        }
        const handleKeyDown = (event: globalThis.KeyboardEvent) => {
            if (event.key === "Escape") {
                setMobileNavOpen(false);
            }
        };
        window.addEventListener("keydown", handleKeyDown);
        return () => {
            window.removeEventListener("keydown", handleKeyDown);
        };
    }, [mobileNavOpen]);

    useEffect(() => {
        if (mobileNavOpen) {
            const previous = document.body.style.overflow;
            document.body.style.overflow = "hidden";
            return () => {
                document.body.style.overflow = previous;
            };
        }
        return;
    }, [mobileNavOpen]);

    useEffect(() => {
        setMobileNavOpen(false);
    }, [location.pathname]);

    const scrollToTop = useCallback(() => {
        const node = scrollContainerRef.current;
        if (!node) {
            return;
        }
        node.scrollTo({ top: 0, behavior: "smooth" });
    }, []);

    useEffect(() => {
        const params = new URLSearchParams(location.search);
        const nextValue = params.get("q") ?? "";
        setGlobalSearchValue((prev) => (prev === nextValue ? prev : nextValue));
    }, [location.pathname, location.search]);

    const handleGlobalSearchChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        setGlobalSearchValue(event.target.value);
    }, []);

    const handleGlobalSearchSubmit = useCallback(() => {
        const trimmed = globalSearchValue.trim();
        const targetSearch = trimmed ? `?q=${encodeURIComponent(trimmed)}` : "";
        navigate({
            pathname: "/public-prompts",
            search: targetSearch
        });
    }, [globalSearchValue, navigate]);

    const handleGlobalSearchKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
        if (event.key === "Enter") {
            event.preventDefault();
            handleGlobalSearchSubmit();
        }
    }, [handleGlobalSearchSubmit]);

    const animationsEnabled = profile?.settings?.enable_animations ?? true;

    const shellBackgroundClass = animationsEnabled
        ? "bg-[var(--bg)]/60 backdrop-blur-xl"
        : "bg-[var(--bg)]";

    return (
        <div className="relative flex h-screen w-screen overflow-hidden text-[var(--fg)] transition-colors">
            {animationsEnabled ? <AuroraBackdrop /> : null}
            <div className={`relative z-10 flex h-full w-full flex-col ${shellBackgroundClass}`}>
            <TitleBar />
            <DesktopContextMenu />
            {mobileNavOpen ? (
                <div
                    className="fixed inset-0 z-40 flex md:hidden"
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="app-shell-mobile-nav-title"
                >
                    <div
                        className="absolute inset-0 bg-slate-950/60 backdrop-blur-sm"
                        onClick={closeMobileNav}
                    />
                    <div
                        id="app-shell-mobile-nav"
                        className="relative flex h-full w-72 max-w-[80%] flex-col border-r border-white/40 bg-white/95 px-4 py-6 shadow-2xl transition dark:border-slate-800/70 dark:bg-slate-900/95"
                    >
                        <div className="mb-6 flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <Sparkles className="h-6 w-6 text-primary" />
                                <span
                                    id="app-shell-mobile-nav-title"
                                    className="text-lg font-semibold text-primary"
                                >
                                    {t("app.name")}
                                </span>
                            </div>
                            <button
                                type="button"
                                onClick={closeMobileNav}
                                className="flex h-9 w-9 items-center justify-center rounded-xl border border-white/60 bg-white/80 text-slate-600 shadow-sm transition hover:border-primary/50 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-primary/40"
                                aria-label={t("appShell.mobileMenu.close")}
                            >
                                <X className="h-5 w-5" />
                            </button>
                        </div>
                        <nav className="flex flex-1 flex-col gap-2 overflow-y-auto">
                            {primaryNavItems.map(({ labelKey, icon: Icon, to }) => (
                                <NavLink
                                    key={to}
                                    to={to}
                                    onClick={closeMobileNav}
                                    className={({ isActive }) =>
                                        cn(
                                            "flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition",
                                            isActive
                                                ? "bg-primary text-white shadow-glow"
                                                : "text-slate-600 hover:bg-white/80 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"
                                        )
                                    }
                                >
                                    <Icon className="h-4 w-4" />
                                    {t(labelKey)}
                                </NavLink>
                            ))}
                            {profile?.user.is_admin && adminNavList.length > 0 ? (
                                <div className="mt-4 flex flex-col gap-2 border-t border-white/60 pt-4 dark:border-slate-800/60">
                                    <span className="px-3 text-xs font-medium uppercase tracking-[0.3em] text-slate-400">
                                        {t("nav.adminSection")}
                                    </span>
                                    {adminNavList.map(({ labelKey, icon: Icon, to }) => (
                                        <NavLink
                                            key={to}
                                            to={to}
                                            onClick={closeMobileNav}
                                            className={({ isActive }) =>
                                                cn(
                                                    "flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition",
                                                    isActive
                                                        ? "bg-primary text-white shadow-glow"
                                                        : "text-slate-600 hover:bg-white/80 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"
                                                )
                                            }
                                        >
                                            <Icon className="h-4 w-4" />
                                            {t(labelKey)}
                                        </NavLink>
                                    ))}
                                </div>
                            ) : null}
                        </nav>
                        {!localMode ? (
                            <Button
                                variant="ghost"
                                className="mt-6 flex items-center justify-start gap-2 text-sm text-slate-500 dark:text-slate-300"
                                onClick={() => {
                                    closeMobileNav();
                                    void handleLogout();
                                }}
                            >
                                <LogOut className="h-4 w-4" /> {t("appShell.logout")}
                            </Button>
                        ) : null}
                    </div>
                </div>
            ) : null}
            <div className="flex flex-1 overflow-hidden">
                <motion.aside
                    initial={false}
                    animate={{ width: sidebarCollapsed ? 80 : 224 }}
                    transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
                    className="hidden h-full shrink-0 border-r border-white/40 bg-white/70 px-3 py-6 backdrop-blur-2xl transition-colors dark:border-slate-800/70 dark:bg-slate-900/60 md:flex md:flex-col"
                >
                    {/* 顶部品牌区：展示应用名称与图标 */}
                    <div className="mb-8 flex items-center justify-between gap-2">
                        <div
                            className={cn(
                                "flex items-center",
                                sidebarCollapsed ? "justify-center" : "gap-3",
                            )}
                            title={sidebarCollapsed ? t("app.name") : undefined}
                        >
                            <Sparkles className="h-6 w-6 text-primary" />
                            {!sidebarCollapsed ? (
                                <span className="text-lg font-semibold text-primary">{t("app.name")}</span>
                            ) : null}
                        </div>
                        <button
                            type="button"
                            onClick={toggleSidebarCollapsed}
                            className={cn(
                                "flex h-9 w-9 items-center justify-center rounded-xl border border-white/60 bg-white/80 text-slate-500 shadow-sm transition hover:border-primary/40 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-primary/40",
                            )}
                            aria-label={
                                sidebarCollapsed
                                    ? t("appShell.sidebar.expand")
                                    : t("appShell.sidebar.collapse")
                            }
                            title={
                                sidebarCollapsed
                                    ? t("appShell.sidebar.expand")
                                    : t("appShell.sidebar.collapse")
                            }
                        >
                            {sidebarCollapsed ? (
                                <PanelRightOpen className="h-4 w-4" />
                            ) : (
                                <PanelLeftClose className="h-4 w-4" />
                            )}
                        </button>
                    </div>
                    {/* 导航列表：动态渲染侧边导航按钮 */}
                    <nav className="flex flex-1 flex-col gap-2">
                        {primaryNavItems.map(({ labelKey, icon: Icon, to }) => (
                            <NavLink
                                key={to}
                                to={to}
                                className={({ isActive }) =>
                                    cn(
                                        "flex items-center rounded-xl text-sm font-medium transition",
                                        sidebarCollapsed ? "justify-center px-2 py-2" : "gap-3 px-3 py-2",
                                        isActive
                                            ? "bg-primary text-white shadow-glow"
                                            : "text-slate-600 hover:bg-white/60 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"
                                    )
                                }
                                title={t(labelKey)}
                            >
                                <Icon className="h-4 w-4" />
                                {!sidebarCollapsed ? <span>{t(labelKey)}</span> : null}
                            </NavLink>
                        ))}
                        {profile?.user.is_admin && adminNavList.length > 0 ? (
                            <div className="mt-6 flex flex-col gap-2 border-t border-white/60 pt-4 dark:border-slate-800/60">
                                {!sidebarCollapsed ? (
                                    <span className="px-3 text-xs font-medium uppercase tracking-[0.3em] text-slate-400">
                                        {t("nav.adminSection")}
                                    </span>
                                ) : null}
                                {adminNavList.map(({ labelKey, icon: Icon, to }) => (
                                    <NavLink
                                        key={to}
                                        to={to}
                                        className={({ isActive }) =>
                                            cn(
                                                "flex items-center rounded-xl text-sm font-medium transition",
                                                sidebarCollapsed ? "justify-center px-2 py-2" : "gap-3 px-3 py-2",
                                                isActive
                                                    ? "bg-primary text-white shadow-glow"
                                                    : "text-slate-600 hover:bg-white/60 hover:text-primary hover:shadow-sm dark:text-slate-300 dark:hover:bg-slate-800/80"
                                            )
                                        }
                                        title={t(labelKey)}
                                    >
                                        <Icon className="h-4 w-4" />
                                        {!sidebarCollapsed ? <span>{t(labelKey)}</span> : null}
                                    </NavLink>
                                ))}
                            </div>
                        ) : null}
                    </nav>
                    {/* 预留退出登录操作 */}
                    {!localMode ? (
                        <Button
                            variant="ghost"
                            className={cn(
                                "mt-auto flex items-center text-sm text-slate-500 dark:text-slate-300",
                                sidebarCollapsed ? "justify-center px-2" : "justify-start gap-2 px-3",
                            )}
                            onClick={handleLogout}
                            title={t("appShell.logout")}
                        >
                            <LogOut className="h-4 w-4" />
                            {!sidebarCollapsed ? <span>{t("appShell.logout")}</span> : null}
                        </Button>
                    ) : null}
                </motion.aside>
                <main className="flex flex-1 flex-col overflow-hidden">
                    {/* 顶部工具栏：包含全局操作按钮和头像占位 */}
                    <header className="glass sticky top-0 z-10 flex h-16 items-center justify-between gap-4 border-b border-white/60 bg-white/70 px-6 backdrop-blur-lg transition-colors dark:border-slate-800/70 dark:bg-slate-900/60">
                        <div className="flex flex-1 items-center gap-3">
                            <button
                                type="button"
                                onClick={toggleMobileNav}
                                className="flex h-10 w-10 items-center justify-center rounded-xl border border-white/60 bg-white/80 text-slate-600 shadow-sm transition hover:border-primary/50 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-primary/40 md:hidden"
                                aria-label={
                                    mobileNavOpen
                                        ? t("appShell.mobileMenu.close")
                                        : t("appShell.mobileMenu.open")
                                }
                                aria-expanded={mobileNavOpen}
                                aria-controls="app-shell-mobile-nav"
                            >
                                {mobileNavOpen ? (
                                    <X className="h-5 w-5" />
                                ) : (
                                    <Menu className="h-5 w-5" />
                                )}
                            </button>
                            <SpotlightSearch
                                placeholder={t("appShell.globalSearch.placeholder")}
                                value={globalSearchValue}
                                onChange={handleGlobalSearchChange}
                                onKeyDown={handleGlobalSearchKeyDown}
                                aria-label={t("appShell.globalSearch.ariaLabel")}
                                className="min-w-0 flex-1"
                            />
                            {rightSlot ? (
                                <div className="flex items-center gap-3 text-sm text-slate-500 dark:text-slate-400">
                                    {rightSlot}
                                </div>
                            ) : null}
                        </div>
                        <div className="flex items-center gap-3">
                            <button
                                type="button"
                                onClick={() => navigate("/settings")}
                                className="group flex h-9 items-center gap-3 rounded-full border border-white/60 bg-white/80 px-3 text-left shadow-sm transition-colors hover:border-primary/50 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-primary/40"
                                aria-label={t("nav.settings")}
                            >
                                {avatarSrc ? (
                                    <img
                                        src={avatarSrc}
                                        alt={profile?.user.username ?? ""}
                                        className="h-7 w-7 rounded-full object-cover"
                                    />
                                ) : (
                                    <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
                                        {avatarInitial}
                                    </div>
                                )}
                                <span className="text-sm text-slate-600 transition-colors group-hover:text-primary dark:text-slate-300 dark:group-hover:text-primary">
                                    {profile?.user.username}
                                </span>
                            </button>
                        </div>
                    </header>
                    {/* 主内容区域，通过 children 注入各业务页面 */}
                    <div ref={scrollContainerRef} className="flex-1 overflow-y-auto bg-[var(--bg)] px-6 py-6 transition-colors">
                        <div className="flex min-h-full flex-col gap-8">
                            <div
                                className={cn(
                                    "flex flex-1 flex-col transition-opacity duration-200 ease-out",
                                    fading ? "opacity-0" : "opacity-100"
                                )}
                            >
                                {children}
                            </div>
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
                    {showScrollTop ? (
                        <Button
                            type="button"
                            size="sm"
                            variant="secondary"
                            className="fixed bottom-6 right-6 z-20 shadow-lg transition-transform hover:-translate-y-0.5"
                            onClick={scrollToTop}
                        >
                            <Rocket className="mr-2 h-4 w-4" />
                            {t("appShell.backToTop")}
                        </Button>
                    ) : null}
                </main>
            </div>
        </div>
        </div>
    );
}
