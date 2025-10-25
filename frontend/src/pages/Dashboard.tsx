/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 00:38:36
 * @FilePath: \electron-go-app\frontend\src\pages\Dashboard.tsx
 * @LastEditTime: 2025-10-10 00:38:41
 */
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
    ArrowUpRight,
    BarChart3,
    Clock,
    CheckCircle,
    FileText,
    History,
    LoaderCircle,
    LucideIcon,
    Sparkles,
    TrendingUp,
    XCircle,
    X
} from "lucide-react";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { GlassCard } from "../components/ui/glass-card";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { useAuth } from "../hooks/useAuth";
import {
    fetchMyPrompts,
    fetchUserModels,
    PromptListItem,
    PromptListResponse
} from "../lib/api";
import { cn } from "../lib/utils";
import { MagneticButton } from "../components/ui/magnetic-button";

const SEARCH_HISTORY_STORAGE_KEY = "promptgen/dashboard/search-history";
const SEARCH_HISTORY_LIMIT = 5;
const RECENT_PROMPT_LIMIT = 5;

type MetricTrend = "up" | "down" | "neutral";

interface Metric {
    id: string;
    label: string;
    value: string;
    hint: string;
    icon: LucideIcon;
    trend: MetricTrend;
}

interface FormattedDateTime {
    date: string;
    time: string;
}


interface TypewriterTextProps {
    text: string;
    speed?: number;
    startDelay?: number;
    className?: string;
}

function TypewriterText({ text, speed = 38, startDelay = 160, className }: TypewriterTextProps) {
    const [display, setDisplay] = useState(text);

    useEffect(() => {
        if (typeof window === "undefined") {
            setDisplay(text);
            return;
        }
        if (!text) {
            setDisplay("");
            return;
        }
        let index = 0;
        let typingTimer: number | undefined;
        const kickoff = window.setTimeout(() => {
            setDisplay("");
            const tick = () => {
                index += 1;
                setDisplay(text.slice(0, index));
                if (index < text.length) {
                    typingTimer = window.setTimeout(tick, speed);
                }
            };
            tick();
        }, startDelay);
        return () => {
            window.clearTimeout(kickoff);
            if (typingTimer !== undefined) {
                window.clearTimeout(typingTimer);
            }
        };
    }, [text, speed, startDelay]);

    return <span className={className}>{display}</span>;
}

function MetricCard({ icon: Icon, label, value, hint, trend }: Metric): JSX.Element {
    const trendClass =
        trend === "up"
            ? "text-emerald-600"
            : trend === "down"
                ? "text-rose-500"
                : "text-slate-500 dark:text-slate-400";

    return (
        <GlassCard className="relative overflow-hidden">
            <div className="flex items-start justify-between gap-4">
                <div>
                    <p className="text-sm text-slate-500 dark:text-slate-400">{label}</p>
                    <p className="mt-3 text-3xl font-semibold text-slate-900 dark:text-slate-100">{value}</p>
                </div>
                <div className="rounded-2xl bg-primary/10 p-3 text-primary shadow-glow">
                    <Icon className="h-6 w-6" aria-hidden="true" />
                </div>
            </div>
            <p className={cn("mt-4 text-xs font-medium", trendClass)}>{hint}</p>
        </GlassCard>
    );
}

interface SpotlightHeroProps {
    eyebrow: string;
    title: string;
    description: string;
    ctaLabel: string;
    onCtaClick: () => void;
    metrics: Metric[];
}

