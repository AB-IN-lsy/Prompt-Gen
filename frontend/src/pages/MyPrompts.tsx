import { FormEvent, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  LoaderCircle,
  Search,
  Edit3,
  Trash2,
  ChevronLeft,
  ChevronRight,
  RefreshCcw,
  Sparkles,
} from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import {
  PROMPT_KEYWORD_MAX_LENGTH,
  PROMPT_TAG_MAX_LENGTH,
} from "../config/prompt";
import { KEYWORD_ROW_LIMIT, DEFAULT_KEYWORD_WEIGHT } from "../config/env";
import {
  deletePrompt,
  fetchMyPrompts,
  fetchPromptDetail,
  PromptDetailResponse,
  PromptListItem,
  PromptListKeyword,
  PromptListResponse,
  PromptListMeta,
} from "../lib/api";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import type { Keyword, KeywordSource } from "../lib/api";
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

const fallbackSource = (value?: string): KeywordSource => {
  if (!value) return "manual";
  const normalised = value.toLowerCase();
  if (normalised === "local" || normalised === "api" || normalised === "manual") {
    return normalised as KeywordSource;
  }
  return "manual";
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

  const [status, setStatus] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const [committedSearch, setCommittedSearch] = useState("");

  const listQuery = useQuery<PromptListResponse>({
    queryKey: ["my-prompts", { status, page, committedSearch }],
    queryFn: () =>
      fetchMyPrompts({
        status: status === "all" ? undefined : status,
        page,
        pageSize: undefined,
        query: committedSearch || undefined,
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
    },
    onError: (error: unknown) => {
      toast.error(
        error instanceof Error ? error.message : t("errors.generic"),
      );
    },
  });

  const meta: PromptListMeta | undefined = listQuery.data?.meta;
  const items: PromptListItem[] = listQuery.data?.items ?? [];
  const totalPages = meta?.total_pages ?? 1;
  const isLoading = listQuery.isLoading;
  const isFetching = listQuery.isFetching;

  function populateWorkbench(detail: PromptDetailResponse) {
    resetWorkbench();
    setTopic(detail.topic);
    setPrompt(detail.body);
    setModel(detail.model);
    setPromptId(String(detail.id));
    setWorkspaceToken(detail.workspace_token ?? null);

    const positive = mapKeywords(detail.positive_keywords, "positive");
    const negative = mapKeywords(detail.negative_keywords, "negative");
    setCollections(positive, negative);
    setTags(detail.tags ?? []);
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
      return {
        id:
          item.keyword_id !== undefined && item.keyword_id !== null
            ? String(item.keyword_id)
            : `${polarity}-${item.word}-${index}`,
        word: value,
        polarity,
        source: fallbackSource(item.source),
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

  const handlePageChange = (nextPage: number) => {
    if (nextPage < 1) return;
    if (totalPages && nextPage > totalPages) return;
    setPage(nextPage);
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
        }
      />
      <form
          className="flex flex-col gap-3 rounded-3xl border border-white/60 bg-white/80 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 md:flex-row md:items-center md:justify-between"
          onSubmit={handleSearchSubmit}
        >
          <div className="flex flex-1 items-center gap-3">
            <div className="flex flex-1 items-center gap-2 rounded-full border border-white/60 bg-white/90 px-3 py-2 transition-colors dark:border-slate-700 dark:bg-slate-900/60">
              <Search className="h-4 w-4 text-slate-500 dark:text-slate-400" />
              <Input
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
                placeholder={t("myPrompts.searchPlaceholder")}
                className="border-none bg-transparent text-sm focus-visible:ring-0"
              />
            </div>
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
          <div className="overflow-x-auto">
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
                    onEdit={() => editMutation.mutate(item.id)}
                    onDelete={() => {
                      if (deleteMutation.isPending) {
                        return;
                      }
                      const confirmed = window.confirm(
                        t("myPrompts.deleteConfirm"),
                      );
                      if (!confirmed) {
                        return;
                      }
                      deleteMutation.mutate(item.id);
                    }}
                    isEditing={editingId === item.id && editMutation.isPending}
                    isDeleting={
                      deleteMutation.isPending &&
                      deleteMutation.variables === item.id
                    }
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <footer className="flex items-center justify-between rounded-3xl border border-white/60 bg-white/75 px-4 py-3 text-sm text-slate-500 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
        <span>
          {t("myPrompts.pagination.page", {
            page,
            total: totalPages,
          })}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handlePageChange(page - 1)}
            disabled={page <= 1}
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handlePageChange(page + 1)}
            disabled={page >= totalPages}
          >
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </footer>
    </div>
  );
}

interface PromptRowProps {
  item: PromptListItem;
  locale: string;
  onEdit: () => void;
  onDelete: () => void;
  isEditing: boolean;
  isDeleting: boolean;
}

function PromptRow({
  item,
  locale,
  onEdit,
  onDelete,
  isEditing,
  isDeleting,
}: PromptRowProps) {
  const { t } = useTranslation();
  const statusLabel = t(`myPrompts.statusBadge.${item.status}`);
  const formattedUpdatedAt = formatDateTime(item.updated_at, locale);
  return (
    <tr className="bg-white/40 transition hover:bg-white/70 dark:bg-slate-900/40 dark:hover:bg-slate-900/60">
      <td className="px-6 py-4 align-top">
        <div className="flex flex-col gap-1">
          <span className="text-sm font-semibold text-slate-800 dark:text-slate-100">
            {item.topic}
          </span>
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
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="outline"
            className="whitespace-nowrap"
            onClick={onEdit}
            disabled={isEditing || isDeleting}
          >
            {isEditing ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Edit3 className="h-4 w-4" />
            )}
            <span className="ml-2 whitespace-nowrap">
              {t("myPrompts.actions.edit")}
            </span>
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="whitespace-nowrap text-rose-500 hover:bg-rose-50 hover:text-rose-600 dark:text-rose-400 dark:hover:bg-rose-500/10"
            onClick={onDelete}
            disabled={isDeleting || isEditing}
          >
            {isDeleting ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
            <span className="ml-2 whitespace-nowrap">
              {t("myPrompts.actions.delete")}
            </span>
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
  const samples = keywords?.slice(0, KEYWORD_ROW_LIMIT) ?? [];
  const remaining = Math.max((keywords?.length ?? 0) - samples.length, 0);
  const badgeClass =
    polarity === "positive"
      ? "border-blue-200 text-blue-600 dark:border-blue-500/60 dark:text-blue-300"
      : "border-rose-200 text-rose-600 dark:border-rose-500/60 dark:text-rose-300";

  const hasContent = samples.length > 0;

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs font-semibold text-slate-400 dark:text-slate-500 whitespace-nowrap">
        {label}
      </span>
      <div className="flex flex-wrap items-center gap-1">
        {hasContent ? (
          <>
            {samples.map((keyword, index) => {
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
              <Badge
                variant="outline"
                className="border-slate-200 text-slate-400 dark:border-slate-700 dark:text-slate-400 whitespace-nowrap"
              >
                +{remaining}
              </Badge>
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
