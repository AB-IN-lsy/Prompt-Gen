/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 13:22:30
 * @FilePath: \electron-go-app\frontend\src\pages\Logs.tsx
 * @LastEditTime: 2025-10-11 13:22:30
 */
import { FormEvent, useEffect, useMemo, useReducer, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Textarea } from "../components/ui/textarea";
import {
    createChangelogEntry,
    deleteChangelogEntry,
    fetchChangelogEntries,
    updateChangelogEntry,
    type ChangelogEntry,
    type ChangelogPayload,
    type ChangelogCreateResult
} from "../lib/api";
import { useAuth } from "../hooks/useAuth";
import { cn } from "../lib/utils";

interface EditorState {
    id: number | null;
    locale: string;
    badge: string;
    title: string;
    summary: string;
    itemsRaw: string;
    publishedAt: string;
}

const initialEditorState = (locale: string): EditorState => ({
    id: null,
    locale,
    badge: "",
    title: "",
    summary: "",
    itemsRaw: "",
    publishedAt: new Date().toISOString().slice(0, 10)
});

type EditorAction =
    | { type: "reset"; locale: string }
    | { type: "set"; key: keyof EditorState; value: string | number | null }
    | { type: "load"; entry: ChangelogEntry };

function editorReducer(state: EditorState, action: EditorAction): EditorState {
    switch (action.type) {
        case "reset":
            return initialEditorState(action.locale);
        case "set":
            return { ...state, [action.key]: action.value };
        case "load":
            return {
                id: action.entry.id,
                locale: action.entry.locale,
                badge: action.entry.badge,
                title: action.entry.title,
                summary: action.entry.summary,
                itemsRaw: action.entry.items.join("\n"),
                publishedAt: action.entry.published_at.slice(0, 10)
            };
        default:
            return state;
    }
}

