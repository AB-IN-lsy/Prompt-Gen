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
  Heart,
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
  fetchPromptComments,
  createPromptComment,
  reviewPromptComment,
  likePublicPrompt,
  unlikePublicPrompt,
  type PublicPromptDownloadResult,
  type PublicPromptDetail,
  type PublicPromptLikeResult,
  type PublicPromptListResponse,
  type PublicPromptListItem,
  type PromptComment,
  type PromptCommentListResponse,
} from "../lib/api";
import { cn } from "../lib/utils";
import { useAuth } from "../hooks/useAuth";
import { isLocalMode } from "../lib/runtimeMode";
import { Button } from "../components/ui/button";
import { Textarea } from "../components/ui/textarea";
import { PROMPT_COMMENT_PAGE_SIZE, PUBLIC_PROMPT_LIST_PAGE_SIZE } from "../config/prompt";

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
  const [commentPage, setCommentPage] = useState(1);
  const [commentBody, setCommentBody] = useState("");
  const [replyDrafts, setReplyDrafts] = useState<Record<number, string>>({});
  const [replyTarget, setReplyTarget] = useState<number | null>(null);
  const [commentStatusFilter, setCommentStatusFilter] = useState<"approved" | "pending" | "rejected" | "all">("approved");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchInput, setSearchInput] = useState(queryFromUrl);
  const [debouncedSearch, setDebouncedSearch] = useState(queryFromUrl);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>(
    isAdmin ? "all" : "approved",
  );
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [rejectDialog, setRejectDialog] = useState<{ commentId: number } | null>(null);
  const [rejectNote, setRejectNote] = useState("");

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

  const commentStatusOptions = useMemo(() => {
    if (isAdmin) {
      return ["all", "approved", "pending", "rejected"] as ("approved" | "pending" | "rejected" | "all")[];
    }
    return ["approved"] as ("approved" | "pending" | "rejected" | "all")[];
  }, [isAdmin]);

  const selectedDetail: PublicPromptDetail | undefined = detailQuery.data;
  const isDownloadingSelected =
    downloadMutation.isPending && downloadMutation.variables === selectedDetail?.id;
  const sourcePromptId = selectedDetail?.source_prompt_id ?? null;

  useEffect(() => {
    setCommentPage(1);
    setCommentBody("");
    setReplyDrafts({});
    setReplyTarget(null);
    setCommentStatusFilter(isAdmin ? "all" : "approved");
  }, [sourcePromptId, isAdmin]);

  const commentQuery = useQuery<PromptCommentListResponse>({
    queryKey: ["prompt-comments", sourcePromptId, commentPage, commentStatusFilter],
    enabled: sourcePromptId != null,
    placeholderData: (previous) => previous,
    queryFn: () =>
      fetchPromptComments(sourcePromptId ?? 0, {
        page: commentPage,
        pageSize: PROMPT_COMMENT_PAGE_SIZE,
        status: commentStatusFilter === "all" ? undefined : commentStatusFilter,
      }),
  });

  const commentItems: PromptComment[] = commentQuery.data?.items ?? [];
  const commentMeta = commentQuery.data?.meta;
  const isLoadingComments = commentQuery.isLoading || commentQuery.isFetching;

  const commentError = commentQuery.error;

  const commentErrorMessage = commentError
    ? commentError instanceof Error
      ? commentError.message
      : t("comments.loadError")
    : null;

  const isCommentBodyEmpty = commentBody.trim().length === 0;

  const commentMutation = useMutation<PromptComment, unknown, { body: string; parentId?: number | null }>({
    mutationFn: async ({ body, parentId }) => {
      if (offlineMode) {
        throw new Error(t("comments.offlineDisabled"));
      }
      if (sourcePromptId == null) {
        throw new Error(t("comments.loadError"));
      }
      return createPromptComment(sourcePromptId, {
        body,
        parentId: typeof parentId === "number" && parentId > 0 ? parentId : null,
      });
    },
    onSuccess: (result, variables) => {
      if (result.status === "approved") {
        toast.success(t("comments.createSuccess"));
      } else if (result.status === "pending") {
        toast.success(t("comments.createPending"));
      } else {
        toast.success(t("comments.createSuccess"));
      }
      setCommentBody("");
      if (!variables.parentId) {
        // noop
      } else {
        setReplyDrafts((prev) => ({ ...prev, [variables.parentId!]: "" }));
        setReplyTarget(null);
      }
      void queryClient.invalidateQueries({ queryKey: ["prompt-comments", sourcePromptId] });
    },
    onError: (error: unknown) => {
      const message = error instanceof Error ? error.message : t("errors.generic");
      toast.error(message);
    },
  });

  const commentReviewMutation = useMutation<PromptComment, unknown, { commentId: number; status: "approved" | "rejected"; note?: string }>({
    mutationFn: ({ commentId, status, note }) => reviewPromptComment(commentId, { status, note }),
    onSuccess: () => {
      toast.success(t("comments.reviewSuccess"));
      setRejectDialog(null);
      setRejectNote("");
      void queryClient.invalidateQueries({ queryKey: ["prompt-comments", sourcePromptId] });
    },
    onError: (error: unknown) => {
      const message = error instanceof Error ? error.message : t("errors.generic");
      toast.error(message);
    },
  });

  const handleSubmitComment = () => {
    const trimmed = commentBody.trim();
    if (offlineMode) {
      toast.error(t("comments.offlineDisabled"));
      return;
    }
    if (!sourcePromptId) {
      toast.error(t("comments.loadError"));
      return;
    }
    if (trimmed === "") {
      toast.error(t("comments.emptyBody"));
      return;
    }
    commentMutation.mutate({ body: trimmed });
  };

  const handleReplyDraftChange = (commentId: number, value: string) => {
    setReplyDrafts((prev) => ({ ...prev, [commentId]: value }));
  };

  const handleSubmitReply = (commentId: number) => {
    const draft = (replyDrafts[commentId] ?? "").trim();
    if (offlineMode) {
      toast.error(t("comments.offlineDisabled"));
      return;
    }
    if (!sourcePromptId) {
      toast.error(t("comments.loadError"));
      return;
    }
    if (draft === "") {
      toast.error(t("comments.emptyBody"));
      return;
    }
    commentMutation.mutate({ body: draft, parentId: commentId });
  };

  const handleStartReply = (commentId: number) => {
    setReplyTarget(commentId);
    setReplyDrafts((prev) => ({ ...prev, [commentId]: prev[commentId] ?? "" }));
  };

  const handleCancelReply = () => {
    setReplyTarget(null);
  };

  const handleReviewComment = (commentId: number, nextStatus: "approved" | "rejected", existingNote = "") => {
    if (commentReviewMutation.isPending) {
      return;
    }
    if (nextStatus === "rejected") {
      setRejectDialog({ commentId });
      setRejectNote(existingNote ?? "");
      return;
    }
    commentReviewMutation.mutate({ commentId, status: nextStatus });
  };

  const handleCommentStatusChange = (value: "approved" | "pending" | "rejected" | "all") => {
    setCommentStatusFilter(value);
    setCommentPage(1);
  };

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

  const likeMutation = useMutation<PublicPromptLikeResult, unknown, { id: number; next: boolean }>({
    mutationFn: ({ id, next }) => (next ? likePublicPrompt(id) : unlikePublicPrompt(id)),
    onSuccess: (result, variables) => {
      queryClient.setQueriesData<PublicPromptListResponse>({ queryKey: ["public-prompts"] }, (previous) => {
        if (!previous || !Array.isArray(previous.items)) {
          return previous;
        }
        return {
          ...previous,
          items: previous.items.map((item) =>
            item.id === variables.id
              ? { ...item, is_liked: result.liked, like_count: result.like_count }
              : item,
          ),
        };
      });
      queryClient.setQueryData<PublicPromptDetail>(
        ["public-prompts", "detail", variables.id],
        (previous) => {
          if (!previous) {
            return previous;
          }
          return {
            ...previous,
            is_liked: result.liked,
            like_count: result.like_count,
          };
        },
      );
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error ? error.message : t("publicPrompts.likeError");
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

  const handleToggleLike = (id: number, liked: boolean) => {
    if (offlineMode) {
      toast.error(t("publicPrompts.likeOfflineDisabled"));
      return;
    }
    if (likeMutation.isPending && likeMutation.variables?.id === id) {
      return;
    }
    likeMutation.mutate({ id, next: !liked });
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

  const commentStatusBadge = (status: string) => {
    const normalized = status.toLowerCase();
    if (normalized === "pending") {
      return {
        label: t("comments.pendingBadge"),
        className: "bg-amber-100 text-amber-600 border-amber-200 dark:bg-amber-400/10 dark:text-amber-300 dark:border-amber-500/30",
      };
    }
    if (normalized === "rejected") {
      return {
        label: t("comments.rejectedBadge"),
        className: "bg-rose-100 text-rose-600 border-rose-200 dark:bg-rose-400/10 dark:text-rose-300 dark:border-rose-500/30",
      };
    }
    return {
      label: t("comments.status.approved"),
      className: "bg-emerald-100 text-emerald-600 border-emerald-200 dark:bg-emerald-400/10 dark:text-emerald-300 dark:border-emerald-500/30",
    };
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
          const likePending = likeMutation.isPending && likeMutation.variables?.id === item.id;
          const liked = Boolean(item.is_liked);
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
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    className={cn(
                      "inline-flex items-center gap-1 rounded-full border px-2.5 py-1 transition",
                      liked
                        ? "border-rose-300 bg-rose-50 text-rose-500 shadow-sm dark:border-rose-500/40 dark:bg-rose-500/10 dark:text-rose-200"
                        : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30",
                    )}
                    onClick={(event) => {
                      event.stopPropagation();
                      handleToggleLike(item.id, liked);
                    }}
                    disabled={likePending}
                    aria-pressed={liked}
                    aria-label={liked
                      ? t("publicPrompts.unlikeAction")
                      : t("publicPrompts.likeAction")}
                    title={liked
                      ? t("publicPrompts.unlikeAction")
                      : t("publicPrompts.likeAction")}
                  >
                    {likePending ? (
                      <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Heart
                        className="h-3.5 w-3.5 transition"
                        fill={liked ? "currentColor" : "none"}
                        strokeWidth={liked ? 1.5 : 2}
                      />
                    )}
                    <span>{item.like_count}</span>
                  </button>
                  <div className="flex items-center gap-1 text-slate-500 dark:text-slate-400">
                    <Download className="h-4 w-4" />
                    <span>{item.download_count}</span>
                  </div>
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
          const detailLiked = Boolean(selectedDetail.is_liked);
          const detailLikePending =
            likeMutation.isPending && likeMutation.variables?.id === selectedDetail.id;
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
                      <button
                        type="button"
                        className={cn(
                          "inline-flex h-10 items-center gap-2 rounded-full border px-4 text-sm font-medium transition",
                          detailLiked
                            ? "border-rose-300 bg-rose-50 text-rose-500 shadow-sm dark:border-rose-500/40 dark:bg-rose-500/10 dark:text-rose-200"
                            : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30",
                        )}
                        onClick={() => handleToggleLike(selectedDetail.id, detailLiked)}
                        disabled={detailLikePending}
                        aria-pressed={detailLiked}
                        aria-label={detailLiked
                          ? t("publicPrompts.unlikeAction")
                          : t("publicPrompts.likeAction")}
                        title={detailLiked
                          ? t("publicPrompts.unlikeAction")
                          : t("publicPrompts.likeAction")}
                      >
                        {detailLikePending ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <Heart
                            className="h-4 w-4 transition"
                            fill={detailLiked ? "currentColor" : "none"}
                            strokeWidth={detailLiked ? 1.5 : 2}
                          />
                        )}
                        <span>{t("publicPrompts.likeCountLabel", { count: selectedDetail.like_count })}</span>
                      </button>
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
              </GlassCard>        <GlassCard className="bg-white/85 dark:bg-slate-900/70 md:col-span-2">
          <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <span className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                  {t("comments.title")}
                </span>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  {t("comments.subtitle")}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge className="border-transparent bg-primary/10 text-primary dark:bg-primary/20">
                  {t("comments.total", { count: commentMeta?.total_items ?? commentItems.length })}
                </Badge>
                {isAdmin ? (
                  <div className="flex items-center gap-2">
                    {commentStatusOptions.map((option) => (
                      <button
                        key={`comment-status-${option}`}
                        type="button"
                        onClick={() => handleCommentStatusChange(option)}
                        className={cn(
                          "rounded-full border px-3 py-1 text-xs font-medium transition-colors",
                          commentStatusFilter === option
                            ? "border-primary/40 bg-primary/10 text-primary dark:border-primary/30 dark:bg-primary/20"
                            : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30",
                        )}
                      >
                        {option === "all"
                          ? t("comments.status.all")
                          : t(`comments.status.${option}`)}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <Textarea
                rows={3}
                value={commentBody}
                onChange={(event) => setCommentBody(event.target.value)}
                placeholder={t("comments.placeholder")}
                disabled={commentMutation.isPending || sourcePromptId == null}
              />
              <div className="flex justify-end">
                <Button
                  type="button"
                  className="inline-flex items-center gap-2"
                  onClick={handleSubmitComment}
                  disabled={commentMutation.isPending || sourcePromptId == null || isCommentBodyEmpty || offlineMode}
                >
                  {commentMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  {t("comments.submit")}
                </Button>
              </div>
            </div>

            {commentErrorMessage ? (
              <p className="text-sm text-rose-500 dark:text-rose-300">{commentErrorMessage}</p>
            ) : null}

            {offlineMode ? (
              <p className="text-xs text-slate-400 dark:text-slate-500">
                {t("comments.offlineDisabled")}
              </p>
            ) : null}

            {isLoadingComments ? (
              <div className="flex flex-col gap-3">
                {Array.from({ length: 3 }).map((_, index) => (
                  <div
                    key={`comment-skeleton-${index}`}
                    className="animate-pulse rounded-2xl border border-slate-200/60 bg-white/60 p-4 dark:border-slate-800/60 dark:bg-slate-900/40"
                  >
                    <div className="h-4 w-32 rounded bg-slate-200 dark:bg-slate-700" />
                    <div className="mt-3 h-3 w-full rounded bg-slate-200 dark:bg-slate-700" />
                    <div className="mt-2 h-3 w-3/4 rounded bg-slate-200 dark:bg-slate-700" />
                  </div>
                ))}
              </div>
            ) : commentItems.length === 0 ? (
              <p className="text-sm text-slate-400 dark:text-slate-500">
                {t("comments.empty")}
              </p>
            ) : (
              <div className="flex flex-col gap-4">
                {commentItems.map((item) => {
                  const statusMeta = commentStatusBadge(item.status);
                  const replyDraftValue = replyDrafts[item.id] ?? "";
                  const isReplyDraftEmpty = replyDraftValue.trim().length === 0;
                  return (
                    <div
                      key={`comment-${item.id}`}
                      className="flex flex-col gap-4 rounded-2xl border border-slate-200/60 bg-white/60 p-4 dark:border-slate-800/60 dark:bg-slate-900/40"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex flex-col gap-1">
                          <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                            {item.author?.username ?? t("comments.anonymous")}
                          </span>
                          <span className="text-xs text-slate-400 dark:text-slate-500">
                            {formatDateTime(item.created_at, i18n.language).date} · {formatDateTime(item.created_at, i18n.language).time}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          {item.status !== "approved" ? (
                            <Badge className={statusMeta.className}>{statusMeta.label}</Badge>
                          ) : null}
                          {isAdmin ? (
                            <div className="flex items-center gap-2">
                              <Button
                                type="button"
                                variant="outline"
                                className="h-8 px-3 text-xs"
                                disabled={commentReviewMutation.isPending || item.status === "approved"}
                                onClick={() => handleReviewComment(item.id, "approved")}
                              >
                                {t("comments.actions.approve")}
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                className="h-8 px-3 text-xs text-rose-500 border-rose-300 hover:bg-rose-50 dark:border-rose-500/40 dark:text-rose-200 dark:hover:bg-rose-500/10"
                                disabled={commentReviewMutation.isPending || item.status === "rejected"}
                                onClick={() => handleReviewComment(item.id, "rejected", item.review_note ?? "")}
                              >
                                {t("comments.actions.reject")}
                              </Button>
                            </div>
                          ) : null}
                        </div>
                      </div>
                      <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-600 dark:text-slate-300">{item.body}</p>
                      {item.review_note && isAdmin ? (
                        <p className="text-xs text-slate-500 dark:text-slate-400">{item.review_note}</p>
                      ) : null}
                      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400 dark:text-slate-500">
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 rounded-full border border-slate-200 px-3 py-1 transition hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30"
                          onClick={() => handleStartReply(item.id)}
                          disabled={commentMutation.isPending || sourcePromptId == null || item.status !== "approved" || offlineMode}
                        >
                          {t("comments.reply")}
                        </button>
                        <span>{t("comments.replyCount", { count: item.reply_count })}</span>
                      </div>
                      {replyTarget === item.id ? (
                        <div className="flex flex-col gap-2 rounded-xl border border-slate-200/60 bg-white/70 p-3 dark:border-slate-800/60 dark:bg-slate-900/30">
                          <Textarea
                            rows={3}
                            value={replyDraftValue}
                            onChange={(event) => handleReplyDraftChange(item.id, event.target.value)}
                            placeholder={t("comments.replyPlaceholder")}
                            disabled={commentMutation.isPending || offlineMode}
                          />
                          <div className="flex items-center justify-end gap-2">
                            <Button
                              type="button"
                              variant="outline"
                              className="h-8 px-3 text-xs"
                              onClick={handleCancelReply}
                              disabled={commentMutation.isPending}
                            >
                              {t("comments.cancelReply")}
                            </Button>
                            <Button
                              type="button"
                              className="h-8 px-3 text-xs"
                              onClick={() => handleSubmitReply(item.id)}
                              disabled={commentMutation.isPending}
                            >
                              {commentMutation.isPending ? (
                                <LoaderCircle className="mr-1 h-3.5 w-3.5 animate-spin" />
                              ) : null}
                              {t("comments.reply")}
                            </Button>
                          </div>
                        </div>
                      ) : null}
                      {item.replies && item.replies.length > 0 ? (
                        <div className="flex flex-col gap-3 border-l border-slate-200/60 pl-4 dark:border-slate-800/60">
                          {item.replies.map((reply) => {
                            const replyMeta = commentStatusBadge(reply.status);
                            return (
                              <div
                                key={`reply-${item.id}-${reply.id}`}
                                className="flex flex-col gap-2 rounded-2xl border border-slate-200/60 bg-white/60 p-3 dark:border-slate-800/60 dark:bg-slate-900/40"
                              >
                                <div className="flex items-start justify-between gap-3">
                                  <div className="flex flex-col gap-1">
                                    <span className="text-sm font-medium text-slate-700 dark:text-slate-200">
                                      {reply.author?.username ?? t("comments.anonymous")}
                                    </span>
                                    <span className="text-xs text-slate-400 dark:text-slate-500">
                                      {formatDateTime(reply.created_at, i18n.language).date} · {formatDateTime(reply.created_at, i18n.language).time}
                                    </span>
                                  </div>
                                  <div className="flex items-center gap-2">
                                    {reply.status !== "approved" ? (
                                      <Badge className={replyMeta.className}>{replyMeta.label}</Badge>
                                    ) : null}
                                    {isAdmin ? (
                                      <div className="flex items-center gap-1">
                                        <Button
                                          type="button"
                                          variant="ghost"
                                          className="h-8 px-2 text-xs"
                                          disabled={commentReviewMutation.isPending || reply.status === "approved"}
                                          onClick={() => handleReviewComment(reply.id, "approved")}
                                        >
                                          {t("comments.actions.approve")}
                                        </Button>
                                        <Button
                                          type="button"
                                          variant="ghost"
                                          className="h-8 px-2 text-xs text-rose-500 hover:text-rose-600 dark:text-rose-200 dark:hover:text-rose-100"
                                          disabled={commentReviewMutation.isPending || reply.status === "rejected"}
                                          onClick={() => handleReviewComment(reply.id, "rejected", reply.review_note ?? "")}
                                        >
                                          {t("comments.actions.reject")}
                                        </Button>
                                      </div>
                                    ) : null}
                                  </div>
                                </div>
                                <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-600 dark:text-slate-300">
                                  {reply.body}
                                </p>
                                {reply.review_note && isAdmin ? (
                                  <p className="text-xs text-slate-500 dark:text-slate-400">{reply.review_note}</p>
                                ) : null}
                              </div>
                            );
                          })}
                        </div>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            )}

            {commentMeta ? (
              <div className="flex flex-col gap-2 pt-2 text-xs text-slate-400 dark:text-slate-500 md:flex-row md:items-center md:justify-between">
                <span>{t("comments.pagination", { page: commentMeta.page, totalPages: commentMeta.total_pages, totalItems: commentMeta.total_items })}</span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    className="rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-500 transition hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30"
                    onClick={() => setCommentPage((prev) => Math.max(prev - 1, 1))}
                    disabled={commentPage <= 1}
                  >
                    {t("publicPrompts.prevPage")}
                  </button>
                  <button
                    type="button"
                    className="rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-500 transition hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30"
                    onClick={() =>
                      setCommentPage((prev) =>
                        commentMeta ? Math.min(prev + 1, commentMeta.total_pages) : prev + 1,
                      )
                    }
                    disabled={commentMeta ? commentPage >= commentMeta.total_pages : true}
                  >
                    {t("publicPrompts.nextPage")}
                  </button>
                </div>
              </div>
            ) : null}
          </div>
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
      <ConfirmDialog
        open={rejectDialog != null}
        title={t("comments.rejectConfirmTitle")}
        description={t("comments.rejectConfirmDescription")}
        confirmLabel={t("comments.actions.reject")}
        cancelLabel={t("common.cancel")}
        loading={commentReviewMutation.isPending}
        onCancel={() => {
          if (!commentReviewMutation.isPending) {
            setRejectDialog(null);
            setRejectNote("");
          }
        }}
        onConfirm={() => {
          if (!rejectDialog || commentReviewMutation.isPending) {
            return;
          }
          commentReviewMutation.mutate({
            commentId: rejectDialog.commentId,
            status: "rejected",
            note: rejectNote.trim() ? rejectNote.trim() : undefined,
          });
        }}
      >
        <Textarea
          rows={3}
          value={rejectNote}
          onChange={(event) => setRejectNote(event.target.value)}
          placeholder={t("comments.notePlaceholder")}
          disabled={commentReviewMutation.isPending}
        />
      </ConfirmDialog>
    </div>
  );
}
