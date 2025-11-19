/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-20 02:12:17
 * @FilePath: \electron-go-app\frontend\src\pages\PublicPrompts.tsx
 * @LastEditTime: 2025-10-20 02:12:17
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Download,
  ArrowDownUp,
  LoaderCircle,
  Sparkles,
  Clock3,
  Tags,
  CircleCheck,
  AlertCircle,
  Copy,
  ArrowUpRight,
  Heart,
  Eye,
} from "lucide-react";
import { GlassCard } from "../components/ui/glass-card";
import { PromptDetailModal } from "../components/public-prompts/PromptDetailModal";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { Badge } from "../components/ui/badge";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import { PaginationControls } from "../components/ui/pagination-controls";
import {
  deletePublicPrompt,
  downloadPublicPrompt,
  fetchPublicPromptDetail,
  fetchPublicPrompts,
  fetchPromptComments,
  createPromptComment,
  reviewPromptComment,
  likePromptComment,
  unlikePromptComment,
  likePublicPrompt,
  unlikePublicPrompt,
  type PublicPromptDownloadResult,
  type PublicPromptDetail,
  type PublicPromptLikeResult,
  type PublicPromptListResponse,
  type PublicPromptListItem,
  type PromptComment,
  type PromptCommentLikeResult,
  type PromptCommentListResponse,
} from "../lib/api";
import { AnimatePresence, motion } from "framer-motion";
import { buildCardMotion } from "../lib/animationConfig";
import { cn, resolveAssetUrl } from "../lib/utils";
import { useAuth, useIsAuthenticated } from "../hooks/useAuth";
import { isLocalMode } from "../lib/runtimeMode";
import { Button } from "../components/ui/button";
import { Textarea } from "../components/ui/textarea";
import { PROMPT_COMMENT_PAGE_SIZE, PUBLIC_PROMPT_LIST_PAGE_SIZE } from "../config/prompt";

type StatusFilter = "all" | "approved" | "pending" | "rejected";
type SortOption = "score" | "downloads" | "likes" | "visits" | "updated_at" | "created_at";

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

const sortOptions: SortOption[] = ["score", "downloads", "likes", "visits", "updated_at", "created_at"];

