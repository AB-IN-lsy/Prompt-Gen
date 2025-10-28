import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertCircle,
  CheckCircle2,
  Clock3,
  LoaderCircle,
  Sparkles,
  ShieldBan,
  Tags,
  Trash2,
} from "lucide-react";

import { PageHeader } from "../components/layout/PageHeader";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { Textarea } from "../components/ui/textarea";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import {
  deletePublicPrompt,
  fetchPublicPromptDetail,
  fetchPublicPrompts,
  reviewPublicPrompt,
  type PublicPromptDetail,
  type PublicPromptListItem,
} from "../lib/api";
import { cn } from "../lib/utils";
import { useAuth } from "../hooks/useAuth";

type StatusFilter = "pending" | "approved" | "rejected" | "all";

const reviewPageSizeRaw =
  import.meta.env.VITE_PUBLIC_PROMPT_REVIEW_PAGE_SIZE ?? "12";
const parsedReviewPageSize = Number.parseInt(reviewPageSizeRaw, 10);
const REVIEW_PAGE_SIZE = Number.isNaN(parsedReviewPageSize)
  ? 12
  : parsedReviewPageSize;
const rejectRowsRaw =
  import.meta.env.VITE_PUBLIC_PROMPT_REVIEW_REASON_ROWS ?? "3";
const parsedRejectRows = Number.parseInt(rejectRowsRaw, 10);
const REVIEW_REASON_ROWS = Number.isNaN(parsedRejectRows)
  ? 3
  : parsedRejectRows;

const statusOptions: StatusFilter[] = ["pending", "approved", "rejected", "all"];

