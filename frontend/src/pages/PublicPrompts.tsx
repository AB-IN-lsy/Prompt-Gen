/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-20 02:12:17
 * @FilePath: \electron-go-app\frontend\src\pages\PublicPrompts.tsx
 * @LastEditTime: 2025-10-20 02:12:17
 */
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Download,
  LoaderCircle,
  Sparkles,
  Clock3,
  Tags,
  CircleCheck,
  AlertCircle,
  Copy,
  X,
} from "lucide-react";
import { GlassCard } from "../components/ui/glass-card";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { Badge } from "../components/ui/badge";
import { MagneticButton } from "../components/ui/magnetic-button";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import {
  deletePublicPrompt,
  downloadPublicPrompt,
  fetchPublicPromptDetail,
  fetchPublicPrompts,
  type PublicPromptDownloadResult,
  type PublicPromptDetail,
  type PublicPromptListResponse,
  type PublicPromptListItem,
} from "../lib/api";
import { cn } from "../lib/utils";
import { useAuth } from "../hooks/useAuth";
import { isLocalMode } from "../lib/runtimeMode";
import { Button } from "../components/ui/button";
import { PUBLIC_PROMPT_LIST_PAGE_SIZE } from "../config/prompt";

type StatusFilter = "all" | "approved" | "pending" | "rejected";

const formatDateTime = (value?: string | null, locale?: string) => {
  if (!value) {
    return { date: "—", time: "" };
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return { date: value, time: "" };
  }
  const dateFormatter = new Intl.DateTimeFormat(locale ?? undefined, {
    dateStyle: "medium",
  } as Intl.DateTimeFormatOptions);
  const timeFormatter = new Intl.DateTimeFormat(locale ?? undefined, {
    timeStyle: "short",
  } as Intl.DateTimeFormatOptions);
  return {
    date: dateFormatter.format(date),
    time: timeFormatter.format(date),
  };
};

const ensureUniqueIntegers = (values: (number | null | undefined)[]) =>
  values.filter((item, index, arr) => item != null && arr.indexOf(item) === index);