export default function LogsPage() {
    const { t, i18n } = useTranslation();
    const isAdmin = useAuth((state) => state.profile?.user?.is_admin ?? false);
    const queryClient = useQueryClient();
    const locale = i18n.language.startsWith("zh") ? "zh-CN" : "en";
    const [editorState, dispatch] = useReducer(editorReducer, initialEditorState(locale));
    const [isDeleting, setDeleting] = useState<number | null>(null);

    const eyebrow = t("logsPage.eyebrow");
    const title = t("logsPage.title");
    const subtitle = t("logsPage.subtitle");

    const availableLocales = useMemo(
        () => [
            { value: "en", label: t("logsPage.admin.localeOption.en") },
            { value: "zh-CN", label: t("logsPage.admin.localeOption.zh-CN") }
        ],
        [t]
    );

    const [autoTranslate, setAutoTranslate] = useState(false);
    const [translateTargets, setTranslateTargets] = useState<string[]>(() =>
        locale === "zh-CN" ? ["en"] : ["zh-CN"]
    );
    const [translationModelKey, setTranslationModelKey] = useState("");

    const entriesQuery = useQuery({
        queryKey: ["changelog", locale],
        queryFn: () => fetchChangelogEntries(locale),
        staleTime: 1000 * 60 * 5
    });

    useEffect(() => {
        dispatch({ type: "reset", locale });
        setAutoTranslate(false);
        setTranslateTargets(locale === "zh-CN" ? ["en"] : ["zh-CN"]);
        setTranslationModelKey("");
    }, [locale]);

    useEffect(() => {
        setTranslateTargets((prev) => {
            const filtered = prev.filter((target) => target !== editorState.locale);
            if (filtered.length > 0) {
                return filtered;
            }
            const fallback = editorState.locale === "zh-CN" ? "en" : "zh-CN";
            return fallback === editorState.locale ? [] : [fallback];
        });
    }, [editorState.locale]);

    const createMutation = useMutation<ChangelogCreateResult, unknown, ChangelogPayload>({
        mutationFn: createChangelogEntry,
        onSuccess: (result) => {
            const translations = result.translations ?? [];
            if (translations.length > 0) {
                toast.success(t("logsPage.admin.successCreateWithTranslations", { count: translations.length }));
            } else {
                toast.success(t("logsPage.admin.successCreate"));
            }
            dispatch({ type: "reset", locale });
            setAutoTranslate(false);
            setTranslateTargets(locale === "zh-CN" ? ["en"] : ["zh-CN"]);
            setTranslationModelKey("");
            void queryClient.invalidateQueries({ queryKey: ["changelog", locale] });
        },
        onError: (error: unknown) => {
            console.error(error);
            toast.error(t("errors.generic"));
        }
    });

    const updateMutation = useMutation<ChangelogEntry, unknown, { id: number; payload: ChangelogPayload }>({
        mutationFn: ({ id, payload }: { id: number; payload: Parameters<typeof updateChangelogEntry>[1] }) =>
            updateChangelogEntry(id, payload),
        onSuccess: () => {
            toast.success(t("logsPage.admin.successUpdate"));
            dispatch({ type: "reset", locale });
            setAutoTranslate(false);
            setTranslateTargets(locale === "zh-CN" ? ["en"] : ["zh-CN"]);
            setTranslationModelKey("");
            void queryClient.invalidateQueries({ queryKey: ["changelog", locale] });
        },
        onError: (error: unknown) => {
            console.error(error);
            toast.error(t("errors.generic"));
        }
    });

    const deleteMutation = useMutation({
        mutationFn: deleteChangelogEntry,
        onSuccess: () => {
            toast.success(t("logsPage.admin.successDelete"));
            void queryClient.invalidateQueries({ queryKey: ["changelog", locale] });
        },
        onError: (error: unknown) => {
            console.error(error);
            toast.error(t("errors.generic"));
        },
        onSettled: () => {
            setDeleting(null);
        }
    });

    const isSubmitting = createMutation.isPending || updateMutation.isPending;

    const entries = entriesQuery.data ?? [];

    const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!isAdmin) {
            toast.error(t("errors.FORBIDDEN"));
            return;
        }

        const items = editorState.itemsRaw
            .split("\n")
            .map((line) => line.trim())
            .filter(Boolean);

        if (items.length === 0) {
            toast.error(t("logsPage.admin.itemsRequired"));
            return;
        }

        const payload: ChangelogPayload = {
            locale: editorState.locale,
            badge: editorState.badge,
            title: editorState.title,
            summary: editorState.summary,
            items,
            published_at: editorState.publishedAt
        };

        if (autoTranslate) {
            const targets = translateTargets.filter((target) => target !== editorState.locale);
            if (targets.length === 0) {
                toast.error(t("logsPage.admin.translateTargetsRequired"));
                return;
            }
            if (!translationModelKey.trim()) {
                toast.error(t("logsPage.admin.translateModelRequired"));
                return;
            }
            payload.translate_to = targets;
            payload.translation_model_key = translationModelKey.trim();
        }

        if (editorState.id && editorState.id > 0) {
            updateMutation.mutate({ id: editorState.id, payload });
        } else {
            createMutation.mutate(payload);
        }
    };

    const handleDelete = (id: number) => {
        if (!isAdmin) {
            toast.error(t("errors.FORBIDDEN"));
            return;
        }
        if (window.confirm(t("logsPage.admin.confirmDelete"))) {
            setDeleting(id);
            deleteMutation.mutate(id);
        }
    };

    const adminToolbar = isAdmin ? (
        <GlassCard className="space-y-4">
            <div className="space-y-1">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                    {t("logsPage.admin.title")}
                </h2>
                <p className="text-sm text-slate-500 dark:text-slate-400">{t("logsPage.admin.description")}</p>
            </div>
            <form className="space-y-4" onSubmit={handleSubmit}>
                <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                            {t("logsPage.admin.locale")}
                        </label>
                        <select
                            className="h-10 w-full rounded-xl border border-white/60 bg-white/80 px-3 text-sm text-slate-700 transition focus:outline-none focus:ring-2 focus:ring-primary dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-200"
                            value={editorState.locale}
                            onChange={(event) => dispatch({ type: "set", key: "locale", value: event.target.value })}
                        >
                            <option value="en">English</option>
                            <option value="zh-CN">简体中文</option>
                        </select>
                    </div>
                    <div className="space-y-2">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                            {t("logsPage.admin.publishedAt")}
                        </label>
                        <Input
                            value={editorState.publishedAt}
                            onChange={(event) => dispatch({ type: "set", key: "publishedAt", value: event.target.value })}
                            required
                            placeholder="2025-10-12"
                        />
                    </div>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                            {t("logsPage.admin.badge")}
                        </label>
                        <Input
                            value={editorState.badge}
                            onChange={(event) => dispatch({ type: "set", key: "badge", value: event.target.value })}
                            required
                        />
                    </div>
                    <div className="space-y-2">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                            {t("logsPage.admin.titleLabel")}
                        </label>
                        <Input
                            value={editorState.title}
                            onChange={(event) => dispatch({ type: "set", key: "title", value: event.target.value })}
                            required
                        />
                    </div>
                </div>
                <div className="space-y-2">
                    <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                        {t("logsPage.admin.summary")}
                    </label>
                    <Textarea
                        value={editorState.summary}
                        onChange={(event) => dispatch({ type: "set", key: "summary", value: event.target.value })}
                        required
                        rows={3}
                    />
                </div>
                <div className="space-y-2">
                    <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                        {t("logsPage.admin.items")}
                    </label>
                    <Textarea
                        value={editorState.itemsRaw}
                        onChange={(event) => dispatch({ type: "set", key: "itemsRaw", value: event.target.value })}
                        placeholder={t("logsPage.admin.itemsHint") ?? ""}
                        rows={4}
                    />
                </div>
                <div className="space-y-2">
                    <label className="flex items-center gap-2 text-xs font-medium text-slate-600 dark:text-slate-300">
                        <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary"
                            checked={autoTranslate}
                            onChange={(event) => setAutoTranslate(event.target.checked)}
                        />
                        {t("logsPage.admin.translate")}
                    </label>
                    <p className="text-xs text-slate-500 dark:text-slate-400">
                        {t("logsPage.admin.translateDescription")}
                    </p>
                    {autoTranslate ? (
                        <div className="space-y-3 rounded-xl border border-white/60 bg-white/60 p-4 dark:border-slate-700 dark:bg-slate-900/40">
                            <div className="space-y-2">
                                <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
                                    {t("logsPage.admin.translateTargets")}
                                </span>
                                <div className="flex flex-wrap gap-3">
                                    {availableLocales
                                        .filter((option) => option.value !== editorState.locale)
                                        .map((option) => {
                                            const checked = translateTargets.includes(option.value);
                                            return (
                                                <label
                                                    key={option.value}
                                                    className="inline-flex items-center gap-2 rounded-lg border border-white/70 bg-white px-3 py-1 text-xs text-slate-600 transition hover:bg-primary/5 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-300"
                                                >
                                                    <input
                                                        type="checkbox"
                                                        className="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary"
                                                        checked={checked}
                                                        onChange={() =>
                                                            setTranslateTargets((prev) =>
                                                                checked
                                                                    ? prev.filter((value) => value !== option.value)
                                                                    : [...prev, option.value]
                                                            )
                                                        }
                                                    />
                                                    {option.label}
                                                </label>
                                            );
                                        })}
                                </div>
                            </div>
                            <div className="space-y-1">
                                <label className="text-xs font-medium text-slate-600 dark:text-slate-300">
                                    {t("logsPage.admin.translateModelKey")}
                                </label>
                                <Input
                                    value={translationModelKey}
                                    onChange={(event) => setTranslationModelKey(event.target.value)}
                                    placeholder="deepseek-chat"
                                />
                                <p className="text-xs text-slate-500 dark:text-slate-400">
                                    {t("logsPage.admin.translateModelHint")}
                                </p>
                            </div>
                        </div>
                    ) : null}
                </div>
                <div className="flex flex-wrap gap-3">
                    <Button type="submit" disabled={isSubmitting}>
                        {editorState.id ? t("logsPage.admin.update") : t("logsPage.admin.create")}
                    </Button>
                    <Button
                        type="button"
                        variant="outline"
                        onClick={() => {
                            dispatch({ type: "reset", locale });
                            setAutoTranslate(false);
                            setTranslateTargets(locale === "zh-CN" ? ["en"] : ["zh-CN"]);
                            setTranslationModelKey("");
                        }}
                    >
                        {editorState.id ? t("logsPage.admin.cancel") : t("logsPage.admin.reset")}
                    </Button>
                </div>
            </form>
        </GlassCard>
    ) : null;

    return (
        <div className="space-y-8 text-slate-700 transition-colors dark:text-slate-200">
            <div className="space-y-3">
                <Badge variant="outline">{eyebrow}</Badge>
                <h1 className="text-3xl font-semibold text-slate-900 dark:text-white sm:text-4xl">{title}</h1>
                <p className="max-w-3xl text-sm text-slate-500 dark:text-slate-400 sm:text-base">{subtitle}</p>
            </div>

            {adminToolbar}

            {entriesQuery.isLoading ? (
                <GlassCard>
                    <p className="text-sm text-slate-500 dark:text-slate-400">{t("common.loading")}</p>
                </GlassCard>
            ) : null}

            {entriesQuery.isError ? (
                <GlassCard className="border-rose-200 bg-rose-50/70 dark:border-rose-400/40 dark:bg-rose-500/10">
                    <p className="text-sm text-rose-600 dark:text-rose-200">{t("errors.generic")}</p>
                </GlassCard>
            ) : null}

            {entries.length === 0 && !entriesQuery.isLoading ? (
                <GlassCard>
                    <p className="text-sm text-slate-500 dark:text-slate-400">{t("logsPage.empty")}</p>
                </GlassCard>
            ) : null}

            <div className="space-y-4">
                {entries.map((entry) => {
                    const canEdit = isAdmin && entry.id > 0;
                    return (
                        <GlassCard key={`${entry.published_at}-${entry.title}`}>
                            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                                <div className="sm:w-48">
                                    <p className="text-xs uppercase tracking-wide text-slate-400 dark:text-slate-500">
                                        {entry.published_at.slice(0, 10)}
                                    </p>
                                    <span className="mt-2 inline-flex items-center rounded-full bg-primary/10 px-3 py-1 text-xs font-medium text-primary dark:bg-primary/20">
                                        {entry.badge}
                                    </span>
                                </div>
                                <div className="flex-1 space-y-3">
                                    <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                                        <div>
                                            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                                {entry.title}
                                            </h2>
                                            {entry.summary ? (
                                                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                                                    {entry.summary}
                                                </p>
                                            ) : null}
                                        </div>
                                        {canEdit ? (
                                            <div className="flex gap-2">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => {
                                                        dispatch({ type: "load", entry });
                                                        setAutoTranslate(false);
                                                        setTranslateTargets(entry.locale === "zh-CN" ? ["en"] : ["zh-CN"]);
                                                        setTranslationModelKey("");
                                                    }}
                                                >
                                                    {t("logsPage.admin.edit")}
                                                </Button>
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    className={cn(
                                                        "border-rose-300 text-rose-500 hover:bg-rose-50 dark:border-rose-400/60 dark:text-rose-300 dark:hover:bg-rose-500/10",
                                                        isDeleting === entry.id ? "opacity-60" : ""
                                                    )}
                                                    disabled={deleteMutation.isPending && isDeleting === entry.id}
                                                    onClick={() => handleDelete(entry.id)}
                                                >
                                                    {t("logsPage.admin.delete")}
                                                </Button>
                                            </div>
                                        ) : null}
                                    </div>
                                    {entry.items.length > 0 ? (
                                        <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-300">
                                            {entry.items.map((item) => (
                                                <li
                                                    key={item}
                                                    className="flex items-start gap-2 rounded-lg bg-white/60 px-3 py-2 dark:bg-slate-800/60"
                                                >
                                                    <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                                                    <span>{item}</span>
                                                </li>
                                            ))}
                                        </ul>
                                    ) : null}
                                </div>
                            </div>
                        </GlassCard>
                    );
                })}
            </div>
        </div>
    );
}