function SpotlightHero({
    eyebrow,
    title,
    description,
    ctaLabel,
    onCtaClick,
    metrics
}: SpotlightHeroProps) {
    return (
        <section className="relative overflow-hidden rounded-3xl border border-white/50 bg-white/80 px-8 py-12 shadow-lg transition-colors dark:border-slate-800/60 dark:bg-slate-900/70">
            <div className="pointer-events-none absolute inset-0">
                <div className="absolute -top-24 left-1/4 h-72 w-72 rounded-full bg-gradient-to-br from-indigo-400/40 via-sky-300/30 to-purple-400/30 blur-3xl" />
                <div className="absolute -bottom-32 right-1/4 h-80 w-80 rounded-full bg-gradient-to-br from-emerald-300/35 via-cyan-300/25 to-blue-300/25 blur-[120px]" />
                <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(255,255,255,0.65),transparent_60%)] dark:bg-[radial-gradient(circle_at_top_left,rgba(148,163,255,0.12),transparent_70%)]" />
                <div className="absolute inset-0 opacity-[0.08] bg-noise" />
            </div>

            <div className="relative z-10 flex flex-col gap-8 text-slate-700 dark:text-slate-100">
                <div className="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
                    <div className="max-w-2xl space-y-3">
                        <span className="inline-flex items-center rounded-full border border-white/60 bg-white/40 px-3 py-1 text-xs font-medium uppercase tracking-[0.3em] text-slate-400 shadow-sm backdrop-blur-xl dark:border-slate-800/60 dark:bg-slate-900/50 dark:text-slate-500">
                            {eyebrow}
                        </span>
                        <h1 className="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white md:text-4xl">
                            <TypewriterText text={title} className="block" />
                        </h1>
                        <p className="text-sm text-slate-500 dark:text-slate-400">
                            {description}
                        </p>
                    </div>
                    <MagneticButton
                        variant="outline"
                        size="lg"
                        className="group flex items-center gap-2 rounded-full border border-primary/40 bg-primary/10 px-5 py-3 text-sm font-medium text-primary shadow-[0_15px_35px_-12px_rgba(59,130,246,0.45)] transition hover:bg-primary hover:text-white dark:border-primary/30 dark:bg-primary/20"
                        onClick={onCtaClick}
                        intensity={18}
                    >
                        <Sparkles className="h-4 w-4 transition group-hover:scale-110" aria-hidden="true" />
                        {ctaLabel}
                    </MagneticButton>
                </div>

                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                    {metrics.map((metric) => (
                        <MetricCard key={metric.id} {...metric} />
                    ))}
                </div>
            </div>
        </section>
    );
}

const statusBadgeClass: Record<PromptListItem["status"], string> = {
    published:
        "border-emerald-300 text-emerald-600 dark:border-emerald-500 dark:text-emerald-300",
    draft:
        "border-slate-300 text-slate-500 dark:border-slate-600 dark:text-slate-300",
    archived:
        "border-amber-300 text-amber-600 dark:border-amber-500 dark:text-amber-300"
};

function formatDateTime(value?: string | null, locale?: string): FormattedDateTime {
    if (!value) {
        return { date: "—", time: "" };
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return { date: value, time: "" };
    }
    const dateFormatter = new Intl.DateTimeFormat(locale ?? undefined, { dateStyle: "short" });
    const timeFormatter = new Intl.DateTimeFormat(locale ?? undefined, { timeStyle: "short" });
    return {
        date: dateFormatter.format(date),
        time: timeFormatter.format(date)
    };
}

