import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { buildCardMotion } from "../lib/animationConfig";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  LoaderCircle,
  Edit3,
  Trash2,
  Save,
  RefreshCcw,
  Sparkles,
  Settings,
  Star,
} from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { SpotlightSearch } from "../components/ui/spotlight-search";
import { PaginationControls } from "../components/ui/pagination-controls";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import {
  PROMPT_KEYWORD_MAX_LENGTH,
  PROMPT_TAG_MAX_LENGTH,
  KEYWORD_ROW_LIMIT,
  DEFAULT_KEYWORD_WEIGHT,
  MY_PROMPTS_PAGE_SIZE,
  PROMPT_GENERATE_TEMPERATURE_DEFAULT,
  PROMPT_GENERATE_TOP_P_DEFAULT,
  PROMPT_GENERATE_MAX_OUTPUT_DEFAULT,
} from "../config/prompt";
import {
  deletePrompt,
  fetchMyPrompts,
  fetchPromptDetail,
  savePrompt,
  updatePromptFavorite,
  normaliseKeywordSource,
  PromptDetailResponse,
  PromptListItem,
  PromptListKeyword,
  PromptListResponse,
  PromptListMeta,
  PromptKeywordInput,
  SavePromptRequest,
} from "../lib/api";
import { ApiError } from "../lib/errors";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import type { Keyword } from "../lib/api";
import { nanoid } from "nanoid";
import { clampTextWithOverflow, formatOverflowLabel, cn } from "../lib/utils";
import { PageHeader } from "../components/layout/PageHeader";

type StatusFilter = "all" | "draft" | "published" | "archived";

const clampWeight = (value?: number): number => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  if (value < 0) return 0;
  if (value > DEFAULT_KEYWORD_WEIGHT) return DEFAULT_KEYWORD_WEIGHT;
  return Math.round(value);
};

type FormattedDateTime = {
  date: string;
  time: string;
};

const formatDateTime = (value?: string | null, locale?: string): FormattedDateTime => {
  if (!value) return { date: "—", time: "" };
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return { date: value, time: "" };
  }
  const dateFormatter = new Intl.DateTimeFormat(locale ?? undefined, { dateStyle: "short" });
  const timeFormatter = new Intl.DateTimeFormat(locale ?? undefined, { timeStyle: "short" });
  return {
    date: dateFormatter.format(date),
    time: timeFormatter.format(date),
  };
};