export default function AdminPublicPromptsPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const profile = useAuth((state) => state.profile);
  const isAdmin = Boolean(profile?.user?.is_admin);

  const [page, setPage] = useState(1);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("pending");
  const [rejectReason, setRejectReason] = useState("");
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedSearch(searchInput.trim());
      setPage(1);
    }, 280);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    setRejectReason("");
  }, [selectedId]);

  const listQuery = useQuery({
    queryKey: [
      "admin-public-prompts",
      statusFilter,
      page,
      debouncedSearch,
    ],
    enabled: isAdmin,
    queryFn: () =>
      fetchPublicPrompts({
        status: statusFilter === "all" ? undefined : statusFilter,
        page,
        pageSize: REVIEW_PAGE_SIZE,
        query: debouncedSearch || undefined,
      }),
    placeholderData: (previous) => previous,
  });

  const items: PublicPromptListItem[] = listQuery.data?.items ?? [];
  const meta = listQuery.data?.meta;
  const totalPages = meta?.total_pages ?? 1;
  const totalItems = meta?.total_items ?? items.length;

  useEffect(() => {
    if (items.length === 0) {
      setSelectedId(null);
      return;
    }
    if (!selectedId) {
      setSelectedId(items[0]?.id ?? null);
    }
  }, [items, selectedId]);

  const detailQuery = useQuery<PublicPromptDetail>({
    queryKey: ["admin-public-prompts", "detail", selectedId],
    enabled: isAdmin && selectedId != null,
    queryFn: () => fetchPublicPromptDetail(selectedId ?? 0),
  });

  const reviewMutation = useMutation<
    void,
    unknown,
    { id: number; status: "approved" | "rejected"; reason?: string }
  >({
    mutationFn: ({ id, status, reason }) =>
      reviewPublicPrompt(id, { status, reason }),
    onSuccess: (_, variables) => {
      if (variables.status === "approved") {
        toast.success(t("publicPromptReview.approveSuccess"));
      } else {
        toast.success(t("publicPromptReview.rejectSuccess"));
      }
      setRejectReason("");
      void queryClient.invalidateQueries({ queryKey: ["admin-public-prompts"] });
      void queryClient.invalidateQueries({
        queryKey: ["public-prompts"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["admin-public-prompts", "detail", variables.id],
      });
      if (statusFilter === "pending") {
        setSelectedId(null);
      }
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : t("publicPromptReview.actionError");
      toast.error(message);
    },
  });

  const deleteMutation = useMutation<void, unknown, number>({
    mutationFn: (id: number) => deletePublicPrompt(id),
    onSuccess: (_, id) => {
      toast.success(t("publicPromptReview.deleteSuccess"));
      setRejectReason("");
      setSelectedId(null);
      void queryClient.invalidateQueries({ queryKey: ["admin-public-prompts"] });
      void queryClient.invalidateQueries({ queryKey: ["public-prompts"] });
      void queryClient.invalidateQueries({
        queryKey: ["admin-public-prompts", "detail", id],
      });
      setConfirmDeleteId(null);
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : t("publicPromptReview.deleteError");
      toast.error(message);
    },
  });

  const formatDateTime = useMemo(() => {
    return new Intl.DateTimeFormat(i18n.language, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  }, [i18n.language]);

  if (!isAdmin) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          eyebrow={t("publicPromptReview.eyebrow")}
          title={t("publicPromptReview.title")}
          description={t("publicPromptReview.subtitle")}
        />
        <GlassCard className="flex flex-1 flex-col items-center justify-center gap-3 text-slate-500">
          <ShieldBan className="h-6 w-6 text-rose-400" />
          <p className="text-sm">{t("publicPromptReview.noPermission")}</p>
        </GlassCard>
      </div>
    );
  }

  const handleApprove = () => {
    if (!selectedId || deleteMutation.isPending) {
      return;
    }
    reviewMutation.mutate({ id: selectedId, status: "approved" });
  };

  const handleReject = () => {
    if (!selectedId || deleteMutation.isPending) {
      return;
    }
    reviewMutation.mutate({
      id: selectedId,
      status: "rejected",
      reason: rejectReason.trim() || undefined,
    });
  };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow={t("publicPromptReview.eyebrow")}
        title={t("publicPromptReview.title")}
        description={t("publicPromptReview.subtitle")}
      />

      <div className="grid gap-6 lg:grid-cols-[360px,1fr]">
        <GlassCard className="flex flex-col gap-4">
          <div className="flex flex-col gap-3">
            <SpotlightSearch
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder={t("publicPromptReview.searchPlaceholder")}
            />
            <div className="flex flex-wrap items-center gap-2">
              {statusOptions.map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => {
                    setStatusFilter(option);
                    setPage(1);
                  }}
                  className={cn(
                    "rounded-full border px-3 py-1 text-xs font-medium transition",
                    statusFilter === option
                      ? "border-primary/40 bg-primary/10 text-primary dark:border-primary/40 dark:bg-primary/20 dark:text-primary-foreground"
                      : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400",
                  )}
                >
                  {t(`publicPromptReview.status.${option}`)}
                </button>
              ))}
            </div>
          </div>

          <div className="rounded-2xl border border-white/50 bg-white/60 p-3 dark:border-slate-800/60 dark:bg-slate-900/60">
            {listQuery.isLoading ? (
              <div className="flex items-center justify-center gap-2 py-10 text-sm text-slate-500 dark:text-slate-400">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                {t("common.loading")}
              </div>
            ) : items.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-2 py-10 text-sm text-slate-500 dark:text-slate-400">
                <AlertCircle className="h-5 w-5" />
                <span>{t("publicPromptReview.empty")}</span>
              </div>
            ) : (
              <ul className="flex flex-col gap-2">
                {items.map((item) => {
                  const isActive = selectedId === item.id;
                  return (
                    <li key={item.id}>
                      <button
                        type="button"
                        onClick={() =>
                          setSelectedId((prev) =>
                            prev === item.id ? prev : item.id,
                          )
                        }
                        className={cn(
                          "flex w-full flex-col gap-2 rounded-xl border px-3 py-3 text-left transition hover:border-primary/30 hover:bg-primary/5 dark:hover:border-primary/30 dark:hover:bg-primary/10",
                          isActive
                            ? "border-primary/40 bg-primary/10 text-primary dark:border-primary/40 dark:bg-primary/15 dark:text-primary-foreground"
                            : "border-transparent bg-white/80 text-slate-600 dark:bg-slate-900/70 dark:text-slate-300",
                        )}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-sm font-semibold">
                            {item.title || item.topic}
                          </span>
                          <Badge variant="outline" className="border-slate-200 text-xs text-slate-500 dark:border-slate-700 dark:text-slate-400">
                            {t(`publicPromptReview.status.${item.status}`)}
                          </Badge>
                        </div>
                        <p className="line-clamp-2 text-xs text-slate-500 dark:text-slate-400">
                          {item.summary || t("publicPromptReview.noSummary")}
                        </p>
                        <div className="flex items-center justify-between text-[11px] text-slate-400 dark:text-slate-500">
                          <span>
                            {formatDateTime.format(
                              item.updated_at ? new Date(item.updated_at) : new Date(),
                            )}
                          </span>
                          <span>{item.tags.slice(0, 3).join(", ")}</span>
                        </div>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>

          <div className="flex items-center justify-between text-xs text-slate-400 dark:text-slate-500">
            <span>
              {t("publicPromptReview.pagination", {
                page,
                totalPages,
                totalItems,
              })}
            </span>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setPage((prev) => Math.max(prev - 1, 1))}
                disabled={page <= 1}
              >
                {t("publicPromptReview.prevPage")}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setPage((prev) => Math.min(prev + 1, totalPages))}
                disabled={page >= totalPages}
              >
                {t("publicPromptReview.nextPage")}
              </Button>
            </div>
          </div>
        </GlassCard>

        <GlassCard className="flex flex-col gap-4">
          {selectedId == null ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-slate-500 dark:text-slate-400">
              <AlertCircle className="h-5 w-5" />
              <span>{t("publicPromptReview.noSelection")}</span>
            </div>
          ) : detailQuery.isLoading ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-slate-500 dark:text-slate-400">
              <LoaderCircle className="h-5 w-5 animate-spin" />
              <span>{t("common.loading")}</span>
            </div>
          ) : detailQuery.isError || !detailQuery.data ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-rose-500 dark:text-rose-400">
              <AlertCircle className="h-5 w-5" />
              <span>{t("publicPromptReview.detailError")}</span>
            </div>
          ) : (
            <PublicPromptDetailPanel
              detail={detailQuery.data}
              rejectReason={rejectReason}
              onRejectReasonChange={setRejectReason}
              onApprove={handleApprove}
              onReject={handleReject}
              onDelete={() => {
                if (detailQuery.data) {
                  setConfirmDeleteId(detailQuery.data.id);
                }
              }}
              isSubmitting={reviewMutation.isPending}
              isDeleting={
                deleteMutation.isPending &&
                deleteMutation.variables === detailQuery.data.id
              }
              formatDateTime={formatDateTime}
              t={t}
            />
          )}
        </GlassCard>
      </div>
      <ConfirmDialog
        open={confirmDeleteId != null}
        title={t("publicPromptReview.deleteDialogTitle")}
        description={t("publicPromptReview.deleteConfirm")}
        confirmLabel={t("publicPromptReview.deleteAction")}
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