export default function DashboardPage(): JSX.Element {
    const { t, i18n } = useTranslation();
    const navigate = useNavigate();
    const profile = useAuth((state) => state.profile);

    const [searchInput, setSearchInput] = useState("");
    const [searchHistory, setSearchHistory] = useState<string[]>(() => {
        if (typeof window === "undefined") {
            return [];
        }
        try {
            const raw = window.localStorage.getItem(SEARCH_HISTORY_STORAGE_KEY);
            if (!raw) {
                return [];
            }
            const parsed = JSON.parse(raw) as unknown;
            if (!Array.isArray(parsed)) {
                return [];
            }
            return parsed.filter((item): item is string => typeof item === "string");
        } catch (error) {
            console.warn("Failed to parse search history", error);
            return [];
        }
    });
    const [searchResults, setSearchResults] = useState<PromptListItem[]>([]);
    const [searchLoading, setSearchLoading] = useState(false);
    const [searchError, setSearchError] = useState<string | null>(null);
    const [activeSearch, setActiveSearch] = useState<string | null>(null);

    useEffect(() => {
        if (typeof window === "undefined") {
            return;
        }
        window.localStorage.setItem(
            SEARCH_HISTORY_STORAGE_KEY,
            JSON.stringify(searchHistory)
        );
    }, [searchHistory]);

    const persistHistory = useCallback((term: string) => {
        setSearchHistory((prev) => {
            const next = [term, ...prev.filter((item) => item !== term)];
            return next.slice(0, SEARCH_HISTORY_LIMIT);
        });
    }, []);

    const handleRemoveHistory = useCallback((term: string) => {
        setSearchHistory((prev) => prev.filter((item) => item !== term));
    }, []);

    const handleClearHistory = useCallback(() => {
        setSearchHistory([]);
    }, []);

    const performSearch = useCallback(
        async (term: string) => {
            const value = term.trim();
            setSearchError(null);
            setActiveSearch(value || null);

            if (!value) {
                setSearchResults([]);
                return;
            }

            setSearchLoading(true);
            try {
                const response: PromptListResponse = await fetchMyPrompts({
                    query: value,
                    page: 1,
                    pageSize: RECENT_PROMPT_LIMIT
                });
                setSearchResults(response.items);
                persistHistory(value);
            } catch (error) {
                setSearchResults([]);
                const message =
                    error instanceof Error ? error.message : t("errors.generic");
                setSearchError(message);
            } finally {
                setSearchLoading(false);
            }
        },
        [persistHistory, t]
    );

    const handleSearchSubmit = useCallback(
        (event?: FormEvent<HTMLFormElement>) => {
            event?.preventDefault();
            void performSearch(searchInput);
        },
        [performSearch, searchInput]
    );

    const handleSelectHistory = useCallback(
        (term: string) => {
            setSearchInput(term);
            void performSearch(term);
        },
        [performSearch]
    );

    const {
        data: allPromptsData,
        isLoading: allLoading,
        error: allError
    } = useQuery({
        queryKey: ["dashboard", "prompts", "all"],
        queryFn: () => fetchMyPrompts({ page: 1, pageSize: RECENT_PROMPT_LIMIT }),
        staleTime: 60_000
    });

    const {
        data: draftData,
        isLoading: draftLoading,
        error: draftError
    } = useQuery({
        queryKey: ["dashboard", "prompts", "drafts"],
        queryFn: () => fetchMyPrompts({ status: "draft", page: 1, pageSize: RECENT_PROMPT_LIMIT }),
        staleTime: 60_000
    });

    const {
        data: publishedData
    } = useQuery({
        queryKey: ["dashboard", "prompts", "published"],
        queryFn: () => fetchMyPrompts({ status: "published", page: 1, pageSize: 1 }),
        staleTime: 60_000
    });

    const {
        data: modelsData,
        isLoading: modelsLoading,
        error: modelsError
    } = useQuery({
        queryKey: ["dashboard", "models"],
        queryFn: () => fetchUserModels(),
        staleTime: 60_000
    });

    const totalPrompts = allPromptsData?.meta.total_items ?? 0;
    const draftCount = draftData?.meta.total_items ?? 0;
    const publishedCount = publishedData?.meta.total_items ?? 0;
    const successRate =
        totalPrompts > 0 ? Math.round((publishedCount / totalPrompts) * 100) : null;

    const metrics = useMemo<Metric[]>(
        () => [
            {
                id: "active",
                label: t("dashboard.metrics.activePrompts"),
                value: String(publishedCount),
                hint: t("dashboard.metrics.activeHint", { count: totalPrompts }),
                icon: Sparkles,
                trend: publishedCount > 0 ? "up" : "neutral"
            },
            {
                id: "drafts",
                label: t("dashboard.metrics.drafts"),
                value: String(draftCount),
                hint: t("dashboard.metrics.draftHint", { count: draftCount }),
                icon: FileText,
                trend: draftCount > 0 ? "down" : "neutral"
            },
            {
                id: "successRate",
                label: t("dashboard.metrics.successRate"),
                value: successRate !== null ? `${successRate}%` : t("dashboard.metrics.noData"),
                hint: t("dashboard.metrics.successRateHint"),
                icon: BarChart3,
                trend: successRate !== null && successRate >= 60 ? "up" : "neutral"
            }
        ],
        [draftCount, publishedCount, successRate, t, totalPrompts]
    );

    const recentPrompts = allPromptsData?.items ?? [];

    const displayName =
        profile?.user.username ?? profile?.user.email ?? t("dashboard.defaultName");

    const handleOpenPrompt = useCallback(
        (promptId: number) => {
            navigate(`/prompts/${promptId}`);
        },
        [navigate]
    );

    const renderPromptRow = (item: PromptListItem) => {
        const { date, time } = formatDateTime(item.updated_at, i18n.language);
        const statusLabel = t(`myPrompts.statusBadge.${item.status}`);

        return (
            <div
                key={item.id}
                className="group flex items-start justify-between gap-3 rounded-2xl border border-white/60 bg-white/70 px-4 py-3 shadow-sm transition duration-200 hover:-translate-y-0.5 hover:border-primary/35 hover:bg-primary/5 hover:ring-2 hover:ring-primary/20 hover:ring-offset-2 hover:ring-offset-white dark:border-slate-800/60 dark:bg-slate-900/50 dark:hover:bg-primary/10 dark:hover:ring-offset-slate-900"
            >
                <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                        <button
                            type="button"
                            onClick={() => handleOpenPrompt(item.id)}
                            className="bg-transparent text-left text-sm font-medium text-slate-900 transition-all duration-200 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:text-slate-100 dark:hover:text-primary-200 group-hover:text-primary group-hover:translate-x-0.5 dark:group-hover:text-primary-200"
                        >
                            {item.topic}
                        </button>
                        <Badge
                            variant="outline"
                            className={cn("whitespace-nowrap text-xs", statusBadgeClass[item.status])}
                        >
                            {statusLabel}
                        </Badge>
                    </div>
                    <div className="text-xs text-slate-500 dark:text-slate-400">
                        {date}
                        {time ? ` · ${time}` : ""}
                    </div>
                </div>
                <Button
                    variant="ghost"
                    size="sm"
                    className="gap-2 text-xs transition-colors duration-200 group-hover:text-primary"
                    onClick={() => handleOpenPrompt(item.id)}
                >
                    <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
                    {t("dashboard.recent.open")}
                </Button>
            </div>
        );
    };

    const renderDraftRow = (item: PromptListItem) => {
        const { date, time } = formatDateTime(item.updated_at, i18n.language);
        return (
            <div
                key={item.id}
                className="flex items-start justify-between gap-3 rounded-2xl border border-dashed border-slate-200 bg-white/80 px-4 py-3 transition-colors hover:border-primary/40 hover:bg-white dark:border-slate-800/70 dark:bg-slate-900/60 dark:hover:border-primary/40"
            >
                <div className="space-y-1">
                    <button
                        type="button"
                        onClick={() => handleOpenPrompt(item.id)}
                        className="bg-transparent text-left text-sm font-medium text-slate-800 transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:text-slate-100"
                    >
                        {item.topic}
                    </button>
                    <div className="text-xs text-slate-500 dark:text-slate-400">
                        {t("dashboard.drafts.updatedAt", { date, time })}
                    </div>
                </div>
                <Button
                    variant="secondary"
                    size="sm"
                    className="gap-2 whitespace-nowrap text-xs"
                    onClick={() => handleOpenPrompt(item.id)}
                >
                    <Sparkles className="h-4 w-4" aria-hidden="true" />
                    {t("dashboard.drafts.resume")}
                </Button>
            </div>
        );
    };

    const draftItems = draftData?.items.slice(0, RECENT_PROMPT_LIMIT) ?? [];
    const enabledModels = (modelsData ?? []).filter(
        (entry) => entry.status === "enabled"
    );

    return (
        <div className="space-y-6 text-slate-700 transition-colors dark:text-slate-200">
            <SpotlightHero
                eyebrow={t("dashboard.eyebrow")}
                title={t("dashboard.welcome", { name: displayName })}
                description={t("dashboard.subtitle")}
                ctaLabel={t("dashboard.openWorkbench")}
                onCtaClick={() => navigate("/prompt-workbench")}
                metrics={metrics}
            />

            <div className="grid gap-4 lg:grid-cols-3">
                <GlassCard className="lg:col-span-2 space-y-4">
                    <div className="flex items-center justify-between gap-3">
                        <div>
                            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                {t("dashboard.search.title")}
                            </h2>
                            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                                {t("dashboard.search.subtitle")}
                            </p>
                        </div>
                    </div>
                    <form
                        className="flex flex-col gap-3 md:flex-row md:items-center"
                        onSubmit={handleSearchSubmit}
                    >
                        <SpotlightSearch
                            value={searchInput}
                            onChange={(event) => setSearchInput(event.target.value)}
                            placeholder={t("dashboard.search.placeholder")}
                            className="flex-1"
                            name="dashboard-search"
                        />
                        <Button type="submit" size="sm" className="md:w-auto">
                            {t("dashboard.search.submit")}
                        </Button>
                    </form>

                    <div className="space-y-2">
                        <div className="flex items-center justify-between text-xs font-medium uppercase tracking-[0.2em] text-slate-400 dark:text-slate-500">
                            <span className="inline-flex items-center gap-1">
                                <History className="h-3.5 w-3.5" />
                                {t("dashboard.search.historyTitle")}
                            </span>
                            {searchHistory.length > 0 ? (
                                <button
                                    type="button"
                                    onClick={handleClearHistory}
                                    className="text-slate-400 transition-colors hover:text-primary dark:text-slate-500"
                                >
                                    {t("dashboard.search.clear")}
                                </button>
                            ) : null}
                        </div>
                        {searchHistory.length === 0 ? (
                            <p className="text-xs text-slate-400 dark:text-slate-500">
                                {t("dashboard.search.historyEmpty")}
                            </p>
                        ) : (
                            <div className="flex flex-wrap gap-2">
                                {searchHistory.map((item) => (
                                    <div
                                        key={item}
                                        className="group inline-flex items-center gap-1 rounded-full border border-slate-200 bg-white px-3 py-1 text-xs text-slate-600 transition-colors hover:border-primary/40 hover:text-primary dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-primary/40"
                                    >
                                        <button
                                            type="button"
                                            onClick={() => handleSelectHistory(item)}
                                            className="font-medium"
                                        >
                                            {item}
                                        </button>
                                        <button
                                            type="button"
                                            onClick={() => handleRemoveHistory(item)}
                                            className="rounded-full p-0.5 text-slate-300 transition-colors hover:text-rose-500 dark:text-slate-500 dark:hover:text-rose-400"
                                            aria-label={t("dashboard.search.removeHistory")}
                                        >
                                            <X className="h-3 w-3" />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>

                    <div className="space-y-3">
                        <div className="flex items-center justify-between text-xs font-medium uppercase tracking-[0.2em] text-slate-400 dark:text-slate-500">
                            <span>{t("dashboard.search.resultsTitle")}</span>
                            {activeSearch ? (
                                <span className="text-[10px] font-semibold text-primary">
                                    {activeSearch}
                                </span>
                            ) : null}
                        </div>
                        {searchLoading ? (
                            <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                                <LoaderCircle className="h-4 w-4 animate-spin" />
                                {t("dashboard.search.loading")}
                            </div>
                        ) : searchError ? (
                            <p className="text-xs text-rose-500 dark:text-rose-400">{searchError}</p>
                        ) : searchResults.length === 0 ? (
                            <p className="text-xs text-slate-400 dark:text-slate-500">
                                {activeSearch
                                    ? t("dashboard.search.noResults", { query: activeSearch })
                                    : t("dashboard.search.awaiting")}
                            </p>
                        ) : (
                            <div className="space-y-2">
                                {searchResults.map((item) => renderPromptRow(item))}
                            </div>
                        )}
                    </div>
                </GlassCard>

                <GlassCard className="space-y-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                {t("dashboard.drafts.title")}
                            </h2>
                            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                                {t("dashboard.drafts.subtitle")}
                            </p>
                        </div>
                        <Badge variant="outline" className="gap-1 whitespace-nowrap text-xs">
                            <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                            {t("dashboard.drafts.count", { count: draftCount })}
                        </Badge>
                    </div>
                    {draftLoading ? (
                        <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                            <LoaderCircle className="h-4 w-4 animate-spin" />
                            {t("dashboard.drafts.loading")}
                        </div>
                    ) : draftError ? (
                        <p className="text-xs text-rose-500 dark:text-rose-400">
                            {draftError instanceof Error
                                ? draftError.message
                                : t("errors.generic")}
                        </p>
                    ) : draftItems.length === 0 ? (
                        <div className="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-400 dark:border-slate-700 dark:text-slate-500">
                            {t("dashboard.drafts.empty")}
                        </div>
                    ) : (
                        <div className="space-y-2">
                            {draftItems.map((item) => renderDraftRow(item))}
                        </div>
                    )}
                    <Button
                        variant="ghost"
                        size="sm"
                        className="gap-2 text-xs"
                        onClick={() => navigate("/prompts")}
                    >
                        {t("dashboard.drafts.viewAll")}
                        <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
                    </Button>
                </GlassCard>
            </div>

            <div className="grid gap-4 lg:grid-cols-3">
                <GlassCard className="lg:col-span-2 space-y-3">
                    <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                            {t("dashboard.recent.title")}
                        </h2>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="gap-2 text-xs"
                            onClick={() => navigate("/prompts")}
                        >
                            {t("dashboard.recent.viewAll")}
                            <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
                        </Button>
                    </div>
                    {allLoading ? (
                        <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                            <LoaderCircle className="h-4 w-4 animate-spin" />
                            {t("dashboard.recent.loading")}
                        </div>
                    ) : allError ? (
                        <p className="text-xs text-rose-500 dark:text-rose-400">
                            {allError instanceof Error
                                ? allError.message
                                : t("errors.generic")}
                        </p>
                    ) : recentPrompts.length === 0 ? (
                        <div className="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-400 dark:border-slate-700 dark:text-slate-500">
                            {t("dashboard.recent.empty")}
                        </div>
                    ) : (
                        <div className="space-y-2">
                            {recentPrompts.map((item) => renderPromptRow(item))}
                        </div>
                    )}
                </GlassCard>

                <GlassCard className="space-y-4">
                    <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                            {t("dashboard.models.title")}
                        </h2>
                        <TrendingUp className="h-5 w-5 text-primary" aria-hidden="true" />
                    </div>
                    <p className="text-sm text-slate-500 dark:text-slate-400">
                        {t("dashboard.models.subtitle", {
                            enabled: enabledModels.length,
                            total: modelsData?.length ?? 0
                        })}
                    </p>
                    {modelsLoading ? (
                        <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                            <LoaderCircle className="h-4 w-4 animate-spin" />
                            {t("dashboard.models.loading")}
                        </div>
                    ) : modelsError ? (
                        <p className="text-xs text-rose-500 dark:text-rose-400">
                            {modelsError instanceof Error
                                ? modelsError.message
                                : t("errors.generic")}
                        </p>
                    ) : (modelsData ?? []).length === 0 ? (
                        <div className="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-400 dark:border-slate-700 dark:text-slate-500">
                            {t("dashboard.models.empty")}
                        </div>
                    ) : (
                        <div className="space-y-3">
                            {(modelsData ?? []).slice(0, RECENT_PROMPT_LIMIT).map((model) => {
                                const statusLabel =
                                    model.status === "enabled"
                                        ? t("settings.modelCard.statusEnabled")
                                        : t("settings.modelCard.statusDisabled");
                                const { date, time } = formatDateTime(
                                    model.last_verified_at,
                                    i18n.language
                                );
                                return (
                                    <div
                                        key={model.id}
                                        className="rounded-2xl border border-white/60 bg-white/70 px-4 py-3 transition-colors hover:border-primary/40 hover:bg-white dark:border-slate-800/60 dark:bg-slate-900/60 dark:hover:border-primary/40"
                                    >
                                        <div className="flex items-center justify-between gap-2">
                                            <div>
                                                <div className="text-sm font-semibold text-slate-800 dark:text-slate-100">
                                                    {model.display_name}
                                                </div>
                                                <div className="text-xs text-slate-500 dark:text-slate-400">
                                                    {model.provider} · {model.model_key}
                                                </div>
                                            </div>
                                            <span
                                                className={cn(
                                                    "flex items-center gap-1 text-xs font-medium",
                                                    model.status === "enabled"
                                                        ? "text-emerald-600 dark:text-emerald-300"
                                                        : "text-slate-500 dark:text-slate-400"
                                                )}
                                                title={statusLabel}
                                            >
                                                {model.status === "enabled" ? (
                                                    <CheckCircle className="h-4 w-4" aria-hidden="true" />
                                                ) : (
                                                    <XCircle className="h-4 w-4" aria-hidden="true" />
                                                )}
                                                <span className="sr-only">{statusLabel}</span>
                                            </span>
                                        </div>
                                        <div className="mt-2 text-xs text-slate-500 dark:text-slate-400">
                                            {date !== "—"
                                                ? t("dashboard.models.verifiedAt", {
                                                      date,
                                                      time: time ? ` ${time}` : ""
                                                  })
                                                : t("dashboard.models.neverVerified")}
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                    <Button
                        variant="ghost"
                        size="sm"
                        className="gap-2 text-xs"
                        onClick={() => navigate("/settings?tab=models")}
                    >
                        {t("dashboard.models.manage")}
                        <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
                    </Button>
                </GlassCard>
            </div>
        </div>
    );
}