export default function PublicPromptsPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchParamsString = searchParams.toString();
  const queryFromUrl = searchParams.get("q") ?? "";
  const queryClient = useQueryClient();
  const profile = useAuth((state) => state.profile);
  const isAdmin = Boolean(profile?.user?.is_admin);
  const offlineMode = isLocalMode();

  const [page, setPage] = useState(1);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchInput, setSearchInput] = useState(queryFromUrl);
  const [debouncedSearch, setDebouncedSearch] = useState(queryFromUrl);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>(
    isAdmin ? "all" : "approved",
  );
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);

  useEffect(() => {
    if (selectedId == null) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [selectedId]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedSearch(searchInput.trim());
      setPage(1);
    }, 280);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    let changed = false;
    setSearchInput((prev) => {
      if (prev === queryFromUrl) {
        return prev;
      }
      changed = true;
      return queryFromUrl;
    });
    setDebouncedSearch((prev) => {
      if (prev === queryFromUrl) {
        return prev;
      }
      changed = true;
      return queryFromUrl;
    });
    if (changed) {
      setPage(1);
    }
  }, [queryFromUrl]);

  useEffect(() => {
    if (debouncedSearch === queryFromUrl) {
      return;
    }
    const nextParams = new URLSearchParams(searchParamsString);
    if (debouncedSearch) {
      nextParams.set("q", debouncedSearch);
    } else {
      nextParams.delete("q");
    }
    setSearchParams(nextParams, { replace: true });
  }, [debouncedSearch, queryFromUrl, searchParamsString, setSearchParams]);

  const listQuery = useQuery<PublicPromptListResponse>({
    queryKey: [
      "public-prompts",
      {
        page,
        search: debouncedSearch,
        status: statusFilter,
        admin: isAdmin,
      },
    ],
    queryFn: () =>
      fetchPublicPrompts({
        page,
        pageSize: PUBLIC_PROMPT_LIST_PAGE_SIZE,
        query: debouncedSearch || undefined,
        status: statusFilter === "all" ? undefined : statusFilter,
      }),
    placeholderData: (previous) => previous,
  });

  const items: PublicPromptListItem[] = listQuery.data?.items ?? [];
  const listMeta = listQuery.data?.meta;
  const totalPages = listMeta?.total_pages ?? 1;
  const totalItems = listMeta?.total_items ?? items.length;
  const currentCount =
    listMeta?.current_count ??
    Math.min(items.length, PUBLIC_PROMPT_LIST_PAGE_SIZE);

  const detailQuery = useQuery<PublicPromptDetail>({
    queryKey: ["public-prompts", "detail", selectedId],
    queryFn: () => fetchPublicPromptDetail(selectedId ?? 0),
    enabled: selectedId != null,
  });

  const downloadMutation = useMutation<
    PublicPromptDownloadResult,
    unknown,
    number
  >({
    mutationFn: (id: number) => downloadPublicPrompt(id),
    onSuccess: (result, id) => {
      toast.success(t("publicPrompts.downloadSuccess"));
      if (result.promptId) {
        const cachedIds =
          queryClient.getQueryData<number[]>(["public-prompts", "downloaded"]) ?? [];
        queryClient.setQueryData(
          ["public-prompts", "downloaded"],
          ensureUniqueIntegers([...cachedIds, result.promptId]),
        );
      }
      void queryClient.invalidateQueries({ queryKey: ["my-prompts"] });
      if (selectedId === id) {
        setSelectedId(null);
      }
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error ? error.message : t("errors.generic");
      toast.error(message);
    },
  });

  const statusOptions = useMemo(() => {
    if (isAdmin) {
      return ["all", "approved", "pending", "rejected"] as StatusFilter[];
    }
    return ["approved", "pending", "rejected"] as StatusFilter[];
  }, [isAdmin]);

  const selectedDetail: PublicPromptDetail | undefined = detailQuery.data;
  const isDownloadingSelected =
    downloadMutation.isPending && downloadMutation.variables === selectedDetail?.id;
  const deleteMutation = useMutation<void, unknown, number>({
    mutationFn: (id: number) => deletePublicPrompt(id),
    onSuccess: (_, id) => {
      toast.success(t("publicPrompts.deleteSuccess"));
      const cachedIds =
        queryClient.getQueryData<number[]>(["public-prompts", "downloaded"]) ?? [];
      if (cachedIds.length > 0) {
        queryClient.setQueryData(
          ["public-prompts", "downloaded"],
          cachedIds.filter((item) => item !== id),
        );
      }
      void queryClient.invalidateQueries({ queryKey: ["public-prompts"] });
      void queryClient.invalidateQueries({ queryKey: ["public-prompts", "detail", id] });
      if (selectedId === id) {
        setSelectedId(null);
      }
      setConfirmDeleteId(null);
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error ? error.message : t("errors.generic");
      toast.error(message);
    },
  });

  const handleCloseDetail = () => {
    if (downloadMutation.isPending || deleteMutation.isPending) {
      return;
    }
    setSelectedId(null);
    setConfirmDeleteId(null);
  };
  const isDeletingSelected =
    deleteMutation.isPending && deleteMutation.variables === selectedDetail?.id;

  const handleStatusChange = (value: StatusFilter) => {
    setStatusFilter(value);
    setPage(1);
  };

  const handleDownload = (id: number) => {
    downloadMutation.mutate(id);
  };

  const allowDelete = isAdmin || offlineMode;

  const handleCopyBody = async () => {
    if (!selectedDetail?.body) {
      toast.error(t("publicPrompts.copyBodyEmpty"));
      return;
    }
    const textToCopy = selectedDetail.body;
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(textToCopy);
      } else if (typeof document !== "undefined") {
        const textarea = document.createElement("textarea");
        textarea.value = textToCopy;
        textarea.setAttribute("readonly", "");
        textarea.style.position = "absolute";
        textarea.style.left = "-9999px";
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand("copy");
        document.body.removeChild(textarea);
      } else {
        throw new Error("clipboard unsupported");
      }
      toast.success(t("publicPrompts.copyBodySuccess"));
    } catch {
      toast.error(t("publicPrompts.copyBodyFailure"));
    }
  };

  const statusBadge = (status: string) => {
    const normalized = status.toLowerCase();
    if (normalized === "approved") {
      return {
        label: t("publicPrompts.status.approved"),
        className: "bg-emerald-100 text-emerald-600 border-emerald-200 dark:bg-emerald-400/10 dark:text-emerald-300 dark:border-emerald-500/30",
      };
    }
    if (normalized === "pending") {
      return {
        label: t("publicPrompts.status.pending"),
        className: "bg-amber-100 text-amber-600 border-amber-200 dark:bg-amber-400/10 dark:text-amber-300 dark:border-amber-500/30",
      };
    }
    if (normalized === "rejected") {
      return {
        label: t("publicPrompts.status.rejected"),
        className: "bg-rose-100 text-rose-600 border-rose-200 dark:bg-rose-400/10 dark:text-rose-300 dark:border-rose-500/30",
      };
    }
    return {
      label: status,
      className: "bg-slate-200 text-slate-600 border-slate-300 dark:bg-slate-700 dark:text-slate-200 dark:border-slate-600",
    };
  };

  const isLoadingList = listQuery.isLoading || listQuery.isFetching;
  const isLoadingDetail = detailQuery.isLoading;

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between flex-wrap gap-4">
            <div>
              <span className="text-xs font-medium uppercase tracking-[0.3em] text-slate-400">
                {t("publicPrompts.eyebrow")}
              </span>
              <h1 className="mt-2 text-2xl font-semibold text-slate-900 dark:text-white">
                {t("publicPrompts.title")}
              </h1>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                {t("publicPrompts.subtitle")}
              </p>
            </div>
            <div className="flex items-center gap-3">
              <Badge className="border-transparent bg-primary/10 text-primary dark:bg-primary/20">
                {t("publicPrompts.total", { count: totalItems })}
              </Badge>
              {offlineMode ? (
                <Badge variant="outline" className="border-slate-300 text-slate-500 dark:border-slate-600 dark:text-slate-400">
                  {t("publicPrompts.offlineHint")}
                </Badge>
              ) : null}
            </div>
          </div>
          <SpotlightSearch
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder={t("publicPrompts.searchPlaceholder")}
            aria-label={t("publicPrompts.searchPlaceholder")}
          />
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="inline-flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
            <Tags className="h-4 w-4" />
            {t("publicPrompts.filters.title")}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {statusOptions.map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => handleStatusChange(item)}
                className={cn(
                  "rounded-full border px-3 py-1 text-xs font-medium transition-colors",
                  statusFilter === item
                    ? "border-primary/40 bg-primary/10 text-primary dark:border-primary/30 dark:bg-primary/20"
                    : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30",
                )}
              >
                {t(`publicPrompts.filters.${item}`)}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        {isLoadingList && items.length === 0
          ? Array.from({ length: PUBLIC_PROMPT_LIST_PAGE_SIZE }).map((_, index) => (
              <GlassCard
                key={`skeleton-${index}`}
                className="animate-pulse border-dashed border-slate-200 bg-white/60 dark:border-slate-800/60 dark:bg-slate-900/50"
              >
                <div className="h-4 w-32 rounded bg-slate-200 dark:bg-slate-700" />
                <div className="mt-4 h-3 w-full rounded bg-slate-200 dark:bg-slate-700" />
                <div className="mt-2 h-3 w-3/4 rounded bg-slate-200 dark:bg-slate-700" />
                <div className="mt-6 flex gap-2">
                  <div className="h-6 w-16 rounded-full bg-slate-200 dark:bg-slate-700" />
                  <div className="h-6 w-12 rounded-full bg-slate-200 dark:bg-slate-700" />
                </div>
              </GlassCard>
            ))
          : null}

        {!isLoadingList && items.length === 0 ? (
          <GlassCard className="md:col-span-2 xl:col-span-3 border-dashed border-slate-200 bg-white/70 text-center dark:border-slate-800/60 dark:bg-slate-900/60">
            <Sparkles className="mx-auto h-8 w-8 text-primary" />
            <p className="mt-3 text-sm text-slate-500 dark:text-slate-400">
              {t("publicPrompts.empty")}
            </p>
          </GlassCard>
        ) : null}

        {items.map((item) => {
          const detailActive = selectedId === item.id;
          const { label, className } = statusBadge(item.status);
          return (
            <GlassCard
              key={item.id}
              className={cn(
                "group relative flex flex-col gap-4 border border-white/60 bg-white/70 transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-[0_25px_45px_-20px_rgba(59,130,246,0.35)] dark:border-slate-800/60 dark:bg-slate-900/60 dark:hover:border-primary/30",
                detailActive ? "border-primary/40 shadow-[0_25px_45px_-20px_rgba(59,130,246,0.45)] dark:border-primary/50" : "",
              )}
              onClick={() => setSelectedId((prev) => (prev === item.id ? null : item.id))}
            >
              <div className="flex items-start justify-between gap-3">
              <div className="flex flex-col gap-1">
                <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100">
                  {item.title || item.topic}
                </h3>
                <span className="text-xs uppercase tracking-[0.25em] text-slate-400">
                  {item.language.toUpperCase()}
                </span>
              </div>
              <Badge className={className}>{label}</Badge>
            </div>
            <p className="line-clamp-3 text-sm text-slate-600 dark:text-slate-400">
              {item.summary || t("publicPrompts.noSummary")}
            </p>
            {item.status === "rejected" && item.review_reason ? (
              <p className="text-xs text-amber-600 dark:text-amber-300">
                {t("publicPrompts.reviewReasonBadge", {
                  reason: item.review_reason,
                })}
              </p>
            ) : null}
              <div className="flex flex-wrap items-center gap-2">
                {item.tags.slice(0, 4).map((tag) => (
                  <Badge key={`${item.id}-${tag}`} variant="outline" className="border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300">
                    #{tag}
                  </Badge>
                ))}
                {item.tags.length > 4 ? (
                  <span className="text-xs text-slate-400 dark:text-slate-500">
                    +{item.tags.length - 4}
                  </span>
                ) : null}
              </div>
              <div className="mt-auto flex items-center justify-between text-xs text-slate-400 dark:text-slate-500">
                <div className="flex items-center gap-2">
                  <Clock3 className="h-4 w-4" />
                  <span>{formatDateTime(item.updated_at, i18n.language).date}</span>
                </div>
                <div className="flex items-center gap-1 text-slate-500 dark:text-slate-400">
                  <Download className="h-4 w-4" />
                  <span>{item.download_count}</span>
                </div>
              </div>
            </GlassCard>
          );
        })}
      </div>

      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-col gap-1 text-xs text-slate-400 dark:text-slate-500 md:flex-row md:items-center md:gap-4">
          <span>
            {t("publicPrompts.paginationInfo", {
              page,
              totalPages,
            })}
          </span>
          <span>
            {t("publicPrompts.paginationCount", {
              count: currentCount,
            })}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-500 transition hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30"
            onClick={() => setPage((prev) => Math.max(prev - 1, 1))}
            disabled={page <= 1}
          >
            {t("publicPrompts.prevPage")}
          </button>
          <button
            type="button"
            className="rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-500 transition hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30"
            onClick={() => setPage((prev) => Math.min(prev + 1, totalPages))}
            disabled={page >= totalPages}
          >
            {t("publicPrompts.nextPage")}
          </button>
        </div>
      </div>

      {selectedId && selectedDetail ? (
        (() => {
          const statusMeta = statusBadge(selectedDetail.status);
          return (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 px-4 py-8 backdrop-blur-sm"
          onClick={handleCloseDetail}
        >
          <GlassCard
            className="relative w-full max-w-4xl overflow-hidden border-primary/40 bg-white/95 p-0 shadow-[0_45px_75px_-35px_rgba(59,130,246,0.7)] dark:border-primary/50 dark:bg-slate-900/95"
            onClick={(event) => event.stopPropagation()}
          >
            <button
              type="button"
              className="absolute right-5 top-5 inline-flex h-12 w-12 items-center justify-center rounded-full bg-white/95 text-primary shadow-[0_18px_45px_-20px_rgba(59,130,246,0.55)] ring-1 ring-slate-200/70 backdrop-blur-md transition hover:scale-105 hover:bg-white hover:ring-primary/40 dark:bg-slate-900/90 dark:text-primary dark:ring-slate-700/60"
              onClick={handleCloseDetail}
              aria-label={t("common.close")}
            >
              <X className="h-8 w-8" strokeWidth={2.4} />
            </button>
            <div className="flex max-h-[80vh] flex-col overflow-y-auto">
              <div className="border-b border-white/70 bg-white/90 px-6 pb-5 pt-7 dark:border-slate-800/60 dark:bg-slate-900/85">
                <div className="flex flex-col gap-4">
                  <div>
                    <span className="text-xs font-semibold uppercase tracking-[0.35em] text-slate-400">
                      {t("publicPrompts.detailHeader.eyebrow")}
                    </span>
                    <h2 className="mt-2 text-2xl font-semibold leading-tight text-slate-900 dark:text-white">
                      {selectedDetail.title || selectedDetail.topic}
                    </h2>
                    <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                      {selectedDetail.summary || t("publicPrompts.detailHeader.subtitle")}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400 dark:text-slate-500">
                    <CircleCheck className="h-4 w-4 text-emerald-500" />
                    <span>
                      {t("publicPrompts.detailMeta.model", {
                        model: selectedDetail.model,
                      })}
                    </span>
                    <span>·</span>
                    <span>
                      {t("publicPrompts.detailMeta.updatedAt", {
                        date: formatDateTime(selectedDetail.updated_at, i18n.language).date,
                        time: formatDateTime(selectedDetail.updated_at, i18n.language).time,
                      })}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-3 pt-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge className={statusMeta.className}>
                        {statusMeta.label}
                      </Badge>
                      <Badge variant="outline" className="border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300">
                        {selectedDetail.language.toUpperCase()}
                      </Badge>
                    </div>
                    <div className="ml-auto flex flex-wrap items-center gap-2">
                      <MagneticButton
                        type="button"
                        className="h-10 whitespace-nowrap rounded-full bg-primary/90 px-4 text-white hover:bg-primary focus-visible:ring-primary/60 dark:bg-primary/80"
                        disabled={isDownloadingSelected || isDeletingSelected}
                        onClick={() => handleDownload(selectedDetail.id)}
                      >
                        {isDownloadingSelected ? (
                          <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                          <Download className="mr-2 h-4 w-4" />
                        )}
                        {t("publicPrompts.downloadShort")}
                      </MagneticButton>
                      {allowDelete ? (
                        <Button
                          type="button"
                          variant="outline"
                          className="h-10 rounded-full border-rose-300 px-4 text-rose-600 hover:bg-rose-50 dark:border-rose-500/40 dark:text-rose-300 dark:hover:bg-rose-500/10"
                          disabled={isDeletingSelected || isDownloadingSelected}
                          onClick={() => setConfirmDeleteId(selectedDetail.id)}
                        >
                          {isDeletingSelected ? (
                            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                          ) : (
                            <AlertCircle className="mr-2 h-4 w-4" />
                          )}
                          {t("publicPrompts.deleteShort")}
                        </Button>
                      ) : null}
                    </div>
                  </div>
                </div>
                {selectedDetail.review_reason ? (
                  <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-700 dark:border-amber-400/40 dark:bg-amber-400/10 dark:text-amber-200">
                    <p className="font-semibold uppercase tracking-[0.3em]">
                      {t("publicPrompts.reviewReasonTitle")}
                    </p>
                    <p className="mt-1 whitespace-pre-wrap leading-relaxed">
                      {selectedDetail.review_reason}
                    </p>
                  </div>
                ) : null}
              </div>

              <div className="flex flex-col gap-5 px-6 pb-6 pt-5">
              <div className="grid gap-4 md:grid-cols-2">
                <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                  <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                    <Sparkles className="h-4 w-4" />
                    {t("publicPrompts.keywords.positive")}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedDetail.positive_keywords.length > 0 ? (
                      selectedDetail.positive_keywords.map((keyword, idx) => (
                        <Badge
                          key={`positive-${keyword.word}-${idx}`}
                          variant="outline"
                          className="border-emerald-300/60 text-emerald-600 dark:border-emerald-400/30 dark:text-emerald-300"
                        >
                          {keyword.word}
                        </Badge>
                      ))
                    ) : (
                      <span className="text-xs text-slate-400 dark:text-slate-500">
                        {t("publicPrompts.keywords.empty")}
                      </span>
                    )}
                  </div>
                </GlassCard>

                <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                  <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                    <Tags className="h-4 w-4" />
                    {t("publicPrompts.keywords.negative")}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedDetail.negative_keywords.length > 0 ? (
                      selectedDetail.negative_keywords.map((keyword, idx) => (
                        <Badge
                          key={`negative-${keyword.word}-${idx}`}
                          variant="outline"
                          className="border-rose-300/60 text-rose-600 dark:border-rose-400/30 dark:text-rose-300"
                        >
                          {keyword.word}
                        </Badge>
                      ))
                    ) : (
                      <span className="text-xs text-slate-400 dark:text-slate-500">
                        {t("publicPrompts.keywords.empty")}
                      </span>
                    )}
                  </div>
                </GlassCard>
              </div>

              <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                  <Clock3 className="h-4 w-4" />
                  {t("publicPrompts.instructions")}
                </div>
                <div className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-slate-600 dark:text-slate-300">
                  {selectedDetail.instructions
                    ? selectedDetail.instructions
                    : t("publicPrompts.noInstructions")}
                </div>
              </GlassCard>

              <GlassCard className="bg-white/85 dark:bg-slate-900/70 md:col-span-2">
                <div className="flex items-center justify-between text-xs uppercase tracking-[0.25em] text-slate-400">
                  <span>{t("publicPrompts.body")}</span>
                  <button
                    type="button"
                    className="inline-flex items-center gap-2 rounded-full bg-primary/5 px-3 py-1 text-[0.75rem] font-medium text-primary transition hover:bg-primary/10 hover:text-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:bg-primary/10 dark:text-primary-200 dark:hover:bg-primary/20"
                    onClick={() => {
                      void handleCopyBody();
                    }}
                  >
                    <Copy className="h-4 w-4" />
                    {t("publicPrompts.copyBody")}
                  </button>
                </div>
                <pre className="mt-3 max-h-[40vh] overflow-y-auto whitespace-pre-wrap break-words rounded-2xl bg-slate-900/5 p-4 text-sm leading-relaxed text-slate-600 dark:bg-slate-900/80 dark:text-slate-200">
                  {selectedDetail.body}
                </pre>
              </GlassCard>
            </div>
            </div>
          </GlassCard>
        </div>
          );
        })()
      ) : null}

      {selectedId && isLoadingDetail ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4 py-8 backdrop-blur-sm">
          <GlassCard className="flex items-center gap-3 border-dashed border-slate-200 bg-white/90 text-sm text-slate-500 dark:border-slate-800/60 dark:bg-slate-900/80 dark:text-slate-300">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            {t("publicPrompts.loadingDetail")}
          </GlassCard>
        </div>
      ) : null}

      <ConfirmDialog
        open={confirmDeleteId != null}
        title={t("publicPrompts.deleteDialogTitle")}
        description={t("publicPrompts.deleteConfirm")}
        confirmLabel={t("publicPrompts.deleteAction")}
        cancelLabel={t("common.cancel")}
        loading={deleteMutation.isPending}
        onCancel={() => {
          if (!deleteMutation.isPending) {
            setConfirmDeleteId(null);
          }
        }}
        onConfirm={() => {
          if (confirmDeleteId != null) {
            deleteMutation.mutate(confirmDeleteId);
          }
        }}
      />
    </div>
  );
}