export default function MyPromptsPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const resetWorkbench = usePromptWorkbench((state) => state.reset);
  const setTopic = usePromptWorkbench((state) => state.setTopic);
  const setPrompt = usePromptWorkbench((state) => state.setPrompt);
  const setModel = usePromptWorkbench((state) => state.setModel);
  const setPromptId = usePromptWorkbench((state) => state.setPromptId);
  const setWorkspaceToken = usePromptWorkbench((state) => state.setWorkspaceToken);
  const setCollections = usePromptWorkbench((state) => state.setCollections);
  const setTags = usePromptWorkbench((state) => state.setTags);
  const setInstructions = usePromptWorkbench((state) => state.setInstructions);
  const overwriteGenerationProfile = usePromptWorkbench(
    (state) => state.overwriteGenerationProfile,
  );

  const [status, setStatus] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const [committedSearch, setCommittedSearch] = useState("");
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [publishingId, setPublishingId] = useState<number | null>(null);
  const [favoritedOnly, setFavoritedOnly] = useState(false);
  const [favoritingId, setFavoritingId] = useState<number | null>(null);

  const listQuery = useQuery<PromptListResponse>({
    queryKey: ["my-prompts", { status, page, committedSearch, favoritedOnly }],
    queryFn: () =>
      fetchMyPrompts({
        status: status === "all" ? undefined : status,
        page,
        pageSize: MY_PROMPTS_PAGE_SIZE,
        query: committedSearch || undefined,
        favorited: favoritedOnly,
      }),
    placeholderData: (previousData) => previousData,
  });
  const [editingId, setEditingId] = useState<number | null>(null);
  const editMutation = useMutation({
    mutationFn: fetchPromptDetail,
    onMutate: (id: number) => {
      setEditingId(id);
      toast.dismiss("prompt-edit");
      toast.loading(t("myPrompts.loading"), { id: "prompt-edit" });
    },
    onSuccess: (detail) => {
      toast.dismiss("prompt-edit");
      populateWorkbench(detail);
      toast.success(t("myPrompts.editSuccess"));
      navigate("/prompt-workbench");
    },
    onError: (error: unknown) => {
      toast.dismiss("prompt-edit");
      toast.error(
        error instanceof Error ? error.message : t("errors.generic"),
      );
    },
    onSettled: () => {
      setEditingId(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deletePrompt,
    onSuccess: () => {
      toast.success(t("myPrompts.deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["my-prompts"] });
      setConfirmDeleteId(null);
    },
    onError: (error: unknown) => {
      toast.error(
        error instanceof Error ? error.message : t("errors.generic"),
      );
    },
  });

  const publishMutation = useMutation({
    mutationFn: async (promptId: number) => {
      const detail = await fetchPromptDetail(promptId);
      const trimmedTopic = detail.topic.trim();
      const trimmedBody = detail.body.trim();
      const trimmedInstructions = (detail.instructions ?? "").trim();
      const missingMessages: string[] = [];
      if (!trimmedTopic) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.topic", {
            defaultValue: "发布前需要填写主题",
          }),
        );
      }
      if (!trimmedBody) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.body", {
            defaultValue: "发布前需要填写提示词正文",
          }),
        );
      }
      if (!trimmedInstructions) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.instructions", {
            defaultValue: "发布前需要补充要求",
          }),
        );
      }
      if (!detail.model) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.model", {
            defaultValue: "发布前需要选择模型",
          }),
        );
      }
      if (detail.positive_keywords.length === 0) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.positiveKeywords", {
            defaultValue: "发布前至少保留一个正向关键词",
          }),
        );
      }
      if (detail.negative_keywords.length === 0) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.negativeKeywords", {
            defaultValue: "发布前至少保留一个负向关键词",
          }),
        );
      }
      if (detail.tags.length === 0) {
        missingMessages.push(
          t("promptWorkbench.publishValidation.tags", {
            defaultValue: "发布前至少设置一个标签",
          }),
        );
      }
      if (missingMessages.length > 0) {
        missingMessages.forEach((message) => toast.warning(message));
        throw new ApiError({
          message: t("promptWorkbench.publishValidation.failed", {
            defaultValue: "发布条件未满足，请补全必填项",
          }),
        });
      }

      const payload: SavePromptRequest = {
        prompt_id: detail.id,
        topic: trimmedTopic,
        body: trimmedBody,
        model: detail.model,
        instructions: trimmedInstructions || undefined,
        publish: true,
        status: "published",
        tags: detail.tags ?? [],
        positive_keywords: detail.positive_keywords.map((keyword) =>
          promptListKeywordToInput(keyword, "positive"),
        ),
        negative_keywords: detail.negative_keywords.map((keyword) =>
          promptListKeywordToInput(keyword, "negative"),
        ),
        workspace_token: detail.workspace_token ?? undefined,
      };
      return savePrompt(payload);
    },
    onMutate: (id: number) => {
      setPublishingId(id);
    },
    onSuccess: () => {
      toast.success(
        t("myPrompts.publishSuccess", {
          defaultValue: "已发布，状态已更新为“已发布”。",
        }),
      );
      void queryClient.invalidateQueries({ queryKey: ["my-prompts"] });
      void queryClient.invalidateQueries({ queryKey: ["public-prompts"] });
    },
    onError: (error: unknown) => {
      if (error instanceof ApiError) {
        toast.error(error.message ?? t("myPrompts.publishFailed"));
      } else {
        toast.error(t("myPrompts.publishFailed"));
      }
    },
    onSettled: () => {
      setPublishingId(null);
    },
  });

  const favoriteMutation = useMutation({
    mutationFn: (variables: { promptId: number; favorited: boolean }) =>
      updatePromptFavorite(variables),
    onMutate: (variables) => {
      setFavoritingId(variables.promptId);
    },
    onSuccess: (_result, variables) => {
      queryClient.setQueriesData<PromptListResponse>(
        { queryKey: ["my-prompts"] },
        (previous) => {
          if (!previous) {
            return previous;
          }
          return {
            ...previous,
            items: previous.items.map((record) =>
              record.id === variables.promptId
                ? { ...record, is_favorited: variables.favorited }
                : record,
            ),
          };
        },
      );
      queryClient.setQueryData<PromptDetailResponse>(
        ["prompt-detail", variables.promptId],
        (previous) =>
          previous ? { ...previous, is_favorited: variables.favorited } : previous,
      );
      toast.success(
        variables.favorited
          ? t("myPrompts.favorite.success")
          : t("myPrompts.favorite.removed"),
      );
    },
    onError: (error: unknown) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t("myPrompts.favorite.failed"),
      );
    },
    onSettled: () => {
      setFavoritingId(null);
      void queryClient.invalidateQueries({ queryKey: ["my-prompts"] });
    },
  });

  const meta: PromptListMeta | undefined = listQuery.data?.meta;
  const items: PromptListItem[] = listQuery.data?.items ?? [];
  const motionKey = useMemo(
    () => items.map((item) => item.id).join("-"),
    [items],
  );
  const totalPages = meta?.total_pages ?? 1;
  const currentCount =
    meta?.current_count ?? Math.min(items.length, MY_PROMPTS_PAGE_SIZE);
  const isLoading = listQuery.isLoading;
  const isFetching = listQuery.isFetching;

  function populateWorkbench(detail: PromptDetailResponse) {
    resetWorkbench();
    setTopic(detail.topic);
    setPrompt(detail.body);
    setModel(detail.model);
    setPromptId(String(detail.id));
    setWorkspaceToken(detail.workspace_token ?? null);
    setInstructions(detail.instructions ?? "");

    const positive = mapKeywords(detail.positive_keywords, "positive");
    const negative = mapKeywords(detail.negative_keywords, "negative");
    setCollections(positive, negative);
    setTags(detail.tags ?? []);
    if (detail.generation_profile) {
      overwriteGenerationProfile({
        stepwiseReasoning: Boolean(detail.generation_profile.stepwise_reasoning),
        temperature:
          typeof detail.generation_profile.temperature === "number"
            ? detail.generation_profile.temperature
            : PROMPT_GENERATE_TEMPERATURE_DEFAULT,
        topP:
          typeof detail.generation_profile.top_p === "number"
            ? detail.generation_profile.top_p
            : PROMPT_GENERATE_TOP_P_DEFAULT,
        maxOutputTokens:
          typeof detail.generation_profile.max_output_tokens === "number"
            ? detail.generation_profile.max_output_tokens
            : PROMPT_GENERATE_MAX_OUTPUT_DEFAULT,
      });
    }
  }

  function mapKeywords(
    keywords: PromptListKeyword[],
    polarity: "positive" | "negative",
  ): Keyword[] {
    return keywords.map((item, index) => {
      const { value, overflow } = clampTextWithOverflow(
        item.word ?? "",
        PROMPT_KEYWORD_MAX_LENGTH,
      );
      const keywordId =
        item.keyword_id !== undefined && item.keyword_id !== null
          ? Number(item.keyword_id)
          : undefined;
      return {
        id: nanoid(),
        keywordId,
        word: value,
        polarity,
        source: normaliseKeywordSource(item.source),
        weight: clampWeight(item.weight),
        overflow,
      };
    });
  }

  const handleSearchSubmit = (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    setCommittedSearch(searchInput.trim());
    setPage(1);
  };

  const handleStatusChange = (value: StatusFilter) => {
    setStatus(value);
    setPage(1);
  };

  const handleFavoriteFilterToggle = () => {
    setFavoritedOnly((prev) => {
      const next = !prev;
      setPage(1);
      return next;
    });
  };

  const handlePageChange = (nextPage: number) => {
    if (nextPage < 1) return;
    if (totalPages && nextPage > totalPages) return;
    setPage(nextPage);
  };

  const handleToggleFavorite = (item: PromptListItem) => {
    if (!item) return;
    favoriteMutation.mutate({
      promptId: item.id,
      favorited: !Boolean(item.is_favorited),
    });
  };

  const statusOptions: Array<{ value: StatusFilter; label: string }> = useMemo(
    () => [
      { value: "all", label: t("myPrompts.statusFilter.all") },
      { value: "draft", label: t("myPrompts.statusFilter.draft") },
      { value: "published", label: t("myPrompts.statusFilter.published") },
      { value: "archived", label: t("myPrompts.statusFilter.archived") },
    ],
    [t],
  );

  return (
    <div className="flex h-full flex-col gap-6">
      <PageHeader
        eyebrow={t("myPrompts.eyebrow")}
        title={t("myPrompts.title")}
        description={t("myPrompts.subtitle")}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="shadow-sm dark:shadow-none"
              onClick={() => navigate("/settings?tab=app")}
            >
              <Settings className="mr-2 h-4 w-4" />
              {t("myPrompts.manageBackups")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              className="shadow-sm dark:shadow-none"
              onClick={() => {
                resetWorkbench();
                navigate("/prompt-workbench");
              }}
            >
              <RefreshCcw className="mr-2 h-4 w-4" />
              {t("myPrompts.openWorkbench")}
            </Button>
          </div>
        }
      />
      <form
        className="flex flex-col gap-3 rounded-3xl border border-white/60 bg-white/80 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 md:flex-row md:items-center md:justify-between"
        onSubmit={handleSearchSubmit}
      >
          <div className="flex flex-1 items-center gap-3">
            <SpotlightSearch
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder={t("myPrompts.searchPlaceholder")}
              className="flex-1"
              name="my-prompts-search"
            />
            <Button type="submit" size="sm" variant="secondary">
              {t("common.search")}
            </Button>
          </div>
          <div className="flex items-center gap-3">
            <label className="text-xs font-medium uppercase tracking-[0.22em] text-slate-400">
              {t("myPrompts.statusFilter.label")}
            </label>
            <select
              value={status}
              onChange={(event) =>
                handleStatusChange(event.target.value as StatusFilter)
              }
              className="h-10 rounded-xl border border-white/60 bg-white/90 px-3 text-sm text-slate-600 transition focus:outline-none focus:ring-2 focus:ring-primary/40 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-200"
            >
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <Button
              type="button"
              size="sm"
              variant={favoritedOnly ? "secondary" : "outline"}
              aria-pressed={favoritedOnly}
              onClick={handleFavoriteFilterToggle}
              className={cn(
                "whitespace-nowrap",
                favoritedOnly
                  ? "shadow-sm"
                  : "border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300",
              )}
            >
              <Star
                className={cn(
                  "h-4 w-4",
                  favoritedOnly ? "text-amber-400" : "text-slate-400",
                )}
                fill={favoritedOnly ? "currentColor" : "none"}
              />
              <span className="ml-2">
                {favoritedOnly
                  ? t("myPrompts.favoriteFilter.active")
                  : t("myPrompts.favoriteFilter.label")}
              </span>
            </Button>
          </div>
      </form>

      <section className="flex-1 overflow-hidden rounded-3xl border border-white/60 bg-white/85 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
        <div className="flex items-center justify-between border-b border-white/60 px-6 py-3 text-xs uppercase tracking-[0.26em] text-slate-400 dark:border-slate-800">
          <span>{t("myPrompts.listTitle")}</span>
          {isFetching ? (
            <span className="flex items-center gap-2 text-slate-400">
              <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
              {t("common.loading")}
            </span>
          ) : null}
        </div>
        <div className="px-4 pt-4">
          <PaginationControls
            page={page}
            totalPages={totalPages}
            currentCount={currentCount}
            onPrev={() => handlePageChange(page - 1)}
            onNext={() => handlePageChange(page + 1)}
            prevLabel={t("publicPrompts.prevPage")}
            nextLabel={t("publicPrompts.nextPage")}
            pageLabel={t("myPrompts.pagination.page", { page, total: totalPages })}
            countLabel={t("myPrompts.pagination.count", { count: currentCount })}
            className="border-none bg-transparent px-0 py-0 shadow-none dark:bg-transparent"
          />
        </div>

        {isLoading ? (
          <div className="flex flex-col gap-4 px-6 py-10">
            {Array.from({ length: 4 }).map((_, index) => (
              <SkeletonRow key={index} />
            ))}
          </div>
        ) : listQuery.isError ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-16 text-center text-sm text-slate-500 dark:text-slate-400">
            <p>{t("myPrompts.loadError")}</p>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => listQuery.refetch()}
            >
              <RefreshCcw className="mr-2 h-4 w-4" />
              {t("common.retry")}
            </Button>
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-16 text-center text-sm text-slate-500 dark:text-slate-400">
            <p>{t("myPrompts.empty")}</p>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                resetWorkbench();
                navigate("/prompt-workbench");
              }}
            >
              <Sparkles className="mr-2 h-4 w-4" />
              {t("myPrompts.startCreating")}
            </Button>
          </div>
        ) : (
          <AnimatePresence mode="wait">
            {items.length > 0 ? (
              <motion.div
                key={`${motionKey}-${status}-${page}-${favoritedOnly}-${committedSearch}`}
                className="overflow-x-auto"
                {...buildCardMotion({ offset: 16 })}
              >
                <table className="min-w-full divide-y divide-white/60 text-sm dark:divide-slate-800">
              <thead className="bg-white/80 text-left text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:bg-slate-900/60">
                <tr>
                  <th className="px-6 py-3">{t("myPrompts.table.topic")}</th>
                  <th className="px-4 py-3">{t("myPrompts.table.status")}</th>
                  <th className="px-4 py-3">{t("myPrompts.table.model")}</th>
                  <th className="px-4 py-3">{t("myPrompts.table.keywords")}</th>
                  <th className="px-4 py-3">{t("myPrompts.table.tags")}</th>
                  <th className="px-4 py-3">{t("myPrompts.table.updatedAt")}</th>
                  <th className="px-4 py-3 text-right">
                    {t("myPrompts.table.actions")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/60 dark:divide-slate-800">
                {items.map((item: PromptListItem) => (
                <PromptRow
                  key={item.id}
                  item={item}
                  locale={i18n.language}
                  onView={() => navigate(`/prompts/${item.id}`)}
                  onEdit={() => editMutation.mutate(item.id)}
                  onToggleFavorite={() => handleToggleFavorite(item)}
                  onPublish={() => {
                    if (publishMutation.isPending) {
                      return;
                    }
                    publishMutation.mutate(item.id);
                  }}
                  onDelete={() => {
                    if (deleteMutation.isPending) {
                      return;
                    }
                    setConfirmDeleteId(item.id);
                  }}
                  isEditing={editingId === item.id && editMutation.isPending}
                  isDeleting={
                    deleteMutation.isPending &&
                    deleteMutation.variables === item.id
                  }
                  isPublishing={
                    publishMutation.isPending && publishingId === item.id
                  }
                  isFavorited={Boolean(item.is_favorited)}
                  isFavoriting={
                    favoriteMutation.isPending && favoritingId === item.id
                  }
                />
                ))}
              </tbody>
                </table>
              </motion.div>
            ) : null}
          </AnimatePresence>
        )}
      </section>

      <PaginationControls
        page={page}
        totalPages={totalPages}
        currentCount={currentCount}
        onPrev={() => handlePageChange(page - 1)}
        onNext={() => handlePageChange(page + 1)}
        prevLabel={t("publicPrompts.prevPage")}
        nextLabel={t("publicPrompts.nextPage")}
        pageLabel={t("myPrompts.pagination.page", { page, total: totalPages })}
        countLabel={t("myPrompts.pagination.count", { count: currentCount })}
      />
      <ConfirmDialog
        open={confirmDeleteId != null}
        title={t("myPrompts.deleteDialogTitle")}
        description={t("myPrompts.deleteConfirm")}
        confirmLabel={t("myPrompts.actions.delete")}
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

