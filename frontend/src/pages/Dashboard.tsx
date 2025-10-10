/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 00:38:36
 * @FilePath: \electron-go-app\frontend\src\pages\Dashboard.tsx
 * @LastEditTime: 2025-10-10 00:38:41
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import {
    ArrowUpRight,
    BarChart3,
    Clock,
    FileText,
    LucideIcon,
    Sparkles,
    TrendingUp
} from "lucide-react";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { GlassCard } from "../components/ui/glass-card";
import { useAuth } from "../hooks/useAuth";

interface Metric {
    id: string;
    label: string;
    value: string;
    delta: string;
    icon: LucideIcon;
    trend: "up" | "down" | "neutral";
}

interface ActivityItem {
    id: number;
    time: string;
    label: string;
    note?: string;
    tone: "positive" | "neutral";
}

interface PromptPerformanceRow {
    id: number;
    title: string;
    model: string;
    conversion: string;
    movement: string;
    improving: boolean;
}

interface KeywordInsight {
    id: number;
    keyword: string;
    lift: string;
}

function MetricCard({ icon: Icon, label, value, delta, trend }: Metric): JSX.Element {
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
            <p
                className={
                    "mt-4 text-xs font-medium " +
                    (trend === "up" ? "text-emerald-600" : trend === "down" ? "text-rose-500" : "text-slate-500")
                }
            >
                {delta}
            </p>
        </GlassCard>
    );
}

