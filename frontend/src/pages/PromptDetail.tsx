import { useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { toast } from "sonner";

import { PageHeader } from "../components/layout/PageHeader";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import {
  fetchPromptDetail,
  PromptDetailResponse,
  PromptListKeyword,
  type Keyword,
  type KeywordSource,
} from "../lib/api";
import { cn, clampTextWithOverflow, formatOverflowLabel } from "../lib/utils";
import {
  PROMPT_KEYWORD_MAX_LENGTH,
  PROMPT_TAG_MAX_LENGTH,
  DEFAULT_KEYWORD_WEIGHT,
} from "../config/prompt";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";

export default function PromptDetailPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const promptId = Number.parseInt(id ?? "", 10);

  const resetWorkbench = usePromptWorkbench((state) => state.reset);
  const setTopic = usePromptWorkbench((state) => state.setTopic);
  const setPrompt = usePromptWorkbench((state) => state.setPrompt);
  const setModel = usePromptWorkbench((state) => state.setModel);
  const setPromptId = usePromptWorkbench((state) => state.setPromptId);
  const setWorkspaceToken = usePromptWorkbench((state) => state.setWorkspaceToken);
  const setCollections = usePromptWorkbench((state) => state.setCollections);
  const setTags = usePromptWorkbench((state) => state.setTags);

  const detailQuery = useQuery({
    queryKey: ["prompt-detail", promptId],
    enabled: Number.isInteger(promptId) && promptId > 0,
    queryFn: () => fetchPromptDetail(promptId),
  });

  const detail = detailQuery.data;

  const handleBack = () => {
    if (typeof window !== "undefined" && window.history.length <= 1) {
      navigate("/prompts");
      return;
    }
    navigate(-1);
  };

  const metaItems = useMemo(() => {
    if (!detail) return [];
    const formatter = new Intl.DateTimeFormat(i18n.language, {
      dateStyle: "medium",
      timeStyle: "short",
    });
    const formatDate = (value?: string | null) => {
      if (!value) return "—";
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return value;
      }
      return formatter.format(date);
    };
    return [
      {
        label: t("promptDetail.meta.status"),
        value: t(`myPrompts.statusBadge.${detail.status}`),
      },
      {
        label: t("promptDetail.meta.model"),
        value: detail.model || "—",
      },
      {
        label: t("promptDetail.meta.updatedAt"),
        value: formatDate(detail.updated_at),
      },
      {
        label: t("promptDetail.meta.createdAt"),
        value: formatDate(detail.created_at),
      },
      {
        label: t("promptDetail.meta.publishedAt"),
        value: formatDate(detail.published_at),
      },
    ];
  }, [detail, i18n.language, t]);

  const copyBody = async (content: string | undefined | null, successKey: string) => {
    if (!content) {
      toast.error(t("promptDetail.copy.empty"));
      return;
    }
    try {
      await navigator.clipboard.writeText(content);
      toast.success(t(successKey));
    } catch (error) {
      console.error("copy failed", error);
      toast.error(t("promptDetail.copy.failed"));
    }
  };

  const handleCopyPlain = () => copyBody(detail?.body, "promptDetail.copy.bodySuccess");
  const handleCopyMarkdown = () => {
    const markdown = detail?.body ?? "";
    copyBody(markdown, "promptDetail.copy.markdownSuccess");
  };

  const handleEdit = () => {
    if (!detail) return;
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
    navigate("/prompt-workbench");
  };

  return (
    <div className="flex h-full flex-col gap-6">
      <PageHeader
        eyebrow={t("promptDetail.eyebrow")}
        title={detail ? detail.topic : t("promptDetail.title")}
        description={t("promptDetail.subtitle")}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <Button variant="outline" onClick={handleBack}>
              {t("promptDetail.actions.back")}
            </Button>
            <Button variant="secondary" onClick={handleEdit} disabled={!detail}>
              {t("promptDetail.actions.edit")}
            </Button>
          </div>
        }
      />

      {detailQuery.isLoading ? (
        <LoadingState />
      ) : detailQuery.isError ? (
        <ErrorState onRetry={() => detailQuery.refetch()} />
      ) : !detail ? (
        <EmptyState />
      ) : (
        <div className="grid gap-6 lg:grid-cols-[320px,1fr]">
          <GlassCard className="space-y-4 self-start">
            <section className="space-y-2">
              <h2 className="text-sm font-semibold text-slate-600 dark:text-slate-300">
                {t("promptDetail.sections.meta")}
              </h2>
              <dl className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
                {metaItems.map(({ label, value }) => (
                  <div key={label} className="flex flex-col gap-1">
                    <dt className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
                      {label}
                    </dt>
                    <dd>{value}</dd>
                  </div>
                ))}
              </dl>
            </section>

            <section className="space-y-2">
              <h2 className="text-sm font-semibold text-slate-600 dark:text-slate-300">
                {t("promptDetail.sections.instructions")}
              </h2>
              <p className="text-sm text-slate-600 dark:text-slate-300">
                {detail.instructions?.trim() || t("promptDetail.empty")}
              </p>
            </section>

            <section className="space-y-3">
              <h2 className="text-sm font-semibold text-slate-600 dark:text-slate-300">
                {t("promptDetail.sections.tags")}
              </h2>
              {detail.tags.length === 0 ? (
                <p className="text-sm text-slate-500 dark:text-slate-400">{t("promptDetail.empty")}</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {detail.tags.map((tag) => {
                    const { value, overflow } = clampTextWithOverflow(tag, PROMPT_TAG_MAX_LENGTH);
                    return (
                      <Badge
                        key={tag}
                        variant="outline"
                        className="border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-300"
                      >
                        {formatOverflowLabel(value, overflow)}
                      </Badge>
                    );
                  })}
                </div>
              )}
            </section>

            <section className="space-y-3">
              <h2 className="text-sm font-semibold text-slate-600 dark:text-slate-300">
                {t("promptDetail.sections.keywords")}
              </h2>
              <KeywordGroup
                title={t("promptDetail.keywords.positive")}
                polarity="positive"
                keywords={detail.positive_keywords}
              />
              <KeywordGroup
                title={t("promptDetail.keywords.negative")}
                polarity="negative"
                keywords={detail.negative_keywords}
              />
            </section>
          </GlassCard>

          <GlassCard className="flex h-full flex-col gap-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
                  {t("promptDetail.sections.body")}
                </p>
                <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">
                  {t("promptDetail.bodyTitle")}
                </h2>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="secondary" size="sm" onClick={handleCopyPlain}>
                  {t("promptDetail.actions.copyBody")}
                </Button>
                <Button variant="outline" size="sm" onClick={handleCopyMarkdown}>
                  {t("promptDetail.actions.copyMarkdown")}
                </Button>
              </div>
            </div>

            <div className="prose max-w-none overflow-auto rounded-3xl border border-white/60 bg-white/80 p-6 text-slate-700 shadow-inner transition-colors dark:prose-invert dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200 whitespace-pre-wrap">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
                p: ({ children }) => <p className="whitespace-pre-wrap leading-relaxed">{children}</p>,
              }}>
                {detail.body || t("promptDetail.empty")}
              </ReactMarkdown>
            </div>
          </GlassCard>
        </div>
      )}
    </div>
  );
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
      id: `${polarity}-${keywordId ?? value}-${index}`,
      keywordId,
      word: value,
      polarity,
      source: fallbackSource(item.source),
      weight: normalizeWeight(item.weight),
      overflow,
    };
  });
}

