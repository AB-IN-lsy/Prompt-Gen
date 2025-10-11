/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:47:19
 * @FilePath: \electron-go-app\frontend\src\pages\PromptWorkbench.tsx
 * @LastEditTime: 2025-10-11 00:19:57
 */
import { ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { LoaderCircle, Plus, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { GlassCard } from "../components/ui/glass-card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Textarea } from "../components/ui/textarea";
import { cn } from "../lib/utils";
import {
    regeneratePrompt,
    fetchKeywords,
    saveDraft,
    updateCurrentUser,
    fetchUserModels,
    type UserModelCredential,
} from "../lib/api";
import { ApiError } from "../lib/errors";
import type { Keyword, PromptKeywordRef } from "../lib/api";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import { useAuth } from "../hooks/useAuth";

const TOPIC_FALLBACK = "前端工程师面试";

export default function PromptWorkbenchPage() {
    const { t } = useTranslation();
    // 全局状态存入 Zustand，方便在多个组件间共享提示词上下文。
    const {
        topic,
        setTopic,
        model,
        setModel,
        positiveKeywords,
        negativeKeywords,
        setKeywords,
        addKeyword,
        removeKeyword,
        prompt,
        setPrompt,
        isSaving,
        setSaving,
    } = usePromptWorkbench();

    const profile = useAuth((state) => state.profile);
    const setProfile = useAuth((state) => state.setProfile);

    const [newKeyword, setNewKeyword] = useState("");
    const [polarity, setPolarity] = useState<"positive" | "negative">("positive");

    const defaultTopic = t("promptWorkbench.topicDefault", { defaultValue: TOPIC_FALLBACK });

    // 根据主题动态拉取推荐关键词，依赖 React Query 做缓存与请求状态管理。
    const { data: keywordData, isFetching } = useQuery<Keyword[]>({
        queryKey: ["keywords", topic],
        queryFn: () => fetchKeywords({ topic }),
        enabled: !!topic,
    });
    // 读取模型列表，结合偏好配置控制前端模型可选项
    const modelsQuery = useQuery<UserModelCredential[]>({
        queryKey: ["models"],
        queryFn: fetchUserModels,
    });
    const enabledModelKeys = useMemo(() => {
        if (!modelsQuery.data) {
            return [];
        }
        return modelsQuery.data
            .filter((model) => String(model.status ?? "").toLowerCase() !== "disabled")
            .map((model) => model.model_key);
    }, [modelsQuery.data]);
    const isModelSelectable = useCallback(
        (key: "deepseek" | "gpt-5") => {
            if (!modelsQuery.data || enabledModelKeys.length === 0) {
                return true;
            }
            return enabledModelKeys.includes(key);
        },
        [enabledModelKeys, modelsQuery.data],
    );

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

    useEffect(() => {
        const preferred = profile?.settings?.preferred_model;
        if (preferred === "deepseek" || preferred === "gpt-5") {
            const currentModel = usePromptWorkbench.getState().model;
            if (currentModel !== preferred) {
                setModel(preferred);
            }
        }
    }, [profile?.settings?.preferred_model, setModel]);

    // 当后端模型禁用/删除时，自动回退到可用模型并提示用户
    useEffect(() => {
        if (!modelsQuery.data) {
            return;
        }
        const currentEnabled = new Set(enabledModelKeys);
        const currentModel = model;
        const preferred = profile?.settings?.preferred_model ?? "";

        let fallbackKey: string | null = null;

        if (currentEnabled.size === 0) {
            fallbackKey = "deepseek";
        } else if (!currentEnabled.has(currentModel)) {
            if (preferred && currentEnabled.has(preferred)) {
                fallbackKey = preferred;
            } else {
                fallbackKey = Array.from(currentEnabled)[0];
            }
        }

        if (fallbackKey) {
            const fallbackModel: "deepseek" | "gpt-5" = fallbackKey === "gpt-5" ? "gpt-5" : "deepseek";
            if (fallbackModel !== currentModel) {
                setModel(fallbackModel);
                const displayName = fallbackModel === "gpt-5" ? "GPT-5" : "DeepSeek";
                toast.info(t("promptWorkbench.modelFallback", { model: displayName }));
            }
        }
    }, [enabledModelKeys, model, modelsQuery.data, profile?.settings?.preferred_model, setModel, t]);

    // 转换关键词数据结构，方便和后端交互保持一致。
    const toPromptKeywordRefs = (keywords: Keyword[]): PromptKeywordRef[] =>
        keywords.map((item) => ({
            keywordId: item.id,
            word: item.word,
            weight: item.weight,
        }));

    const modelPreferenceMutation = useMutation({
        mutationFn: (nextModel: "deepseek" | "gpt-5") => updateCurrentUser({ preferred_model: nextModel }),
        onSuccess: (updatedProfile) => setProfile(updatedProfile),
        onError: (error: unknown) => {
            const message = error instanceof ApiError ? error.message ?? t("errors.generic") : t("errors.generic");
            toast.error(message);
        },
    });

    // 触发 AI 生成逻辑，成功后替换当前草稿。
    const generateMutation = useMutation({
        mutationFn: () =>
            regeneratePrompt({
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
    const deepseekSelectable = isModelSelectable("deepseek");
    const gptSelectable = isModelSelectable("gpt-5");

    // 选择模型，同时持久化偏好
    const handleModelSelect = (nextModel: "deepseek" | "gpt-5") => {
        if (model === nextModel) {
            return;
        }
        if (!isModelSelectable(nextModel)) {
            toast.error(t("promptWorkbench.modelDisabled"));
            return;
        }
        setModel(nextModel);
        const preferred = profile?.settings?.preferred_model;
        if (!profile || preferred === nextModel) {
            return;
        }
        modelPreferenceMutation.mutate(nextModel);
    };

    const handleAddKeyword = () => {
        if (!newKeyword.trim()) return;
        const keyword: Omit<Keyword, "id"> = {
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
    const sortedPositive = useMemo(
        () => [...positiveKeywords].sort((a, b) => b.weight - a.weight),
        [positiveKeywords],
    );
    const sortedNegative = useMemo(
        () => [...negativeKeywords].sort((a, b) => b.weight - a.weight),
        [negativeKeywords],
    );

    return (
        <div className="grid grid-cols-1 gap-6 text-slate-700 transition-colors dark:text-slate-200 xl:grid-cols-[380px,minmax(0,1fr)]">
            <GlassCard className="flex flex-col gap-6">
                <header className="flex items-center justify-between">
                    <div>
                        <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">{t("promptWorkbench.keywordsTitle")}</p>
                        <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">{t("promptWorkbench.keywordsSubtitle")}</h2>
                    </div>
                    {isFetching ? <LoaderCircle className="h-5 w-5 animate-spin text-primary" /> : null}
                </header>
                <div className="rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
                    <p className="text-sm font-medium text-slate-600 dark:text-slate-300">{t("promptWorkbench.addKeyword")}</p>
                    <div className="mt-3 flex gap-2">
                        <Input
                            value={newKeyword}
                            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewKeyword(event.target.value)}
                            placeholder={t("promptWorkbench.inputPlaceholder")}
                        />
                        <Button
                            variant="secondary"
                            size="default"
                            className="min-w-[132px] justify-center px-5"
                            onClick={handleAddKeyword}
                        >
                            <Plus className="mr-1.5 h-4 w-4" />
                            {t("promptWorkbench.addKeyword")}
                        </Button>
                    </div>
                    <div className="mt-3 flex gap-2 text-xs">
                        <Badge
                            className={cn(
                                "cursor-pointer px-3 py-1",
                                polarity === "positive" && "border-transparent bg-primary text-white",
                            )}
                            variant={polarity === "positive" ? "default" : "outline"}
                            onClick={() => setPolarity("positive")}
                        >
                            {t("promptWorkbench.positive")}
                        </Badge>
                        <Badge
                            className={cn(
                                "cursor-pointer px-3 py-1",
                                polarity === "negative" && "border-transparent bg-secondary text-white",
                            )}
                            variant={polarity === "negative" ? "default" : "outline"}
                            onClick={() => setPolarity("negative")}
                        >
                            {t("promptWorkbench.negative")}
                        </Badge>
                    </div>
                </div>
                <KeywordSection
                    title={t("promptWorkbench.positiveSectionTitle")}
                    hint={t("promptWorkbench.positiveHint")}
                    keywords={sortedPositive}
                    onRemove={removeKeyword}
                />
                <KeywordSection
                    title={t("promptWorkbench.negativeSectionTitle")}
                    keywords={sortedNegative}
                    onRemove={removeKeyword}
                    tint="negative"
                />
            </GlassCard>

            <GlassCard className="flex flex-col gap-6">
                <header className="flex items-center justify-between">
                    <div>
                        <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">{t("promptWorkbench.workbenchEyebrow")}</p>
                        <h2 className="mt-1 text-2xl font-semibold text-slate-800 dark:text-slate-100">{t("promptWorkbench.workbenchTitle")}</h2>
                    </div>
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => generateMutation.mutate()}
                        disabled={!hasPositive || generateMutation.isPending}
                    >
                        {generateMutation.isPending ? (
                            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                            <Sparkles className="mr-2 h-4 w-4" />
                        )}
                        {t("promptWorkbench.regenerate")}
                    </Button>
                </header>

                <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                    <div className="rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
                        <label className="text-sm font-medium text-slate-600 dark:text-slate-300">{t("promptWorkbench.topicLabel")}</label>
                        <Input value={topic} onChange={(event) => setTopic(event.target.value)} className="mt-2" />
                    </div>
                    <div className="rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
                        <label className="text-sm font-medium text-slate-600 dark:text-slate-300">{t("promptWorkbench.modelLabel")}</label>
                        <div className="mt-3 flex gap-2 text-xs">
                            <Badge
                                className={cn(
                                    "cursor-pointer px-3 py-1",
                                    !deepseekSelectable && "cursor-not-allowed opacity-60",
                                    model === "deepseek" && "border-transparent bg-primary text-white",
                                )}
                                variant={model === "deepseek" ? "default" : "outline"}
                                onClick={() => handleModelSelect("deepseek")}
                                aria-disabled={!deepseekSelectable}
                            >
                                DeepSeek
                            </Badge>
                            <Badge
                                className={cn(
                                    "cursor-pointer px-3 py-1",
                                    !gptSelectable && "cursor-not-allowed opacity-60",
                                    model === "gpt-5" && "border-transparent bg-secondary text-white",
                                )}
                                variant={model === "gpt-5" ? "default" : "outline"}
                                onClick={() => handleModelSelect("gpt-5")}
                                aria-disabled={!gptSelectable}
                            >
                                GPT-5
                            </Badge>
                        </div>
                    </div>
                </div>

                <div className="rounded-3xl border border-white/60 bg-white/80 p-5 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70">
                    <label className="text-sm font-medium text-slate-600 dark:text-slate-300">{t("promptWorkbench.draftLabel")}</label>
                    <Textarea
                        className="mt-3 min-h-[280px]"
                        value={prompt}
                        onChange={(event) => setPrompt(event.target.value)}
                        placeholder={t("promptWorkbench.draftPlaceholder")}
                    />
                    <div className="mt-3 text-xs text-slate-400 dark:text-slate-500">
                        {t("promptWorkbench.autosave")}:{" "}
                        {isSaving ? t("promptWorkbench.autosaveSaving") : t("promptWorkbench.autosaveSaved")}
                    </div>
                </div>

                <div className="flex flex-wrap items-center justify-end gap-3">
                    <Button variant="ghost">{t("promptWorkbench.cancel")}</Button>
                    <Button
                        variant="outline"
                        onClick={() => saveMutation.mutate()}
                        disabled={saveMutation.isPending}
                    >
                        {saveMutation.isPending ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}
                        {t("promptWorkbench.saveDraft")}
                    </Button>
                    <Button variant="secondary" size="lg" disabled={!hasPositive || !prompt.trim()}>
                        {t("promptWorkbench.publish")}
                    </Button>
                </div>
            </GlassCard>
        </div>
    );
}

interface KeywordSectionProps {
    title: string;
    hint?: string;
    keywords: Keyword[];
    tint?: "positive" | "negative";
    onRemove: (id: string) => void;
}

function KeywordSection({ title, hint, keywords, tint = "positive", onRemove }: KeywordSectionProps) {
    const { t } = useTranslation();

    if (!keywords.length) {
        // 无关键词时展示占位提示，保持布局稳定。
        return (
            <div className="rounded-2xl border border-dashed border-slate-200 bg-white/60 p-6 text-center text-sm text-slate-400 transition-colors dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-500">
                {t("promptWorkbench.emptySection", { title })}
            </div>
        );
    }

    return (
        <div className="space-y-3">
            <div className="flex items-center justify-between text-sm text-slate-500 dark:text-slate-400">
                <span>{title}</span>
                {hint ? <span className="text-xs text-slate-400 dark:text-slate-500">{hint}</span> : null}
            </div>
            <div className="flex flex-wrap gap-2">
                {keywords.map((keyword) => (
                    <button
                        key={keyword.id}
                        onClick={() => onRemove(keyword.id)}
                        className={cn(
                            "group flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium shadow-sm transition",
                            tint === "positive"
                                ? "border-primary/20 bg-primary/10 text-primary hover:border-primary hover:bg-primary hover:text-white"
                                : "border-secondary/20 bg-secondary/10 text-secondary hover:border-secondary hover:bg-secondary hover:text-white",
                        )}
                    >
                        <span>{keyword.word}</span>
                        <span className="rounded-full bg-white/40 px-1.5 py-0.5 text-[10px] text-slate-500 group-hover:bg-black/10 group-hover:text-white">
                            {t("promptWorkbench.weight", { value: keyword.weight })}
                        </span>
                    </button>
                ))}
            </div>
        </div>
    );
}
