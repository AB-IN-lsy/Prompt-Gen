import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:47:19
 * @FilePath: \electron-go-app\frontend\src\pages\PromptWorkbench.tsx
 * @LastEditTime: 2025-10-09 22:47:24
 */
import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { LoaderCircle, Plus, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { GlassCard } from "../components/ui/glass-card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Textarea } from "../components/ui/textarea";
import { cn } from "../lib/utils";
import { regeneratePrompt, fetchKeywords, saveDraft } from "../lib/api";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
const TOPIC_FALLBACK = "前端工程师面试";
export default function PromptWorkbenchPage() {
    const { t } = useTranslation();
    // 全局状态存入 Zustand，方便在多个组件间共享提示词上下文。
    const { topic, setTopic, model, setModel, positiveKeywords, negativeKeywords, setKeywords, addKeyword, removeKeyword, prompt, setPrompt, isSaving, setSaving, } = usePromptWorkbench();
    const [newKeyword, setNewKeyword] = useState("");
    const [polarity, setPolarity] = useState("positive");
    const defaultTopic = t("promptWorkbench.topicDefault", { defaultValue: TOPIC_FALLBACK });
    // 根据主题动态拉取推荐关键词，依赖 React Query 做缓存与请求状态管理。
    const { data: keywordData, isFetching } = useQuery({
        queryKey: ["keywords", topic],
        queryFn: () => fetchKeywords({ topic }),
        enabled: !!topic,
    });
    useEffect(() => {
        if (!topic) {
            setTopic(defaultTopic);
        }
    }, [defaultTopic, setTopic, topic]);
    useEffect(() => {
        if (keywordData) {
            setKeywords(keywordData);
        }
    }, [keywordData, setKeywords]);
    // 转换关键词数据结构，方便和后端交互保持一致。
    const toPromptKeywordRefs = (keywords) => keywords.map((item) => ({
        keywordId: item.id,
        word: item.word,
        weight: item.weight,
    }));
    // 触发 AI 生成逻辑，成功后替换当前草稿。
    const generateMutation = useMutation({
        mutationFn: () => regeneratePrompt({
            topic,
            model,
            prompt,
            positiveKeywords: toPromptKeywordRefs(positiveKeywords),
            negativeKeywords: toPromptKeywordRefs(negativeKeywords),
            status: "draft",
        }),
        onSuccess: (result) => setPrompt(result.prompt),
    });
    // 保存草稿到后端，配合 isSaving 状态做按钮/提示展示。
    const saveMutation = useMutation({
        mutationFn: async () => {
            setSaving(true);
            await saveDraft({
                topic,
                model,
                prompt,
                positiveKeywords: toPromptKeywordRefs(positiveKeywords),
                negativeKeywords: toPromptKeywordRefs(negativeKeywords),
                status: "draft",
            });
        },
        onSettled: () => setSaving(false),
    });
    const hasPositive = positiveKeywords.length > 0;
    const handleAddKeyword = () => {
        if (!newKeyword.trim())
            return;
        const keyword = {
            word: newKeyword.trim(),
            polarity,
            source: "manual",
            weight: 5,
        };
        // 新增关键词默认权重为 5，可根据实际需求做 UX 优化。
        addKeyword(keyword);
        setNewKeyword("");
    };
    // 把权重高的关键词排在前面，让用户更容易关注重点。
    const sortedPositive = useMemo(() => [...positiveKeywords].sort((a, b) => b.weight - a.weight), [positiveKeywords]);
    const sortedNegative = useMemo(() => [...negativeKeywords].sort((a, b) => b.weight - a.weight), [negativeKeywords]);
    return (_jsxs("div", { className: "grid grid-cols-1 gap-6 text-slate-700 transition-colors dark:text-slate-200 xl:grid-cols-[380px,minmax(0,1fr)]", children: [_jsxs(GlassCard, { className: "flex flex-col gap-6", children: [_jsxs("header", { className: "flex items-center justify-between", children: [_jsxs("div", { children: [_jsx("p", { className: "text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500", children: t("promptWorkbench.keywordsTitle") }), _jsx("h2", { className: "mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100", children: t("promptWorkbench.keywordsSubtitle") })] }), isFetching ? _jsx(LoaderCircle, { className: "h-5 w-5 animate-spin text-primary" }) : null] }), _jsxs("div", { className: "rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70", children: [_jsx("p", { className: "text-sm font-medium text-slate-600 dark:text-slate-300", children: t("promptWorkbench.addKeyword") }), _jsxs("div", { className: "mt-3 flex gap-2", children: [_jsx(Input, { value: newKeyword, onChange: (event) => setNewKeyword(event.target.value), placeholder: t("promptWorkbench.inputPlaceholder") }), _jsxs(Button, { variant: "secondary", size: "sm", onClick: handleAddKeyword, children: [_jsx(Plus, { className: "mr-1.5 h-4 w-4" }), t("promptWorkbench.addKeyword")] })] }), _jsxs("div", { className: "mt-3 flex gap-2 text-xs", children: [_jsx(Badge, { className: cn("cursor-pointer px-3 py-1", polarity === "positive" && "border-transparent bg-primary text-white"), variant: polarity === "positive" ? "default" : "outline", onClick: () => setPolarity("positive"), children: t("promptWorkbench.positive") }), _jsx(Badge, { className: cn("cursor-pointer px-3 py-1", polarity === "negative" && "border-transparent bg-secondary text-white"), variant: polarity === "negative" ? "default" : "outline", onClick: () => setPolarity("negative"), children: t("promptWorkbench.negative") })] })] }), _jsx(KeywordSection, { title: t("promptWorkbench.positiveSectionTitle"), hint: t("promptWorkbench.positiveHint"), keywords: sortedPositive, onRemove: removeKeyword }), _jsx(KeywordSection, { title: t("promptWorkbench.negativeSectionTitle"), keywords: sortedNegative, onRemove: removeKeyword, tint: "negative" })] }), _jsxs(GlassCard, { className: "flex flex-col gap-6", children: [_jsxs("header", { className: "flex items-center justify-between", children: [_jsxs("div", { children: [_jsx("p", { className: "text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500", children: t("promptWorkbench.workbenchEyebrow") }), _jsx("h2", { className: "mt-1 text-2xl font-semibold text-slate-800 dark:text-slate-100", children: t("promptWorkbench.workbenchTitle") })] }), _jsxs(Button, { variant: "secondary", size: "sm", onClick: () => generateMutation.mutate(), disabled: !hasPositive || generateMutation.isPending, children: [generateMutation.isPending ? (_jsx(LoaderCircle, { className: "mr-2 h-4 w-4 animate-spin" })) : (_jsx(Sparkles, { className: "mr-2 h-4 w-4" })), t("promptWorkbench.regenerate")] })] }), _jsxs("div", { className: "grid grid-cols-1 gap-4 lg:grid-cols-2", children: [_jsxs("div", { className: "rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70", children: [_jsx("label", { className: "text-sm font-medium text-slate-600 dark:text-slate-300", children: t("promptWorkbench.topicLabel") }), _jsx(Input, { value: topic, onChange: (event) => setTopic(event.target.value), className: "mt-2" })] }), _jsxs("div", { className: "rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70", children: [_jsx("label", { className: "text-sm font-medium text-slate-600 dark:text-slate-300", children: t("promptWorkbench.modelLabel") }), _jsxs("div", { className: "mt-3 flex gap-2 text-xs", children: [_jsx(Badge, { className: cn("cursor-pointer px-3 py-1", model === "deepseek" && "border-transparent bg-primary text-white"), variant: model === "deepseek" ? "default" : "outline", onClick: () => setModel("deepseek"), children: "DeepSeek" }), _jsx(Badge, { className: cn("cursor-pointer px-3 py-1", model === "gpt-5" && "border-transparent bg-secondary text-white"), variant: model === "gpt-5" ? "default" : "outline", onClick: () => setModel("gpt-5"), children: "GPT-5" })] })] })] }), _jsxs("div", { className: "rounded-3xl border border-white/60 bg-white/80 p-5 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70", children: [_jsx("label", { className: "text-sm font-medium text-slate-600 dark:text-slate-300", children: t("promptWorkbench.draftLabel") }), _jsx(Textarea, { className: "mt-3 min-h-[280px]", value: prompt, onChange: (event) => setPrompt(event.target.value), placeholder: t("promptWorkbench.draftPlaceholder") }), _jsxs("div", { className: "mt-3 text-xs text-slate-400 dark:text-slate-500", children: [t("promptWorkbench.autosave"), ":", " ", isSaving ? t("promptWorkbench.autosaveSaving") : t("promptWorkbench.autosaveSaved")] })] }), _jsxs("div", { className: "flex flex-wrap items-center justify-end gap-3", children: [_jsx(Button, { variant: "ghost", children: t("promptWorkbench.cancel") }), _jsxs(Button, { variant: "outline", onClick: () => saveMutation.mutate(), disabled: saveMutation.isPending, children: [saveMutation.isPending ? _jsx(LoaderCircle, { className: "mr-2 h-4 w-4 animate-spin" }) : null, t("promptWorkbench.saveDraft")] }), _jsx(Button, { variant: "secondary", size: "lg", disabled: !hasPositive || !prompt.trim(), children: t("promptWorkbench.publish") })] })] })] }));
}
function KeywordSection({ title, hint, keywords, tint = "positive", onRemove }) {
    const { t } = useTranslation();
    if (!keywords.length) {
        // 无关键词时展示占位提示，保持布局稳定。
        return (_jsx("div", { className: "rounded-2xl border border-dashed border-slate-200 bg-white/60 p-6 text-center text-sm text-slate-400 transition-colors dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-500", children: t("promptWorkbench.emptySection", { title }) }));
    }
    return (_jsxs("div", { className: "space-y-3", children: [_jsxs("div", { className: "flex items-center justify-between text-sm text-slate-500 dark:text-slate-400", children: [_jsx("span", { children: title }), hint ? _jsx("span", { className: "text-xs text-slate-400 dark:text-slate-500", children: hint }) : null] }), _jsx("div", { className: "flex flex-wrap gap-2", children: keywords.map((keyword) => (_jsxs("button", { onClick: () => onRemove(keyword.id), className: cn("group flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium shadow-sm transition", tint === "positive"
                        ? "border-primary/20 bg-primary/10 text-primary hover:border-primary hover:bg-primary hover:text-white"
                        : "border-secondary/20 bg-secondary/10 text-secondary hover:border-secondary hover:bg-secondary hover:text-white"), children: [_jsx("span", { children: keyword.word }), _jsx("span", { className: "rounded-full bg-white/40 px-1.5 py-0.5 text-[10px] text-slate-500 group-hover:bg-black/10 group-hover:text-white", children: t("promptWorkbench.weight", { value: keyword.weight }) })] }, keyword.id))) })] }));
}
