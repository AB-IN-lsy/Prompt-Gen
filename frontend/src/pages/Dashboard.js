import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 00:38:36
 * @FilePath: \electron-go-app\frontend\src\pages\Dashboard.tsx
 * @LastEditTime: 2025-10-10 00:38:41
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { ArrowUpRight, BarChart3, Clock, FileText, Sparkles, TrendingUp } from "lucide-react";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { GlassCard } from "../components/ui/glass-card";
import { useAuth } from "../hooks/useAuth";
function MetricCard({ icon: Icon, label, value, delta, trend }) {
    return (_jsxs(GlassCard, { className: "relative overflow-hidden", children: [_jsxs("div", { className: "flex items-start justify-between gap-4", children: [_jsxs("div", { children: [_jsx("p", { className: "text-sm text-slate-500 dark:text-slate-400", children: label }), _jsx("p", { className: "mt-3 text-3xl font-semibold text-slate-900 dark:text-slate-100", children: value })] }), _jsx("div", { className: "rounded-2xl bg-primary/10 p-3 text-primary shadow-glow", children: _jsx(Icon, { className: "h-6 w-6", "aria-hidden": "true" }) })] }), _jsx("p", { className: "mt-4 text-xs font-medium " +
                    (trend === "up" ? "text-emerald-600" : trend === "down" ? "text-rose-500" : "text-slate-500"), children: delta })] }));
}
export default function DashboardPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const profile = useAuth((state) => state.profile);
    const displayName = profile?.user.username ?? profile?.user.email ?? "Creator";
    // TODO：指标卡当前使用本地 Mock 数据，等后端统计接口就绪后替换为实时数据。
    const metrics = useMemo(() => [
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
    ], [t]);
    // 最近动态时间线同样使用静态数据，后续对接活动流接口。
    const activity = useMemo(() => [
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
    ], [t]);
    // 提示词表现列表用于展示未来的分析数据结构。
    const promptPerformance = useMemo(() => [
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
    ], []);
    // 关键词洞察模块突出近期开启度最高的关键词，暂为示例数据。
    const keywordInsights = useMemo(() => [
        { id: 1, keyword: "user empathy", lift: "+18%" },
        { id: 2, keyword: "product vision", lift: "+12%" },
        { id: 3, keyword: "launch plan", lift: "+9%" }
    ], []);
    return (_jsxs("div", { className: "space-y-6 text-slate-700 transition-colors dark:text-slate-200", children: [_jsxs("div", { className: "flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between", children: [_jsxs("div", { className: "space-y-2", children: [_jsx(Badge, { className: "w-fit", variant: "outline", children: t("dashboard.title") }), _jsx("h1", { className: "text-3xl font-semibold text-slate-900 dark:text-slate-100 sm:text-4xl", children: t("dashboard.welcome", { name: displayName }) }), _jsx("p", { className: "text-sm text-slate-500 dark:text-slate-400 sm:text-base", children: t("dashboard.subtitle") })] }), _jsxs("div", { className: "flex flex-wrap gap-3", children: [_jsxs(Button, { variant: "secondary", size: "lg", onClick: () => navigate("/prompt-workbench"), className: "gap-2", children: [_jsx(Sparkles, { className: "h-4 w-4", "aria-hidden": "true" }), t("dashboard.openWorkbench")] }), _jsxs(Button, { variant: "outline", size: "lg", onClick: () => navigate("/prompt-workbench?tab=create"), className: "gap-2", children: [_jsx(ArrowUpRight, { className: "h-4 w-4", "aria-hidden": "true" }), t("dashboard.createPrompt")] })] })] }), _jsx("div", { className: "grid gap-4 md:grid-cols-2 xl:grid-cols-3", children: metrics.map((metric) => (_jsx(MetricCard, { ...metric }, metric.id))) }), _jsxs("div", { className: "grid gap-4 xl:grid-cols-5", children: [_jsxs(GlassCard, { className: "xl:col-span-3", children: [_jsxs("div", { className: "flex items-center justify-between gap-4", children: [_jsxs("div", { children: [_jsx("h2", { className: "text-lg font-semibold text-slate-900 dark:text-slate-100", children: t("dashboard.activity.title") }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("dashboard.subtitle") })] }), _jsxs(Badge, { variant: "outline", className: "gap-1 text-xs", children: [_jsx(Clock, { className: "h-3.5 w-3.5", "aria-hidden": "true" }), t("common.loading")] })] }), _jsx("div", { className: "mt-6 space-y-4", children: activity.length === 0 ? (_jsx("p", { className: "text-sm text-slate-500 dark:text-slate-400", children: t("dashboard.activity.empty") })) : (activity.map((item) => (_jsxs("div", { className: "flex items-start gap-3", children: [_jsx("div", { className: "mt-1 h-2.5 w-2.5 rounded-full bg-gradient-to-br from-primary to-secondary" }), _jsxs("div", { className: "grow space-y-1", children: [_jsxs("div", { className: "flex flex-wrap items-center gap-2", children: [_jsx("span", { className: "text-xs font-medium uppercase tracking-wide text-slate-400 dark:text-slate-500", children: item.time }), _jsx("span", { className: "text-sm " +
                                                                (item.tone === "positive"
                                                                    ? "text-slate-900 dark:text-slate-100"
                                                                    : "text-slate-600 dark:text-slate-400"), children: item.label })] }), item.note ? (_jsx("span", { className: "text-xs text-slate-500 dark:text-slate-400", children: item.note })) : null] })] }, item.id)))) })] }), _jsxs(GlassCard, { className: "xl:col-span-2", children: [_jsxs("div", { className: "flex items-center justify-between", children: [_jsx("h2", { className: "text-lg font-semibold text-slate-900 dark:text-slate-100", children: t("dashboard.performance.title") }), _jsxs(Button, { variant: "ghost", size: "sm", className: "gap-2 text-xs", onClick: () => navigate("/prompts"), children: [t("dashboard.performance.cta"), _jsx(ArrowUpRight, { className: "h-3 w-3", "aria-hidden": "true" })] })] }), _jsx("div", { className: "mt-5 overflow-hidden rounded-2xl border border-white/60 dark:border-slate-800", children: _jsxs("table", { className: "min-w-full divide-y divide-white/60 dark:divide-slate-800", children: [_jsx("thead", { className: "bg-white/30 text-left text-xs uppercase tracking-wide text-slate-500 dark:bg-slate-900/60 dark:text-slate-400", children: _jsxs("tr", { children: [_jsx("th", { className: "px-4 py-3 font-medium", children: t("dashboard.performance.columns.prompt") }), _jsx("th", { className: "px-4 py-3 font-medium", children: t("dashboard.performance.columns.model") }), _jsx("th", { className: "px-4 py-3 font-medium text-right", children: t("dashboard.performance.columns.conversion") })] }) }), _jsx("tbody", { className: "divide-y divide-white/50 text-sm text-slate-700 dark:divide-slate-800 dark:text-slate-300", children: promptPerformance.map((row) => (_jsxs("tr", { children: [_jsxs("td", { className: "px-4 py-3", children: [_jsx("div", { className: "font-medium text-slate-900 dark:text-slate-100", children: row.title }), _jsx("div", { className: "text-xs text-slate-500 dark:text-slate-400", children: row.movement })] }), _jsx("td", { className: "px-4 py-3 text-slate-600 dark:text-slate-400", children: row.model }), _jsx("td", { className: "px-4 py-3 text-right", children: _jsx("span", { className: "font-semibold " +
                                                                (row.improving ? "text-emerald-600" : "text-rose-500"), children: row.conversion }) })] }, row.id))) })] }) })] })] }), _jsxs("div", { className: "grid gap-4 lg:grid-cols-2", children: [_jsxs(GlassCard, { children: [_jsxs("div", { className: "flex items-center justify-between", children: [_jsx("h2", { className: "text-lg font-semibold text-slate-900 dark:text-slate-100", children: t("dashboard.insights.title") }), _jsx(TrendingUp, { className: "h-5 w-5 text-primary", "aria-hidden": "true" })] }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("dashboard.insights.subtitle") }), _jsx("div", { className: "mt-5 space-y-3", children: keywordInsights.map((insight) => (_jsxs("div", { className: "flex items-center justify-between rounded-2xl bg-white/60 px-4 py-3 transition-colors dark:bg-slate-900/60", children: [_jsx("span", { className: "text-sm font-medium text-slate-700 dark:text-slate-300", children: insight.keyword }), _jsx(Badge, { className: "bg-emerald-50 text-emerald-600 dark:bg-emerald-500/20 dark:text-emerald-200", children: insight.lift })] }, insight.id))) })] }), _jsxs(GlassCard, { className: "relative overflow-hidden", children: [_jsxs("div", { className: "flex items-center justify-between", children: [_jsx("h2", { className: "text-lg font-semibold text-slate-900 dark:text-slate-100", children: t("dashboard.metrics.successRate") }), _jsx("div", { className: "rounded-full bg-primary/10 p-3 text-primary", children: _jsx(TrendingUp, { className: "h-5 w-5", "aria-hidden": "true" }) })] }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("dashboard.trend.up", { value: "4%" }) }), _jsx("div", { className: "mt-6 space-y-4", children: [68, 72, 74].map((value, index) => (_jsxs("div", { className: "space-y-2", children: [_jsxs("div", { className: "flex items-center justify-between text-xs text-slate-500 dark:text-slate-400", children: [_jsxs("span", { children: ["Week ", index + 1] }), _jsxs("span", { children: [value, "%"] })] }), _jsx("div", { className: "h-2 overflow-hidden rounded-full bg-white/60 dark:bg-slate-800/80", children: _jsx("div", { className: "h-full rounded-full bg-gradient-to-r from-primary to-secondary", style: { width: `${value}%` } }) })] }, value))) })] })] })] }));
}