export default function PublicPromptsPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchParamsString = searchParams.toString();
  const queryFromUrl = searchParams.get("q") ?? "";
  const queryClient = useQueryClient();
  const profile = useAuth((state) => state.profile);
  const isAuthenticated = useIsAuthenticated();
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
  const [sortBy, setSortBy] = useState<SortOption>("score");
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
        sort: sortBy,
      },
    ],
    queryFn: () =>
      fetchPublicPrompts({
        page,
        pageSize: PUBLIC_PROMPT_LIST_PAGE_SIZE,
        query: debouncedSearch || undefined,
        status: statusFilter === "all" ? undefined : statusFilter,
        sortBy,
        sortOrder: "desc",
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
  const gridMotionKey = useMemo(
    () => items.map((item) => item.id).join("-"),
    [items],
  );

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
  const sourcePromptId = selectedDetail?.source_prompt_id ?? null;

  useEffect(() => {
    setCommentPage(1);
    setCommentBody("");
    setReplyDrafts({});
    setReplyTarget(null);
    setCommentStatusFilter(isAdmin ? "all" : "approved");
  }, [sourcePromptId, isAdmin]);

  const updateCommentLikeState = useCallback(
    (commentId: number, payload: PromptCommentLikeResult) => {
      if (sourcePromptId == null) {
        return;
      }
      const key = ["prompt-comments", sourcePromptId, commentPage, commentStatusFilter] as const;
      queryClient.setQueryData<PromptCommentListResponse>(key, (previous) => {
        if (!previous) {
          return previous;
        }
        const patch = (comment: PromptComment): PromptComment => {
          let repliesChanged = false;
          let nextReplies = comment.replies;
          if (comment.replies && comment.replies.length > 0) {
            const mapped = comment.replies.map((reply) => patch(reply));
            repliesChanged = mapped.some((next, index) => next !== comment.replies![index]);
            if (repliesChanged) {
              nextReplies = mapped;
            }
          }
          if (comment.id === commentId) {
            return {
              ...comment,
              like_count: payload.like_count,
              is_liked: payload.liked,
              replies: nextReplies,
            };
          }
          if (repliesChanged) {
            return {
              ...comment,
              replies: nextReplies,
            };
          }
          return comment;
        };
        const updatedItems = previous.items.map((item) => patch(item));
        const changed = updatedItems.some((item, index) => item !== previous.items[index]);
        if (!changed) {
          return previous;
        }
        return {
          ...previous,
          items: updatedItems,
        };
      });
    },
    [commentPage, commentStatusFilter, queryClient, sourcePromptId],
  );

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

  const commentLikeMutation = useMutation<PromptCommentLikeResult, unknown, { commentId: number; liked: boolean }>({
    mutationFn: ({ commentId, liked }) =>
      liked ? unlikePromptComment(commentId) : likePromptComment(commentId),
    onSuccess: (result, variables) => {
      updateCommentLikeState(variables.commentId, result);
      if (sourcePromptId != null) {
        void queryClient.invalidateQueries({ queryKey: ["prompt-comments", sourcePromptId] });
      }
    },
    onError: (error: unknown) => {
      const message = error instanceof Error ? error.message : t("comments.likeError");
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

  const handleToggleCommentLike = (commentId: number, liked: boolean, status: string) => {
    const normalizedStatus = typeof status === "string" ? status.toLowerCase() : "";
    if (normalizedStatus !== "approved") {
      return;
    }
    if (sourcePromptId == null) {
      toast.error(t("comments.loadError"));
      return;
    }
    if (offlineMode) {
      toast.error(t("comments.offlineDisabled"));
      return;
    }
    if (!isAuthenticated) {
      toast.error(t("comments.likeLoginRequired"));
      return;
    }
    if (commentLikeMutation.isPending && commentLikeMutation.variables?.commentId === commentId) {
      return;
    }
    commentLikeMutation.mutate({ commentId, liked });
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
  const handlePrevPage = () => setPage((prev) => Math.max(prev - 1, 1));
  const handleNextPage = () => setPage((prev) => Math.min(prev + 1, totalPages));

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

  const handleStatusChange = (value: StatusFilter) => {
    setStatusFilter(value);
    setPage(1);
  };

  const handleSortChange = (value: SortOption) => {
    setSortBy(value);
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

  const handleCopyBody = async (body?: string | null) => {
    const textToCopy = body ?? selectedDetail?.body ?? "";
    if (!textToCopy) {
      toast.error(t("publicPrompts.copyBodyEmpty"));
      return;
    }
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

  const renderAvatar = (displayName: string, rawSrc?: string | null, size: "md" | "sm" = "md") => {
    const resolvedSrc = resolveAssetUrl(rawSrc ?? null);
    const trimmed = displayName?.trim?.() ?? "";
    const firstChar = trimmed ? Array.from(trimmed)[0] : null;
    const fallback = typeof firstChar === "string" && firstChar.length > 0 ? firstChar.toUpperCase() : "U";
    const containerClass = size === "md" ? "h-10 w-10" : "h-8 w-8";
    const textClass = size === "md" ? "text-sm" : "text-xs";
    return (
      <div
        className={cn(
          "flex shrink-0 items-center justify-center overflow-hidden rounded-full border border-white/60 bg-primary/10 text-primary shadow-sm dark:border-slate-800/60 dark:bg-primary/20",
          containerClass,
        )}
      >
        {resolvedSrc ? (
          <img
            src={resolvedSrc}
            alt={t("comments.avatarAlt", { username: displayName })}
            className="h-full w-full object-cover"
          />
        ) : (
          <span className={cn("font-semibold uppercase", textClass)}>{fallback}</span>
        )}
      </div>
    );
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

  const detailStatusMeta = selectedDetail ? statusBadge(selectedDetail.status) : null;
  const detailUpdatedAt = selectedDetail
    ? formatDateTime(selectedDetail.updated_at, i18n.language)
    : null;

  const isLoadingList = listQuery.isLoading || listQuery.isFetching;
  const isLoadingDetail = detailQuery.isLoading;
  const detailError = detailQuery.isError;

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
            {statusOptions.map((item) => {
              const active = statusFilter === item;
              return (
                <button
                  key={item}
                  type="button"
                  onClick={() => handleStatusChange(item)}
                  aria-pressed={active}
                  className={cn(
                    "workbench-pill relative overflow-hidden rounded-full border px-3 py-1 text-xs font-medium uppercase tracking-[0.08em]",
                    active
                      ? "border-transparent bg-primary text-white shadow-glow"
                      : "border-white/60 text-slate-500 transition-colors hover:text-primary dark:border-slate-700 dark:text-slate-400",
                  )}
                >
                  {t(`publicPrompts.filters.${item}`)}
                </button>
              );
            })}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="inline-flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
            <ArrowDownUp className="h-4 w-4" />
            {t("publicPrompts.sort.title")}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {sortOptions.map((item) => {
              const active = sortBy === item;
              return (
                <button
                  key={item}
                  type="button"
                  onClick={() => handleSortChange(item)}
                  aria-pressed={active}
                  className={cn(
                    "workbench-pill relative overflow-hidden rounded-full border px-3 py-1 text-xs font-medium uppercase tracking-[0.08em]",
                    active
                      ? "border-transparent bg-primary text-white shadow-glow"
                      : "border-white/60 text-slate-500 transition-colors hover:text-primary dark:border-slate-700 dark:text-slate-400",
                  )}
                >
                  {t(`publicPrompts.sort.${item}`)}
                </button>
              );
            })}
          </div>
        </div>
    </div>

      <PaginationControls
        page={page}
        totalPages={totalPages}
        currentCount={currentCount}
        onPrev={handlePrevPage}
        onNext={handleNextPage}
        prevLabel={t("publicPrompts.prevPage")}
        nextLabel={t("publicPrompts.nextPage")}
        pageLabel={t("publicPrompts.paginationInfo", { page, totalPages })}
        countLabel={t("publicPrompts.paginationCount", { count: currentCount })}
        className="mb-4 border-none bg-transparent px-0 py-0 shadow-none dark:bg-transparent"
      />

      {isLoadingList && items.length === 0 ? (
        <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: PUBLIC_PROMPT_LIST_PAGE_SIZE }).map((_, index) => (
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
          ))}
        </div>
      ) : !isLoadingList && items.length === 0 ? (
        <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
          <GlassCard className="md:col-span-2 xl:col-span-3 border-dashed border-slate-200 bg-white/70 text-center dark:border-slate-800/60 dark:bg-slate-900/60">
            <Sparkles className="mx-auto h-8 w-8 text-primary" />
            <p className="mt-3 text-sm text-slate-500 dark:text-slate-400">
              {t("publicPrompts.empty")}
            </p>
          </GlassCard>
        </div>
      ) : (
        <AnimatePresence mode="wait">
          <motion.div
            key={`${gridMotionKey}-${page}-${sortBy}-${statusFilter}-${debouncedSearch}`}
            className="grid gap-5 md:grid-cols-2 xl:grid-cols-3"
            {...buildCardMotion({ offset: 20 })}
          >
        {items.map((item) => {
            const detailActive = selectedId === item.id;
            const { label, className } = statusBadge(item.status);
            const likePending = likeMutation.isPending && likeMutation.variables?.id === item.id;
            const liked = Boolean(item.is_liked);
            const scoreDisplay = Number.isFinite(item.quality_score)
            ? item.quality_score.toFixed(1)
            : "0.0";
          return (
            <GlassCard
              role="button"
              tabIndex={0}
              key={item.id}
              className={cn(
                "group relative flex cursor-pointer flex-col gap-4 border border-white/60 bg-white/70 transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-[0_25px_45px_-20px_rgba(59,130,246,0.35)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:border-slate-800/60 dark:bg-slate-900/60 dark:hover:border-primary/30",
                detailActive ? "border-primary/40 shadow-[0_25px_45px_-20px_rgba(59,130,246,0.45)] dark:border-primary/50" : "",
              )}
              onClick={() => setSelectedId((prev) => (prev === item.id ? null : item.id))}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  setSelectedId((prev) => (prev === item.id ? null : item.id));
                }
              }}
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
              <div className="flex items-center gap-2 text-xs font-medium text-primary/80 dark:text-primary/70">
                <Sparkles className="h-3.5 w-3.5" />
                <span>{t("publicPrompts.qualityScoreBadge", { score: scoreDisplay })}</span>
              </div>
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
                  {selectedDetail
                    ? (() => {
                        const scoreValue = Number.isFinite(selectedDetail.quality_score)
                          ? selectedDetail.quality_score.toFixed(1)
                          : "0.0";
                        const scoreLabel = t("publicPrompts.qualityScoreLabel", {
                          score: scoreValue,
                        });
                        return (
                          <div
                            className="flex items-center gap-1 text-primary/80 dark:text-primary/60"
                            aria-label={scoreLabel}
                            title={scoreLabel}
                          >
                            <Sparkles className="h-4 w-4" />
                            <span>{scoreLabel}</span>
                          </div>
                        );
                      })()
                    : null}
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
                  <div
                    className="flex items-center gap-1 text-slate-500 dark:text-slate-400"
                    aria-label={t("publicPrompts.visitCountShort", { count: item.visit_count })}
                    title={t("publicPrompts.visitCountShort", { count: item.visit_count })}
                  >
                    <Eye className="h-4 w-4" />
                    <span>{item.visit_count}</span>
                  </div>
                  <div className="flex items-center gap-1 text-slate-500 dark:text-slate-400">
                    <Download className="h-4 w-4" />
                    <span>{item.download_count}</span>
                  </div>
                </div>
              </div>
            </GlassCard>
          );
        })}
          </motion.div>
        </AnimatePresence>
      )}

      <PaginationControls
        page={page}
        totalPages={totalPages}
        currentCount={currentCount}
        onPrev={handlePrevPage}
        onNext={handleNextPage}
        prevLabel={t("publicPrompts.prevPage")}
        nextLabel={t("publicPrompts.nextPage")}
        pageLabel={t("publicPrompts.paginationInfo", { page, totalPages })}
        countLabel={t("publicPrompts.paginationCount", { count: currentCount })}
      />
      <p className="text-center text-xs text-slate-400 dark:text-slate-500 md:text-right">
        {t("publicPrompts.scoreRefreshHint")}
      </p>

      <PromptDetailModal
        open={selectedId != null}
        detail={selectedDetail ?? null}
        isLoading={isLoadingDetail}
        isError={detailError}
        statusMeta={detailStatusMeta}
        updatedAt={detailUpdatedAt}
        onClose={handleCloseDetail}
        onRetry={() => detailQuery.refetch()}
        onCopyBody={handleCopyBody}
        headerActions={(detail) => {
          const detailLiked = Boolean(detail.is_liked);
          const detailLikePending =
            likeMutation.isPending && likeMutation.variables?.id === detail.id;
          const isDownloadingCurrent =
            downloadMutation.isPending && downloadMutation.variables === detail.id;
          const isDeletingCurrent =
            deleteMutation.isPending && deleteMutation.variables === detail.id;
          return (
            <>
              <button
                type="button"
                className={cn(
                  "inline-flex h-10 items-center gap-2 rounded-full border px-4 text-sm font-medium transition",
                  detailLiked
                    ? "border-rose-300 bg-rose-50 text-rose-500 shadow-sm dark:border-rose-500/40 dark:bg-rose-500/10 dark:text-rose-200"
                    : "border-slate-200 text-slate-500 hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30",
                )}
                onClick={() => handleToggleLike(detail.id, detailLiked)}
                disabled={detailLikePending}
                aria-pressed={detailLiked}
                aria-label={detailLiked ? t("publicPrompts.unlikeAction") : t("publicPrompts.likeAction")}
                title={detailLiked ? t("publicPrompts.unlikeAction") : t("publicPrompts.likeAction")}
              >
                {detailLikePending ? (
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                ) : (
                  <Heart className="h-4 w-4 transition" fill={detailLiked ? "currentColor" : "none"} strokeWidth={detailLiked ? 1.5 : 2} />
                )}
                <span className="font-semibold text-slate-700 dark:text-slate-200">
                  {detail.like_count}
                </span>
              </button>
              <div
                className="inline-flex h-10 items-center gap-2 rounded-full border border-slate-200 px-4 text-sm text-slate-500 dark:border-slate-700 dark:text-slate-400"
                aria-label={t("publicPrompts.visitCountLabel", { count: detail.visit_count })}
                title={t("publicPrompts.visitCountLabel", { count: detail.visit_count })}
              >
                <Eye className="h-4 w-4" />
                <span className="font-semibold text-slate-700 dark:text-slate-200">{detail.visit_count}</span>
              </div>
              <Button
                type="button"
                className="inline-flex h-10 items-center gap-2 rounded-full px-5 text-sm font-medium"
                disabled={isDownloadingCurrent || isDeletingCurrent}
                onClick={() => handleDownload(detail.id)}
              >
                {isDownloadingCurrent ? (
                  <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Download className="mr-2 h-4 w-4" />
                )}
                {t("publicPrompts.downloadShort")}
              </Button>
              {allowDelete ? (
                <Button
                  type="button"
                  variant="outline"
                  className="h-10 rounded-full border-rose-300 px-4 text-rose-600 hover:bg-rose-50 dark:border-rose-500/40 dark:text-rose-300 dark:hover:bg-rose-500/10"
                  disabled={isDeletingCurrent || isDownloadingCurrent}
                  onClick={() => setConfirmDeleteId(detail.id)}
                >
                  {isDeletingCurrent ? (
                    <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <AlertCircle className="mr-2 h-4 w-4" />
                  )}
                  {t("publicPrompts.deleteShort")}
                </Button>
              ) : null}
            </>
          );
        }}
        beforeSections={(detail) =>
          detail.author ? (
            <GlassCard className="flex flex-col gap-3 rounded-3xl border border-white/70 bg-white/90 px-6 py-4 dark:border-slate-800/60 dark:bg-slate-900/80">
              <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400">
                {t("publicPrompts.authorCard.title")}
              </p>
              <div className="flex flex-wrap items-center gap-4">
                <Link to={`/creators/${detail.author.id}`} className="shrink-0">
                  {renderAvatar(detail.author.username, detail.author.avatar_url ?? null, "md")}
                </Link>
                <div className="flex flex-1 flex-col">
                  <p className="text-sm font-semibold text-slate-900 dark:text-white">
                    {detail.author.username}
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {detail.author.headline ? detail.author.headline : t("publicPrompts.authorCard.fallback")}
                  </p>
                </div>
                <Link to={`/creators/${detail.author.id}`}>
                  <Button variant="secondary" size="sm" className="inline-flex items-center gap-1">
                    {t("publicPrompts.authorCard.viewProfile")}
                    <ArrowUpRight className="h-4 w-4" />
                  </Button>
                </Link>
              </div>
            </GlassCard>
          ) : null
        }
        afterSections={() => (
          <GlassCard className="bg-white/85 dark:bg-slate-900/70">
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
                  disabled={commentMutation.isPending}
                />
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    type="button"
                    size="sm"
                    className="inline-flex items-center gap-2"
                    disabled={commentMutation.isPending || isCommentBodyEmpty}
                    onClick={handleSubmitComment}
                  >
                    {commentMutation.isPending ? (
                      <LoaderCircle className="h-4 w-4 animate-spin" />
                    ) : (
                      <Sparkles className="h-4 w-4" />
                    )}
                    {t("comments.submit")}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="text-slate-500 dark:text-slate-400"
                    onClick={() => setCommentBody("")}
                    disabled={commentMutation.isPending || commentBody.length === 0}
                  >
                    {t("comments.reset")}
                  </Button>
                </div>
              </div>

              <div className="flex flex-col gap-4">
                {commentItems.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-slate-200 p-6 text-center text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
                    {t("comments.empty")}
                  </div>
                ) : (
                  commentItems.map((item) => {
                    const commentMeta = commentStatusBadge(item.status);
                    const commentDisplayName = item.author?.username ?? t("comments.anonymous");
                    const commentCreatedAt = formatDateTime(item.created_at, i18n.language);
                    const commentLiked = Boolean(item.is_liked);
                    const likeCount = item.like_count ?? 0;
                    const commentAuthorId = item.author?.id ?? null;
                    const commentAvatar = renderAvatar(
                      commentDisplayName,
                      item.author?.avatar_url ?? null,
                      "md",
                    );
                    const commentAvatarNode = commentAuthorId ? (
                      <Link to={`/creators/${commentAuthorId}`} className="shrink-0">
                        {commentAvatar}
                      </Link>
                    ) : (
                      commentAvatar
                    );
                    const commentNameNode = (
                      <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                        {commentDisplayName}
                      </span>
                    );
                    return (
                      <div
                        key={`comment-${item.id}`}
                        className="flex flex-col gap-4 rounded-2xl border border-slate-200/60 bg-white/60 p-4 dark:border-slate-800/60 dark:bg-slate-900/40"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="flex flex-1 items-start gap-3">
                            {commentAvatarNode}
                            <div className="flex flex-1 flex-col gap-1">
                              {commentNameNode}
                              <span className="text-xs text-slate-400 dark:text-slate-500">
                                {commentCreatedAt.date} · {commentCreatedAt.time}
                              </span>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            {item.status !== "approved" ? (
                              <Badge className={commentMeta.className}>{commentMeta.label}</Badge>
                            ) : null}
                            {isAdmin ? (
                              <div className="flex items-center gap-2">
                                <Button
                                  type="button"
                                  variant="outline"
                                  className="h-8 px-3 text-xs transition hover:bg-primary/10 hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/50"
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
                            className={cn(
                              "inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium transition",
                              commentLiked
                                ? "border-transparent bg-primary/10 text-primary dark:bg-primary/20"
                                : "border-slate-200 text-slate-500 hover:border-primary/30 hover:bg-primary/10 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30 dark:hover:bg-primary/15",
                              "focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/40",
                            )}
                            disabled={commentLikeMutation.isPending && commentLikeMutation.variables?.commentId === item.id}
                            onClick={() => handleToggleCommentLike(item.id, commentLiked, item.status)}
                          >
                            <Heart className="h-3.5 w-3.5" fill={commentLiked ? "currentColor" : "none"} strokeWidth={commentLiked ? 1.5 : 2} />
                            <span>{likeCount}</span>
                          </button>
                          <button
                            type="button"
                            className="inline-flex items-center gap-1 rounded-full border border-slate-200 px-3 py-1 text-xs font-medium text-slate-500 transition hover:border-primary/30 hover:text-primary dark:border-slate-700 dark:text-slate-400"
                            onClick={() => handleStartReply(item.id)}
                          >
                            <ArrowDownUp className="h-3.5 w-3.5" />
                            {t("comments.actions.reply")}
                          </button>
                        </div>
                        {replyTarget === item.id ? (
                          <div className="flex flex-col gap-3 rounded-2xl border border-slate-200/60 bg-white/60 p-3 dark:border-slate-800/60 dark:bg-slate-900/40">
                            <Textarea
                              rows={2}
                              value={replyDrafts[item.id] ?? ""}
                              onChange={(event) =>
                                setReplyDrafts((prev) => ({ ...prev, [item.id]: event.target.value }))
                              }
                              placeholder={t("comments.replyPlaceholder")}
                              disabled={commentMutation.isPending}
                            />
                            <div className="flex flex-wrap items-center gap-2">
                              <Button
                                type="button"
                                size="sm"
                                className="inline-flex items-center gap-2"
                                disabled={commentMutation.isPending}
                                onClick={() => handleSubmitReply(item.id)}
                              >
                                {commentMutation.isPending ? (
                                  <LoaderCircle className="h-4 w-4 animate-spin" />
                                ) : (
                                  <Sparkles className="h-4 w-4" />
                                )}
                                {t("comments.submitReply")}
                              </Button>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="text-slate-500 dark:text-slate-400"
                                disabled={commentMutation.isPending}
                                onClick={handleCancelReply}
                              >
                                {t("common.cancel")}
                              </Button>
                            </div>
                          </div>
                        ) : null}
                        {item.replies?.length ? (
                          <div className="flex flex-col gap-3 rounded-2xl border border-slate-200/60 bg-white/50 p-3 dark:border-slate-800/60 dark:bg-slate-900/30">
                            {item.replies.map((reply) => {
                              const replyMeta = commentStatusBadge(reply.status);
                              const replyDisplayName = reply.author?.username ?? t("comments.anonymous");
                              const replyCreatedAt = formatDateTime(reply.created_at, i18n.language);
                              const replyAuthorId = reply.author?.id ?? null;
                              const replyAvatar = renderAvatar(
                                replyDisplayName,
                                reply.author?.avatar_url ?? null,
                                "sm",
                              );
                              const replyAvatarNode = replyAuthorId ? (
                                <Link to={`/creators/${replyAuthorId}`} className="shrink-0">
                                  {replyAvatar}
                                </Link>
                              ) : (
                                replyAvatar
                              );
                              const replyNameNode = (
                                <span className="text-sm font-medium text-slate-700 dark:text-slate-200">
                                  {replyDisplayName}
                                </span>
                              );
                              return (
                                <div
                                  key={`reply-${item.id}-${reply.id}`}
                                  className="flex flex-col gap-2 rounded-2xl border border-slate-200/60 bg-white/60 p-3 dark:border-slate-800/60 dark:bg-slate-900/40"
                                >
                                  <div className="flex items-start justify-between gap-3">
                                    <div className="flex flex-1 items-start gap-3">
                                      {replyAvatarNode}
                                      <div className="flex flex-1 flex-col gap-1">
                                        {replyNameNode}
                                        <span className="text-xs text-slate-400 dark:text-slate-500">
                                          {replyCreatedAt.date} · {replyCreatedAt.time}
                                        </span>
                                      </div>
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
                                            className="h-8 px-2 text-xs transition hover:bg-primary/10 hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/50"
                                            disabled={commentReviewMutation.isPending || reply.status === "approved"}
                                            onClick={() => handleReviewComment(reply.id, "approved")}
                                          >
                                            {t("comments.actions.approve")}
                                          </Button>
                                          <Button
                                            type="button"
                                            variant="ghost"
                                            className="h-8 px-2 text-xs text-rose-500 transition hover:bg-rose-50 hover:text-rose-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-rose-400 dark:text-rose-200 dark:hover:bg-rose-500/10 dark:hover:text-rose-100"
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
                                  <div className="flex items-center gap-2 text-xs text-slate-400 dark:text-slate-500">
                                    <button
                                      type="button"
                                      className={cn(
                                        "inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium transition",
                                        reply.is_liked
                                          ? "border-transparent bg-primary/10 text-primary dark:bg-primary/20"
                                          : "border-slate-200 text-slate-500 hover:border-primary/30 hover:bg-primary/10 hover:text-primary dark:border-slate-700 dark:text-slate-400 dark:hover:border-primary/30 dark:hover:bg-primary/15",
                                      )}
                                      disabled={commentLikeMutation.isPending && commentLikeMutation.variables?.commentId === reply.id}
                                      onClick={() => handleToggleCommentLike(reply.id, Boolean(reply.is_liked), reply.status)}
                                    >
                                      <Heart className="h-3.5 w-3.5" fill={reply.is_liked ? "currentColor" : "none"} strokeWidth={reply.is_liked ? 1.5 : 2} />
                                      <span>{reply.like_count ?? 0}</span>
                                    </button>
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        ) : null}
                      </div>
                    );
                  })
                )}
              </div>

              <PaginationControls
                page={commentPage}
                totalPages={commentMeta?.total_pages ?? 1}
                currentCount={commentMeta?.current_count ?? commentItems.length}
                onPrev={() => setCommentPage((prev) => Math.max(prev - 1, 1))}
                onNext={() => setCommentPage((prev) => (commentMeta?.total_pages ? Math.min(prev + 1, commentMeta.total_pages) : prev + 1))}
                prevLabel={t("publicPrompts.prevPage")}
                nextLabel={t("publicPrompts.nextPage")}
                pageLabel={t("comments.pagination", {
                  page: commentPage,
                  totalPages: commentMeta?.total_pages ?? 1,
                  totalItems: commentMeta?.total_items ?? commentItems.length,
                })}
                countLabel={t("comments.paginationCount", {
                  count: commentMeta?.current_count ?? commentItems.length,
                })}
              />
            </div>
          </GlassCard>
        )}
      />
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