export default function DashboardPage(): JSX.Element {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const profile = useAuth((state) => state.profile);

    const displayName = profile?.user.username ?? profile?.user.email ?? "Creator";

    // TODO：指标卡当前使用本地 Mock 数据，等后端统计接口就绪后替换为实时数据。
    const metrics = useMemo<Metric[]>(
        () => [
            {
                id: "active",
                label: t("dashboard.metrics.activePrompts"),
                value: "24",
                delta: t("dashboard.trend.up", { value: "12%" }),
                icon: Sparkles,
                trend: "up"
            },
            {
                id: "drafts",
                label: t("dashboard.metrics.drafts"),
                value: "9",
                delta: t("dashboard.trend.down", { value: "5%" }),
                icon: FileText,
                trend: "down"
            },
            {
                id: "successRate",
                label: t("dashboard.metrics.successRate"),
                value: "68%",
                delta: t("dashboard.trend.up", { value: "4%" }),
                icon: BarChart3,
                trend: "up"
            }
        ],
        [t]
    );

    // 最近动态时间线同样使用静态数据，后续对接活动流接口。
    const activity = useMemo<ActivityItem[]>(
        () => [
            {
                id: 1,
                time: "09:24",
                label: t("dashboard.activityItems.promptPublished", { title: "AI onboarding guide" }),
                tone: "positive"
            },
            {
                id: 2,
                time: "08:47",
                label: t("dashboard.activityItems.promptUpdated", { title: "Interview follow-up" }),
                note: "GPT-4.1 mini",
                tone: "neutral"
            },
            {
                id: 3,
                time: "Yesterday",
                label: t("dashboard.activityItems.keywordAdded", { title: "product vision" }),
                note: "+3% completion",
                tone: "positive"
            }
        ],
        [t]
    );

    // 提示词表现列表用于展示未来的分析数据结构。
    const promptPerformance = useMemo<PromptPerformanceRow[]>(
        () => [
            {
                id: 1,
                title: "Growth sync prep",
                model: "GPT-4.1",
                conversion: "74%",
                movement: "+6%",
                improving: true
            },
            {
                id: 2,
                title: "Candidate brief",
                model: "o3-mini",
                conversion: "62%",
                movement: "+2%",
                improving: true
            },
            {
                id: 3,
                title: "Retro facilitator",
                model: "GPT-3.5",
                conversion: "48%",
                movement: "-3%",
                improving: false
            }
        ],
        []
    );

    // 关键词洞察模块突出近期开启度最高的关键词，暂为示例数据。
    const keywordInsights = useMemo<KeywordInsight[]>(
        () => [
            { id: 1, keyword: "user empathy", lift: "+18%" },
            { id: 2, keyword: "product vision", lift: "+12%" },
            { id: 3, keyword: "launch plan", lift: "+9%" }
        ],
        []
    );

    return (
        <div className="space-y-6 text-slate-700 transition-colors dark:text-slate-200">
            <div className="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
                <div className="space-y-2">
                    <Badge className="w-fit" variant="outline">
                        {t("dashboard.title")}
                    </Badge>
                    <h1 className="text-3xl font-semibold text-slate-900 dark:text-slate-100 sm:text-4xl">
                        {t("dashboard.welcome", { name: displayName })}
                    </h1>
                    <p className="text-sm text-slate-500 dark:text-slate-400 sm:text-base">{t("dashboard.subtitle")}</p>
                </div>
                <div className="flex flex-wrap gap-3">
                    <Button
                        variant="secondary"
                        size="lg"
                        onClick={() => navigate("/prompt-workbench")}
                        className="gap-2"
                    >
                        <Sparkles className="h-4 w-4" aria-hidden="true" />
                        {t("dashboard.openWorkbench")}
                    </Button>
                    <Button
                        variant="outline"
                        size="lg"
                        onClick={() => navigate("/prompt-workbench?tab=create")}
                        className="gap-2"
                    >
                        <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
                        {t("dashboard.createPrompt")}
                    </Button>
                </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {metrics.map((metric) => (
                    <MetricCard key={metric.id} {...metric} />
                ))}
            </div>

            <div className="grid gap-4 xl:grid-cols-5">
                <GlassCard className="xl:col-span-3">
                    <div className="flex items-center justify-between gap-4">
                        <div>
                            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{t("dashboard.activity.title")}</h2>
                            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                                {t("dashboard.subtitle")}
                            </p>
                        </div>
                        <Badge variant="outline" className="gap-1 text-xs">
                            <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                            {t("common.loading")}
                        </Badge>
                    </div>
                    <div className="mt-6 space-y-4">
                        {activity.length === 0 ? (
                            <p className="text-sm text-slate-500 dark:text-slate-400">{t("dashboard.activity.empty")}</p>
                        ) : (
                            activity.map((item) => (
                                <div key={item.id} className="flex items-start gap-3">
                                    <div className="mt-1 h-2.5 w-2.5 rounded-full bg-gradient-to-br from-primary to-secondary" />
                                    <div className="grow space-y-1">
                                        <div className="flex flex-wrap items-center gap-2">
                                            <span className="text-xs font-medium uppercase tracking-wide text-slate-400 dark:text-slate-500">
                                                {item.time}
                                            </span>
                                            <span
                                                className={
                                                    "text-sm " +
                                                    (item.tone === "positive"
                                                        ? "text-slate-900 dark:text-slate-100"
                                                        : "text-slate-600 dark:text-slate-400")
                                                }
                                            >
                                                {item.label}
                                            </span>
                                        </div>
                                        {item.note ? (
                                            <span className="text-xs text-slate-500 dark:text-slate-400">{item.note}</span>
                                        ) : null}
                                    </div>
                                </div>
                            ))
                        )}
                    </div>
                </GlassCard>

                <GlassCard className="xl:col-span-2">
                    <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{t("dashboard.performance.title")}</h2>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="gap-2 text-xs"
                            onClick={() => navigate("/prompts")}
                        >
                            {t("dashboard.performance.cta")}
                            <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
                        </Button>
                    </div>
                    <div className="mt-5 overflow-hidden rounded-2xl border border-white/60 dark:border-slate-800">
                        <table className="min-w-full divide-y divide-white/60 dark:divide-slate-800">
                            <thead className="bg-white/30 text-left text-xs uppercase tracking-wide text-slate-500 dark:bg-slate-900/60 dark:text-slate-400">
                                <tr>
                                    <th className="px-4 py-3 font-medium">{t("dashboard.performance.columns.prompt")}</th>
                                    <th className="px-4 py-3 font-medium">{t("dashboard.performance.columns.model")}</th>
                                    <th className="px-4 py-3 font-medium text-right">{t("dashboard.performance.columns.conversion")}</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-white/50 text-sm text-slate-700 dark:divide-slate-800 dark:text-slate-300">
                                {promptPerformance.map((row) => (
                                    <tr key={row.id}>
                                        <td className="px-4 py-3">
                                            <div className="font-medium text-slate-900 dark:text-slate-100">{row.title}</div>
                                            <div className="text-xs text-slate-500 dark:text-slate-400">{row.movement}</div>
                                        </td>
                                        <td className="px-4 py-3 text-slate-600 dark:text-slate-400">{row.model}</td>
                                        <td className="px-4 py-3 text-right">
                                            <span
                                                className={
                                                    "font-semibold " +
                                                    (row.improving ? "text-emerald-600" : "text-rose-500")
                                                }
                                            >
                                                {row.conversion}
                                            </span>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </GlassCard>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
                <GlassCard>
                    <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{t("dashboard.insights.title")}</h2>
                        <TrendingUp className="h-5 w-5 text-primary" aria-hidden="true" />
                    </div>
                    <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{t("dashboard.insights.subtitle")}</p>
                    <div className="mt-5 space-y-3">
                        {keywordInsights.map((insight) => (
                            <div key={insight.id} className="flex items-center justify-between rounded-2xl bg-white/60 px-4 py-3 transition-colors dark:bg-slate-900/60">
                                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">{insight.keyword}</span>
                                <Badge className="bg-emerald-50 text-emerald-600 dark:bg-emerald-500/20 dark:text-emerald-200">{insight.lift}</Badge>
                            </div>
                        ))}
                    </div>
                </GlassCard>

                <GlassCard className="relative overflow-hidden">
                    <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{t("dashboard.metrics.successRate")}</h2>
                        <div className="rounded-full bg-primary/10 p-3 text-primary">
                            <TrendingUp className="h-5 w-5" aria-hidden="true" />
                        </div>
                    </div>
                    <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                        {t("dashboard.trend.up", { value: "4%" })}
                    </p>
                    <div className="mt-6 space-y-4">
                        {[68, 72, 74].map((value, index) => (
                            <div key={value} className="space-y-2">
                                <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
                                    <span>Week {index + 1}</span>
                                    <span>{value}%</span>
                                </div>
                                <div className="h-2 overflow-hidden rounded-full bg-white/60 dark:bg-slate-800/80">
                                    <div
                                        className="h-full rounded-full bg-gradient-to-r from-primary to-secondary"
                                        style={{ width: `${value}%` }}
                                    />
                                </div>
                            </div>
                        ))}
                    </div>
                </GlassCard>
            </div>
        </div>
    );
}