function PublicPromptDetailPanel({
  detail,
  rejectReason,
  onRejectReasonChange,
  onApprove,
  onReject,
  onDelete,
  isSubmitting,
  isDeleting,
  formatDateTime,
  t,
}: {
  detail: PublicPromptDetail;
  rejectReason: string;
  onRejectReasonChange: (value: string) => void;
  onApprove: () => void;
  onReject: () => void;
  onDelete: () => void;
  isSubmitting: boolean;
  isDeleting: boolean;
  formatDateTime: Intl.DateTimeFormat;
  t: (key: string, options?: Record<string, unknown>) => string;
}): JSX.Element {
  const handleDeleteClick = () => {
    if (isSubmitting || isDeleting) {
      return;
    }
    onDelete();
  };

  const scoreValue = Number.isFinite(detail.quality_score)
    ? detail.quality_score.toFixed(1)
    : "0.0";
  const scoreLabel = t("publicPrompts.qualityScoreLabel", { score: scoreValue });

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900 dark:text-white">
          {detail.title || detail.topic}
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          {detail.summary || t("publicPromptReview.noSummary")}
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400 dark:text-slate-500">
        <div className="flex items-center gap-2">
          <Tags className="h-4 w-4" />
          <span>{detail.tags.join(", ") || t("publicPromptReview.noTags")}</span>
        </div>
        <div className="flex items-center gap-2">
          <Clock3 className="h-4 w-4" />
          <span>
            {formatDateTime.format(
              detail.updated_at ? new Date(detail.updated_at) : new Date(),
            )}
          </span>
        </div>
        <div className="flex items-center gap-2 text-primary/80 dark:text-primary/60">
          <Sparkles className="h-4 w-4" />
          <span>{scoreLabel}</span>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm dark:border-slate-800/60 dark:bg-slate-900/60">
          <p className="mb-2 text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
            {t("publicPromptReview.positiveKeywords")}
          </p>
          <div className="flex flex-wrap gap-2">
            {detail.positive_keywords.length === 0 ? (
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("publicPromptReview.noKeywords")}
              </span>
            ) : (
              detail.positive_keywords.map((keyword, index) => (
                <Badge
                  key={`pos-${keyword.word}-${index}`}
                  variant="outline"
                  className="border-emerald-300/60 text-emerald-600 dark:border-emerald-400/30 dark:text-emerald-300"
                >
                  {keyword.word}
                </Badge>
              ))
            )}
          </div>
        </div>
        <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm dark:border-slate-800/60 dark:bg-slate-900/60">
          <p className="mb-2 text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
            {t("publicPromptReview.negativeKeywords")}
          </p>
          <div className="flex flex-wrap gap-2">
            {detail.negative_keywords.length === 0 ? (
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("publicPromptReview.noKeywords")}
              </span>
            ) : (
              detail.negative_keywords.map((keyword, index) => (
                <Badge
                  key={`neg-${keyword.word}-${index}`}
                  variant="outline"
                  className="border-rose-300/60 text-rose-600 dark:border-rose-400/30 dark:text-rose-300"
                >
                  {keyword.word}
                </Badge>
              ))
            )}
          </div>
        </div>
      </div>

      <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm leading-6 text-slate-700 dark:border-slate-800/60 dark:bg-slate-900/60 dark:text-slate-200">
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
          {t("publicPromptReview.instructions")}
        </p>
        <p className="whitespace-pre-wrap">
          {detail.instructions || t("publicPromptReview.noSummary")}
        </p>
      </div>

      <div className="rounded-2xl border border-white/50 bg-white/70 p-4 text-sm leading-6 text-slate-700 dark:border-slate-800/60 dark:bg-slate-900/60 dark:text-slate-200">
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
          {t("publicPromptReview.body")}
        </p>
        <p className="whitespace-pre-wrap">{detail.body}</p>
      </div>

      <div className="space-y-2">
        <label
          htmlFor="public-prompt-review-reason"
          className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
        >
          {t("publicPromptReview.rejectReason")}
        </label>
        <Textarea
          id="public-prompt-review-reason"
          value={rejectReason}
          onChange={(event) => onRejectReasonChange(event.target.value)}
          placeholder={t("publicPromptReview.rejectPlaceholder")}
          rows={REVIEW_REASON_ROWS}
        />
        <p className="text-xs text-slate-400 dark:text-slate-500">
          {t("publicPromptReview.rejectHint")}
        </p>
      </div>

      <div className="mt-auto flex flex-wrap items-center justify-end gap-3">
        <Button
          type="button"
          variant="outline"
          onClick={handleDeleteClick}
          disabled={isSubmitting || isDeleting}
          className="border-rose-200 text-rose-500 transition-transform hover:-translate-y-0.5 hover:bg-rose-500/10 dark:border-rose-500/40 dark:text-rose-300 dark:hover:bg-rose-500/20"
        >
          {isDeleting ? (
            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Trash2 className="mr-2 h-4 w-4" />
          )}
          {t("publicPromptReview.deleteAction")}
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={onReject}
          disabled={isSubmitting || isDeleting}
          className="transition-transform hover:-translate-y-0.5"
        >
          {isSubmitting ? (
            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <AlertCircle className="mr-2 h-4 w-4" />
          )}
          {t("publicPromptReview.rejectAction")}
        </Button>
        <Button
          type="button"
          onClick={onApprove}
          disabled={isSubmitting || isDeleting}
          className="transition-transform hover:-translate-y-0.5"
        >
          {isSubmitting ? (
            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <CheckCircle2 className="mr-2 h-4 w-4" />
          )}
          {t("publicPromptReview.approveAction")}
        </Button>
      </div>
    </div>
  );
}
