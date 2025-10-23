/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-20 02:12:17
 * @FilePath: \electron-go-app\frontend\src\pages\PublicPrompts.tsx
 * @LastEditTime: 2025-10-20 02:12:17
 */
import { useEffect, useMemo, useState } from "react";
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
} from "lucide-react";
import { GlassCard } from "../components/ui/glass-card";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { Badge } from "../components/ui/badge";
import { MagneticButton } from "../components/ui/magnetic-button";
import {
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

type StatusFilter = "all" | "approved" | "pending" | "rejected";

const PAGE_SIZE = 9;

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
  const queryClient = useQueryClient();
  const profile = useAuth((state) => state.profile);
  const isAdmin = Boolean(profile?.user?.is_admin);
  const offlineMode = isLocalMode();

  const [page, setPage] = useState(1);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>(
    isAdmin ? "all" : "approved",
  );

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedSearch(searchInput.trim());
      setPage(1);
    }, 280);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

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
        pageSize: PAGE_SIZE,
        query: debouncedSearch || undefined,
        status: statusFilter === "all" ? undefined : statusFilter,
      }),
    placeholderData: (previous) => previous,
  });

  const items: PublicPromptListItem[] = listQuery.data?.items ?? [];
  const listMeta = listQuery.data?.meta;
  const totalPages = listMeta?.total_pages ?? 1;
  const totalItems = listMeta?.total_items ?? items.length;

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

  const handleStatusChange = (value: StatusFilter) => {
    setStatusFilter(value);
    setPage(1);
  };

  const handleDownload = (id: number) => {
    downloadMutation.mutate(id);
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
          ? Array.from({ length: PAGE_SIZE }).map((_, index) => (
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
        <div className="text-xs text-slate-400 dark:text-slate-500">
          {t("publicPrompts.paginationInfo", {
            page,
            totalPages,
          })}
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
        <GlassCard className="border-primary/40 bg-white/80 shadow-[0_35px_65px_-40px_rgba(59,130,246,0.65)] dark:border-primary/50 dark:bg-slate-900/70">
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div>
                <h2 className="text-xl font-semibold text-slate-900 dark:text-white">
                  {selectedDetail.title || selectedDetail.topic}
                </h2>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  {selectedDetail.summary || t("publicPrompts.noSummary")}
                </p>
                {selectedDetail.review_reason ? (
                  <div className="mt-2 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-700 dark:border-amber-400/40 dark:bg-amber-400/10 dark:text-amber-200">
                    <p className="font-semibold uppercase tracking-[0.3em]">
                      {t("publicPrompts.reviewReasonTitle")}
                    </p>
                    <p className="mt-1 whitespace-pre-wrap">
                      {selectedDetail.review_reason}
                    </p>
                  </div>
                ) : null}
              </div>
              <MagneticButton
                type="button"
                className="bg-primary/90 text-white hover:bg-primary focus-visible:ring-primary/60 dark:bg-primary/80"
                disabled={isDownloadingSelected}
                onClick={() => handleDownload(selectedDetail.id)}
              >
                {isDownloadingSelected ? (
                  <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Download className="mr-2 h-4 w-4" />
                )}
                {t("publicPrompts.downloadAction")}
              </MagneticButton>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <GlassCard className="bg-white/70 dark:bg-slate-900/60">
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
              <GlassCard className="bg-white/70 dark:bg-slate-900/60">
                <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                  <AlertCircle className="h-4 w-4" />
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

            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm leading-6 text-slate-700 shadow-inner dark:border-slate-800/60 dark:bg-slate-900/70 dark:text-slate-200">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                  {t("publicPrompts.instructions")}
                </h3>
                <p className="whitespace-pre-wrap">
                  {selectedDetail.instructions || t("publicPrompts.noInstructions")}
                </p>
              </div>
              <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm leading-6 text-slate-700 shadow-inner dark:border-slate-800/60 dark:bg-slate-900/70 dark:text-slate-200">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                  {t("publicPrompts.body")}
                </h3>
                <p className="max-h-60 overflow-y-auto whitespace-pre-wrap pr-2">
                  {selectedDetail.body}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-3 text-xs text-slate-400 dark:text-slate-500">
              <div className="flex items-center gap-2">
                <CircleCheck className="h-4 w-4" />
                <span>
                  {t("publicPrompts.detailMeta.model", { model: selectedDetail.model })}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <Clock3 className="h-4 w-4" />
                <span>
                  {t("publicPrompts.detailMeta.updatedAt", {
                    date: formatDateTime(selectedDetail.updated_at, i18n.language).date,
                    time: formatDateTime(selectedDetail.updated_at, i18n.language).time,
                  })}
                </span>
              </div>
            </div>
          </div>
        </GlassCard>
      ) : null}

      {selectedId && isLoadingDetail ? (
        <GlassCard className="border-dashed border-slate-200 bg-white/70 text-sm text-slate-500 dark:border-slate-800/60 dark:bg-slate-900/60 dark:text-slate-400">
          <div className="flex items-center gap-2">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            {t("publicPrompts.loadingDetail")}
          </div>
        </GlassCard>
      ) : null}
    </div>
  );
}