function fallbackSource(source?: string): KeywordSource {
  const value = (source ?? "manual").toLowerCase();
  if (value === "local" || value === "api" || value === "manual") {
    return value as KeywordSource;
  }
  return "manual";
}

function normalizeWeight(weight?: number): number {
  if (typeof weight !== "number" || Number.isNaN(weight)) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  if (weight < 0) return 0;
  if (weight > DEFAULT_KEYWORD_WEIGHT) return DEFAULT_KEYWORD_WEIGHT;
  return Math.round(weight);
}

function KeywordGroup({
  title,
  polarity,
  keywords,
}: {
  title: string;
  polarity: "positive" | "negative";
  keywords: PromptListKeyword[];
}) {
  const { t } = useTranslation();

  if (!keywords || keywords.length === 0) {
    return (
      <div>
        <p className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">{title}</p>
        <p className="text-sm text-slate-500 dark:text-slate-400">{t("promptDetail.empty")}</p>
      </div>
    );
  }

  const badgeClass =
    polarity === "positive"
      ? "border-blue-200 text-blue-600 dark:border-blue-500/60 dark:text-blue-300"
      : "border-rose-200 text-rose-600 dark:border-rose-500/60 dark:text-rose-300";

  return (
    <div className="space-y-2">
      <p className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">{title}</p>
      <div className="flex flex-wrap gap-2">
        {keywords.map((keyword, index) => {
          const { value, overflow } = clampTextWithOverflow(
            keyword.word ?? "",
            PROMPT_KEYWORD_MAX_LENGTH,
          );
          return (
            <Badge
              key={`${keyword.keyword_id ?? keyword.word}-${index}`}
              variant="outline"
              className={cn(badgeClass, "whitespace-nowrap")}
            >
              {formatOverflowLabel(value, overflow)}
            </Badge>
          );
        })}
      </div>
    </div>
  );
}

function LoadingState() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-1 items-center justify-center rounded-3xl border border-dashed border-slate-200 py-20 text-sm text-slate-500 dark:border-slate-700 dark:text-slate-400">
      {t("common.loading")}
    </div>
  );
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 rounded-3xl border border-dashed border-rose-200 py-20 text-sm text-rose-500 dark:border-rose-500/40 dark:text-rose-300">
      <p>{t("promptDetail.error")}</p>
      <Button variant="secondary" onClick={onRetry}>
        {t("common.retry")}
      </Button>
    </div>
  );
}

function EmptyState() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-1 items-center justify-center rounded-3xl border border-dashed border-slate-200 py-20 text-sm text-slate-500 dark:border-slate-700 dark:text-slate-400">
      {t("promptDetail.empty")}
    </div>
  );
}