interface PromptRowProps {
  item: PromptListItem;
  locale: string;
  onView: () => void;
  onEdit: () => void;
  onToggleFavorite: () => void;
  onPublish: () => void;
  onDelete: () => void;
  isEditing: boolean;
  isDeleting: boolean;
  isPublishing: boolean;
  isFavorited: boolean;
  isFavoriting: boolean;
}

function promptListKeywordToInput(
  keyword: PromptListKeyword,
  polarity: "positive" | "negative",
): PromptKeywordInput {
  return {
    keyword_id: keyword.keyword_id,
    word: keyword.word,
    polarity,
    source: keyword.source,
    weight: clampWeight(keyword.weight),
  };
}

function PromptRow({
  item,
  locale,
  onView,
  onEdit,
  onToggleFavorite,
  onPublish,
  onDelete,
  isEditing,
  isDeleting,
  isPublishing,
  isFavorited,
  isFavoriting,
}: PromptRowProps) {
  const { t } = useTranslation();
  const statusLabel = t(`myPrompts.statusBadge.${item.status}`);
  const formattedUpdatedAt = formatDateTime(item.updated_at, locale);
  return (
    <tr className="group bg-white/40 transition duration-200 hover:-translate-y-0.5 hover:bg-primary/5 hover:shadow-[0_8px_20px_-10px_rgba(59,130,246,0.35)] hover:ring-2 hover:ring-primary/25 hover:ring-offset-2 hover:ring-offset-white dark:bg-slate-900/40 dark:hover:bg-primary/10 dark:hover:ring-offset-slate-900">
      <td className="px-6 py-4 align-top">
        <div className="flex items-start gap-3">
          <button
            type="button"
            onClick={onToggleFavorite}
            disabled={isFavoriting}
            aria-pressed={isFavorited}
            aria-label={
              isFavorited
                ? t("myPrompts.actions.unfavorite")
                : t("myPrompts.actions.favorite")
            }
            className={cn(
              "flex h-9 w-9 items-center justify-center rounded-full border transition-colors duration-200",
              "border-transparent bg-white/70 text-slate-400 hover:border-amber-200 hover:text-amber-500 dark:bg-slate-900/50 dark:text-slate-500 dark:hover:text-amber-300",
              isFavorited
                ? "border-amber-300 text-amber-500 dark:border-amber-400 dark:text-amber-200"
                : "",
              isFavoriting ? "opacity-60" : "",
            )}
          >
            <Star
              className="h-4 w-4"
              fill={isFavorited ? "currentColor" : "none"}
            />
          </button>
          <div className="flex flex-col gap-1">
            <button
              type="button"
              onClick={onView}
              className="bg-transparent text-left text-sm font-semibold text-slate-800 transition-all duration-200 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:text-slate-100 dark:hover:text-primary-200 group-hover:text-primary group-hover:translate-x-1 dark:group-hover:text-primary-200"
            >
              {item.topic}
            </button>
          </div>
        </div>
      </td>
      <td className="px-4 py-4 align-top">
        <Badge
          variant="outline"
          className={cn(
            "whitespace-nowrap",
            item.status === "published"
              ? "border-emerald-300 text-emerald-600 dark:border-emerald-500 dark:text-emerald-300"
              : item.status === "draft"
                ? "border-slate-300 text-slate-500 dark:border-slate-600 dark:text-slate-300"
                : "border-amber-300 text-amber-600 dark:border-amber-500 dark:text-amber-300",
          )}
        >
          {statusLabel}
        </Badge>
      </td>
      <td className="px-4 py-4 align-top">
        <span className="text-sm text-slate-600 dark:text-slate-300">
          {item.model}
        </span>
      </td>
      <td className="px-4 py-4 align-top">
        <div className="flex flex-col gap-2">
          {renderKeywordLine(
            item.positive_keywords,
            "positive",
            t("myPrompts.keywordLabels.positive", { defaultValue: "正" }),
          )}
          {renderKeywordLine(
            item.negative_keywords,
            "negative",
            t("myPrompts.keywordLabels.negative", { defaultValue: "负" }),
          )}
        </div>
      </td>
      <td className="px-4 py-4 align-top">
        <div className="flex flex-wrap gap-1">
          {item.tags.length === 0 ? (
            <span className="text-xs text-slate-400 dark:text-slate-600">
              —
            </span>
          ) : (
            item.tags.map((tag) => {
              const { value, overflow } = clampTextWithOverflow(
                tag,
                PROMPT_TAG_MAX_LENGTH,
              );
              return (
                <Badge
                  key={tag}
                  variant="outline"
                  className="border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300 whitespace-nowrap"
                  title={value}
                >
                  {formatOverflowLabel(value, overflow)}
                </Badge>
              );
            })
          )}
        </div>
      </td>
      <td className="px-4 py-4 align-top text-sm text-slate-500 dark:text-slate-400 whitespace-nowrap tabular-nums">
        <div className="flex flex-col leading-tight">
          <span>{formattedUpdatedAt.date}</span>
          {formattedUpdatedAt.time ? (
            <span className="text-xs text-slate-400 dark:text-slate-500">{formattedUpdatedAt.time}</span>
          ) : null}
        </div>
      </td>
      <td className="px-4 py-4 align-top">
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button
            size="sm"
            className="h-9 w-9 min-w-[2.25rem] rounded-xl bg-primary/85 p-0 text-white shadow-sm hover:bg-primary focus-visible:ring-primary/60 dark:bg-primary/80"
            onClick={onPublish}
            disabled={isPublishing || isEditing || isDeleting}
            title={t("myPrompts.actions.publish")}
            aria-label={t("myPrompts.actions.publish")}
          >
            {isPublishing ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Save className="h-4 w-4" />
            )}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-9 w-9 min-w-[2.25rem] rounded-xl p-0"
            onClick={onEdit}
            disabled={isEditing || isDeleting}
            title={t("myPrompts.actions.edit")}
            aria-label={t("myPrompts.actions.edit")}
          >
            {isEditing ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Edit3 className="h-4 w-4" />
            )}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-9 w-9 min-w-[2.25rem] rounded-xl border-rose-200 p-0 text-rose-500 hover:bg-rose-50 hover:text-rose-600 dark:border-rose-500/40 dark:text-rose-300 dark:hover:bg-rose-500/10"
            onClick={onDelete}
            disabled={isDeleting || isEditing}
            title={t("myPrompts.actions.delete")}
            aria-label={t("myPrompts.actions.delete")}
          >
            {isDeleting ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
          </Button>
        </div>
      </td>
    </tr>
  );
}

