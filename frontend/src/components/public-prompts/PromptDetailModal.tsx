import { type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck, Clock3, Copy, LoaderCircle, Sparkles, Tags, X } from "lucide-react";

import { GlassCard } from "../ui/glass-card";
import { Badge } from "../ui/badge";
import { Button } from "../ui/button";
import type { PublicPromptDetail } from "../../lib/api";

interface PromptDetailModalProps {
  open: boolean;
  detail: PublicPromptDetail | null;
  isLoading: boolean;
  isError: boolean;
  statusMeta?: { label: string; className: string } | null;
  updatedAt?: { date: string; time: string } | null;
  onClose: () => void;
  onRetry: () => void;
  onCopyBody?: (body?: string | null) => void;
  headerActions?: (detail: PublicPromptDetail) => ReactNode;
  beforeSections?: (detail: PublicPromptDetail) => ReactNode;
  afterSections?: (detail: PublicPromptDetail) => ReactNode;
}

export function PromptDetailModal({
  open,
  detail,
  isLoading,
  isError,
  statusMeta,
  updatedAt,
  onClose,
  onRetry,
  onCopyBody,
  headerActions,
  beforeSections,
  afterSections,
}: PromptDetailModalProps): JSX.Element | null {
  const { t } = useTranslation();

  if (!open) {
    return null;
  }

  const headerActionsNode = detail && headerActions ? headerActions(detail) : null;
  const beforeSectionsNode = detail && beforeSections ? beforeSections(detail) : null;
  const afterSectionsNode = detail && afterSections ? afterSections(detail) : null;
  const handleCopyBody = () => {
    if (!detail || !onCopyBody) {
      return;
    }
    onCopyBody(detail.body);
  };
  const positiveKeywords = detail?.positive_keywords ?? [];
  const negativeKeywords = detail?.negative_keywords ?? [];
  const tags = detail?.tags ?? [];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 px-4 py-8 backdrop-blur-sm"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <GlassCard
        className="relative w-full max-w-4xl overflow-hidden border-primary/40 bg-white/95 p-0 shadow-[0_45px_75px_-35px_rgba(59,130,246,0.7)] dark:border-primary/50 dark:bg-slate-900/95"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex max-h-[80vh] flex-col overflow-y-auto">
          <div className="relative border-b border-white/70 bg-white/90 px-6 pb-5 pt-7 dark:border-slate-800/60 dark:bg-slate-900/85">
            <button
              type="button"
              className="absolute right-6 top-6 inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/60 bg-white/80 text-slate-500 transition hover:border-primary/40 hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/40 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-primary/40"
              onClick={onClose}
              aria-label={t("common.close")}
            >
              <X className="h-5 w-5" />
            </button>
            <div className="flex flex-col gap-4">
              <div>
                <span className="text-xs font-semibold uppercase tracking-[0.35em] text-slate-400">
                  {t("publicPrompts.detailHeader.eyebrow")}
                </span>
                <h2 className="mt-2 pr-12 text-2xl font-semibold leading-tight text-slate-900 dark:text-white">
                  {detail?.title || detail?.topic || t("publicPrompts.detailHeader.subtitle")}
                </h2>
                <p className="mt-2 pr-12 text-sm text-slate-500 dark:text-slate-400">
                  {detail?.summary || t("publicPrompts.detailHeader.subtitle")}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400 dark:text-slate-500">
                <CircleCheck className="h-4 w-4 text-emerald-500" />
                <span>
                  {t("publicPrompts.detailMeta.model", {
                    model: detail?.model ?? "—",
                  })}
                </span>
                <span>·</span>
                <span>
                  {updatedAt
                    ? t("publicPrompts.detailMeta.updatedAt", {
                        date: updatedAt.date,
                        time: updatedAt.time,
                      })
                    : "—"}
                </span>
              </div>
              <GlassCard className="flex w-full flex-wrap items-center gap-3 border border-white/70 bg-white/85 px-4 py-3 dark:border-slate-800/70 dark:bg-slate-900/75">
                <div className="flex flex-wrap items-center gap-2 text-xs font-medium text-slate-500 dark:text-slate-300">
                  {statusMeta ? <Badge className={statusMeta.className}>{statusMeta.label}</Badge> : null}
                  {detail?.language ? (
                    <Badge
                      variant="outline"
                      className="border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300"
                    >
                      {detail.language.toUpperCase()}
                    </Badge>
                  ) : null}
                </div>
                {headerActionsNode ? (
                  <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
                    {headerActionsNode}
                  </div>
                ) : null}
              </GlassCard>
            </div>
            {detail?.review_reason ? (
              <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-700 dark:border-amber-400/40 dark:bg-amber-400/10 dark:text-amber-200">
                <p className="font-semibold uppercase tracking-[0.3em]">
                  {t("publicPrompts.reviewReasonTitle")}
                </p>
                <p className="mt-1 whitespace-pre-wrap leading-relaxed">
                  {detail.review_reason}
                </p>
              </div>
            ) : null}
          </div>
          {isLoading ? (
            <div className="flex h-64 items-center justify-center gap-3 p-8 text-slate-500 dark:text-slate-300">
              <LoaderCircle className="h-6 w-6 animate-spin text-primary" />
              <span>{t("publicPrompts.loadingDetail")}</span>
            </div>
          ) : isError ? (
            <div className="flex flex-col items-center gap-3 p-8 text-center text-slate-500 dark:text-slate-300">
              <p>{t("publicPrompts.detailError")}</p>
              <Button variant="secondary" size="sm" onClick={onRetry}>
                {t("common.retry")}
              </Button>
            </div>
          ) : detail ? (
            <>
              {beforeSectionsNode ? (
                <div className="flex flex-col gap-4 px-6 pb-0 pt-5">
                  {beforeSectionsNode}
                </div>
              ) : null}
              <div className="flex flex-col gap-5 px-6 pb-6 pt-5">
                <div className="grid gap-4 md:grid-cols-2">
                  <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                    <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                      <Sparkles className="h-4 w-4" />
                      {t("publicPrompts.keywords.positive")}
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {positiveKeywords.length > 0 ? (
                        positiveKeywords.map((keyword, index) => (
                          <Badge
                            key={`positive-${keyword.word}-${index}`}
                            variant="outline"
                            className="inline-flex items-center gap-1 border-emerald-300/60 text-emerald-600 dark:border-emerald-400/30 dark:text-emerald-300"
                          >
                            <span>{keyword.word}</span>
                            {typeof keyword.weight === "number" ? (
                              <span className="text-[11px] opacity-70">{keyword.weight}</span>
                            ) : null}
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
                      {negativeKeywords.length > 0 ? (
                        negativeKeywords.map((keyword, index) => (
                          <Badge
                            key={`negative-${keyword.word}-${index}`}
                            variant="outline"
                            className="inline-flex items-center gap-1 border-rose-300/60 text-rose-600 dark:border-rose-400/30 dark:text-rose-300"
                          >
                            <span>{keyword.word}</span>
                            {typeof keyword.weight === "number" ? (
                              <span className="text-[11px] opacity-70">{keyword.weight}</span>
                            ) : null}
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

            {tags.length > 0 ? (
              <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                  <Tags className="h-4 w-4" />
                  {t("publicPrompts.detail.tagsLabel")}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {tags.map((tag) => (
                    <Badge
                      key={`${detail?.id ?? "tag"}-${tag}`}
                      variant="outline"
                      className="border-slate-200 text-slate-500 dark:border-slate-700 dark:text-slate-300"
                    >
                      {tag}
                    </Badge>
                  ))}
                </div>
              </GlassCard>
            ) : null}

            <GlassCard className="bg-white/85 dark:bg-slate-900/70">
              <div className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-slate-400">
                <Clock3 className="h-4 w-4" />
                {t("publicPrompts.instructions")}
              </div>
              <div className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-slate-600 dark:text-slate-300">
                {detail.instructions && detail.instructions.trim().length > 0
                  ? detail.instructions
                  : t("publicPrompts.noInstructions")}
              </div>
            </GlassCard>

                <GlassCard className="bg-white/85 dark:bg-slate-900/70">
                  <div className="flex items-center justify-between text-xs uppercase tracking-[0.25em] text-slate-400">
                    <span>{t("publicPrompts.body")}</span>
                    <button
                      type="button"
                      className="inline-flex items-center gap-2 rounded-full bg-primary/5 px-3 py-1 text-[0.75rem] font-medium text-primary transition hover:bg-primary/10 hover:text-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:bg-primary/10 dark:text-primary-200 dark:hover:bg-primary/20"
                      onClick={handleCopyBody}
                      disabled={!onCopyBody || !detail.body}
                    >
                      <Copy className="h-4 w-4" />
                      {t("publicPrompts.copyBody")}
                    </button>
                  </div>
                  <pre className="mt-3 max-h-[40vh] overflow-y-auto whitespace-pre-wrap break-words rounded-2xl bg-slate-900/5 p-4 text-sm leading-relaxed text-slate-600 dark:bg-slate-900/80 dark:text-slate-200">
                    {detail.body || t("publicPrompts.copyBodyEmpty")}
                  </pre>
                </GlassCard>

                {afterSectionsNode}
              </div>
            </>
          ) : null}
        </div>
      </GlassCard>
    </div>
  );
}