function renderKeywordLine(
  keywords: PromptListKeyword[] | undefined,
  polarity: "positive" | "negative",
  label: string,
) {
  return (
    <KeywordLine keywords={keywords} polarity={polarity} label={label} />
  );
}

interface KeywordLineProps {
  keywords?: PromptListKeyword[];
  polarity: "positive" | "negative";
  label: string;
}

function KeywordLine({ keywords, polarity, label }: KeywordLineProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const list = keywords ?? [];
  const visible = list.slice(0, KEYWORD_ROW_LIMIT);
  const hidden = list.slice(visible.length);
  const remaining = hidden.length;
  const badgeClass =
    polarity === "positive"
      ? "border-blue-200 text-blue-600 dark:border-blue-500/60 dark:text-blue-300"
      : "border-rose-200 text-rose-600 dark:border-rose-500/60 dark:text-rose-300";
  const hasContent = visible.length > 0;

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("keydown", handleEscape);
    };
  }, [open]);

  const triggerLabel = t("myPrompts.keywordOverflow.trigger", {
    count: remaining,
  });

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs font-semibold text-slate-400 dark:text-slate-500 whitespace-nowrap">
        {label}
      </span>
      <div className="flex flex-wrap items-center gap-1">
        {hasContent ? (
          <>
            {visible.map((keyword, index) => {
              const { value, overflow } = clampTextWithOverflow(
                keyword.word ?? "",
                PROMPT_KEYWORD_MAX_LENGTH,
              );
              return (
                <Badge
                  key={`${polarity}-${keyword.word}-${index}`}
                  variant="outline"
                  className={`${badgeClass} whitespace-nowrap`}
                  title={value}
                >
                  {formatOverflowLabel(value, overflow)}
                </Badge>
              );
            })}
            {remaining > 0 ? (
              <div className="relative" ref={containerRef}>
                <button
                  type="button"
                  className={cn(
                    "inline-flex items-center rounded-full border border-slate-200 px-2.5 py-1 text-xs font-medium text-slate-400 transition-colors hover:border-slate-300 hover:text-slate-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-2 dark:border-slate-700 dark:text-slate-400 dark:hover:border-slate-500 dark:hover:text-slate-300 dark:focus-visible:ring-offset-slate-900",
                  )}
                  onClick={() => setOpen((prev) => !prev)}
                  aria-expanded={open}
                  aria-haspopup="dialog"
                  aria-label={triggerLabel}
                >
                  +{remaining}
                </button>
                {open ? (
                  <div className="absolute left-1/2 top-full z-30 mt-2 w-56 -translate-x-1/2 rounded-2xl border border-slate-200 bg-white/95 p-3 text-left shadow-xl backdrop-blur-md dark:border-slate-700 dark:bg-slate-900/95">
                    <div className="text-xs font-semibold text-slate-500 dark:text-slate-400">
                      {t("myPrompts.keywordOverflow.title", {
                        count: remaining,
                      })}
                    </div>
                    <div className="mt-2 flex flex-wrap gap-1">
                      {hidden.length > 0 ? (
                        hidden.map((keyword, index) => {
                          const { value, overflow } = clampTextWithOverflow(
                            keyword.word ?? "",
                            PROMPT_KEYWORD_MAX_LENGTH,
                          );
                          return (
                            <Badge
                              key={`${polarity}-hidden-${keyword.word}-${index}`}
                              variant="outline"
                              className={`${badgeClass} whitespace-nowrap`}
                              title={value}
                            >
                              {formatOverflowLabel(value, overflow)}
                            </Badge>
                          );
                        })
                      ) : (
                        <span className="text-xs text-slate-400 dark:text-slate-500">
                          {t("myPrompts.keywordOverflow.empty")}
                        </span>
                      )}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </>
        ) : (
          <span className="text-xs text-slate-400 dark:text-slate-600">—</span>
        )}
      </div>
    </div>
  );
}

function SkeletonRow() {
  return (
    <div className="flex animate-pulse flex-col gap-3 rounded-2xl border border-white/60 bg-white/70 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/60 md:flex-row md:items-center md:justify-between">
      <div className="flex-1">
        <div className="h-4 w-40 rounded-full bg-slate-200/80 dark:bg-slate-700/60" />
      </div>
      <div className="hidden h-3 w-24 rounded-full bg-slate-200/80 dark:bg-slate-700/60 md:block" />
      <div className="hidden h-3 w-32 rounded-full bg-slate-200/80 dark:bg-slate-700/60 md:block" />
      <div className="h-8 w-32 rounded-full bg-slate-200/80 dark:bg-slate-700/60" />
    </div>
  );
}
