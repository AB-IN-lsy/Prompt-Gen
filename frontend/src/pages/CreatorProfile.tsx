import { useCallback, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowUpRight,
  Download,
  Eye,
  Globe,
  Heart,
  LoaderCircle,
  MapPin,
  Sparkles,
} from "lucide-react";

import {
  fetchCreatorProfile,
  fetchPublicPromptDetail,
  fetchPublicPrompts,
  type PublicPromptAuthor,
  type PublicPromptDetail,
  type PublicPromptListItem,
} from "../lib/api";
import { GlassCard } from "../components/ui/glass-card";
import { PromptDetailModal } from "../components/public-prompts/PromptDetailModal";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { PaginationControls } from "../components/ui/pagination-controls";
import { cn, clampTextWithOverflow, formatOverflowLabel, resolveAssetUrl } from "../lib/utils";
import { toast } from "sonner";

const CREATOR_LIST_PAGE_SIZE = 6;

const formatDateTime = (value?: string | null, locale?: string) => {
  if (!value) {
    return { date: "—", time: "" };
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return { date: value ?? "—", time: "" };
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

export default function CreatorProfilePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const creatorId = Number.parseInt(id ?? "", 10);
  const { t, i18n } = useTranslation();
  const [page, setPage] = useState(1);
  const [selectedPromptId, setSelectedPromptId] = useState<number | null>(null);
  const numberFormatter = useMemo(
    () =>
      new Intl.NumberFormat(i18n.language, {
        notation: "compact",
      }),
    [i18n.language],
  );

  const profileQuery = useQuery({
    queryKey: ["creator-profile", creatorId],
    enabled: Number.isFinite(creatorId) && creatorId > 0,
    queryFn: () => fetchCreatorProfile(creatorId),
  });

  const promptListQuery = useQuery({
    queryKey: ["creator-prompts", creatorId, page],
    enabled: Number.isFinite(creatorId) && creatorId > 0,
    queryFn: () =>
      fetchPublicPrompts({
        authorId: creatorId,
        page,
        pageSize: CREATOR_LIST_PAGE_SIZE,
        sortBy: "updated_at",
        sortOrder: "desc",
      }),
  });
  const detailQuery = useQuery<PublicPromptDetail>({
    queryKey: ["creator-prompt-detail", selectedPromptId],
    enabled: selectedPromptId != null,
    queryFn: () => fetchPublicPromptDetail(selectedPromptId ?? 0),
  });

  if (!Number.isFinite(creatorId) || creatorId <= 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-lg font-semibold text-slate-700 dark:text-slate-200">
          {t("creatorPage.invalid")}
        </p>
        <Link to="/public-prompts" className="inline-flex">
          <Button variant="secondary" size="sm">
            {t("creatorPage.backToList")}
          </Button>
        </Link>
      </div>
    );
  }

  const profile = profileQuery.data;
  const creator: PublicPromptAuthor | null = profile?.creator ?? null;
  const heroAvatarUrl = resolveAssetUrl(creator?.avatar_url ?? null);
  const stats = profile?.stats;
  const heroBio =
    creator?.bio && creator.bio.trim().length > 0
      ? creator.bio
      : t("creatorPage.emptyBio");
  const heroHeadline =
    creator?.headline && creator.headline.trim().length > 0
      ? creator.headline
      : t("creatorPage.fallbackHeadline");
  const highlightPrompts = profile?.recent_prompts ?? [];
  const scrollToPrompts = () => {
    if (typeof document === "undefined") return;
    const target = document.getElementById("creator-prompts");
    target?.scrollIntoView({ behavior: "smooth", block: "start" });
  };
  const selectedDetail = detailQuery.data;
  const isDetailLoading = detailQuery.isLoading;
  const detailError = detailQuery.isError;
  const detailModalOpen = selectedPromptId != null;
  const handleSelectPrompt = useCallback((promptId: number) => {
    setSelectedPromptId(promptId);
  }, []);
  const handleCloseDetail = useCallback(() => {
    setSelectedPromptId(null);
  }, []);
  const buildStatusBadge = useCallback(
    (status: string) => {
      const normalized = status.toLowerCase();
      if (normalized === "approved") {
        return {
          label: t("publicPrompts.status.approved"),
          className:
            "bg-emerald-100 text-emerald-600 border-emerald-200 dark:bg-emerald-400/10 dark:text-emerald-300 dark:border-emerald-500/30",
        };
      }
      if (normalized === "pending") {
        return {
          label: t("publicPrompts.status.pending"),
          className:
            "bg-amber-100 text-amber-600 border-amber-200 dark:bg-amber-400/10 dark:text-amber-300 dark:border-amber-500/30",
        };
      }
      if (normalized === "rejected") {
        return {
          label: t("publicPrompts.status.rejected"),
          className:
            "bg-rose-100 text-rose-600 border-rose-200 dark:bg-rose-400/10 dark:text-rose-300 dark:border-rose-500/30",
        };
      }
      return {
        label: status,
        className:
          "bg-slate-200 text-slate-600 border-slate-300 dark:bg-slate-700 dark:text-slate-200 dark:border-slate-600",
      };
    },
    [t],
  );
  const handleCopyBody = useCallback(
    async (body?: string | null) => {
      const text = (body ?? "").trim();
      if (!text) {
        toast.error(t("publicPrompts.copyBodyEmpty"));
        return;
      }
      try {
        if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(text);
          toast.success(t("publicPrompts.copyBodySuccess"));
        } else {
          throw new Error("clipboard unsupported");
        }
      } catch (error) {
        console.error("copy prompt body failed", error);
        toast.error(t("publicPrompts.copyBodyFailure"));
      }
    },
    [t],
  );
  const detailStatusMeta = selectedDetail
    ? buildStatusBadge(selectedDetail.status)
    : null;
  const detailUpdatedAt = selectedDetail
    ? formatDateTime(selectedDetail.updated_at, i18n.language)
    : null;

  return (
    <div className="flex flex-col gap-8">
      {profileQuery.isLoading ? (
        <div className="flex min-h-[240px] items-center justify-center">
          <LoaderCircle className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : profileQuery.isError ? (
        <div className="flex min-h-[240px] flex-col items-center justify-center gap-4">
          <p className="text-base text-slate-500 dark:text-slate-300">
            {t("creatorPage.loadError")}
          </p>
          <Button
            variant="secondary"
            onClick={() => {
              profileQuery.refetch();
            }}
          >
            {t("creatorPage.retry")}
          </Button>
        </div>
      ) : (
        <>
          <div className="relative overflow-hidden rounded-[32px] border border-white/30 bg-gradient-to-br from-primary/25 via-violet-500/20 to-rose-400/20 p-1 shadow-[0_45px_85px_-55px_rgba(14,165,233,0.9)] transition-all duration-300 hover:border-primary/60 hover:shadow-[0_55px_105px_-60px_rgba(59,130,246,0.85)] dark:border-slate-800/60 dark:hover:border-primary/60">
            <div className="rounded-[28px] bg-slate-950/80 p-6 text-white">
              <div className="flex flex-col gap-6 md:flex-row md:items-center">
                <div className="relative">
                  <div className="h-32 w-32 overflow-hidden rounded-3xl border border-white/20 bg-white/90 shadow-2xl">
                    {heroAvatarUrl ? (
                      <img
                        src={heroAvatarUrl}
                        alt={creator?.username ?? "creator avatar"}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center bg-slate-900 text-4xl font-semibold uppercase">
                        {(creator?.username ?? "U").slice(0, 1)}
                      </div>
                    )}
                  </div>
                </div>
                <div className="flex-1 space-y-3">
                  <div className="flex flex-wrap items-center gap-3">
                    <h1 className="text-3xl font-semibold leading-tight">
                      {creator?.username ?? t("creatorPage.unknownName")}
                    </h1>
                    <Sparkles className="h-5 w-5 text-amber-300" />
                  </div>
                  <p className="text-lg text-slate-200">{heroHeadline}</p>
                  <p className="text-sm text-slate-300">{heroBio}</p>
                  <div className="flex flex-wrap items-center gap-4 text-sm text-slate-300">
                    {creator?.location ? (
                      <span className="flex items-center gap-1">
                        <MapPin className="h-4 w-4 text-emerald-300" />
                        {t("creatorPage.location", { location: creator.location })}
                      </span>
                    ) : null}
                    {creator?.website ? (
                      <a
                        href={creator.website}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-sky-300 transition hover:text-sky-200"
                      >
                        <Globe className="h-4 w-4" />
                        {t("creatorPage.website")}
                      </a>
                    ) : null}
                  </div>
                </div>
                <div className="space-y-3">
                  <Button
                    variant="secondary"
                    className="w-full justify-between"
                    onClick={scrollToPrompts}
                  >
                    {t("creatorPage.allPromptsCta")}
                    <ArrowUpRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                {[
                  {
                    label: t("creatorPage.stats.prompts"),
                    value: stats?.prompt_count ?? 0,
                  },
                  {
                    label: t("creatorPage.stats.downloads"),
                    value: stats?.total_downloads ?? 0,
                  },
                  {
                    label: t("creatorPage.stats.likes"),
                    value: stats?.total_likes ?? 0,
                  },
                  {
                    label: t("creatorPage.stats.visits"),
                    value: stats?.total_visits ?? 0,
                  },
                ].map((stat) => (
                  <GlassCard key={stat.label} className="bg-white/10 p-4 text-white transition-all duration-200 hover:border-white/80 hover:bg-white/20">
                    <p className="text-xs uppercase tracking-[0.3em] text-white/70">
                      {stat.label}
                    </p>
                    <p className="mt-2 text-2xl font-semibold">
                      {numberFormatter.format(stat.value)}
                    </p>
                  </GlassCard>
                ))}
              </div>
            </div>
          </div>

          <section className="space-y-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400">
                {t("creatorPage.recentPrompts.title")}
              </p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-900 dark:text-white">
                {t("creatorPage.recentPrompts.subtitle")}
              </h2>
            </div>
            {highlightPrompts.length === 0 ? (
              <GlassCard className="p-6 text-sm text-slate-500 dark:text-slate-300">
                {t("creatorPage.recentPrompts.empty")}
              </GlassCard>
            ) : (
              <div className="grid gap-4 lg:grid-cols-2">
                {highlightPrompts.map((item) => (
                  <CreatorPromptCard
                    key={`highlight-${item.id}`}
                    item={item}
                    onSelect={handleSelectPrompt}
                  />
                ))}
              </div>
            )}
          </section>

          <section id="creator-prompts" className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400">
                  {t("creatorPage.allPromptsTitle")}
                </p>
                <h2 className="mt-2 text-xl font-semibold text-slate-900 dark:text-white">
                  {t("creatorPage.allPromptsSubtitle")}
                </h2>
              </div>
            </div>
            {promptListQuery.isLoading ? (
              <div className="flex min-h-[160px] items-center justify-center">
                <LoaderCircle className="h-6 w-6 animate-spin text-primary" />
              </div>
            ) : promptListQuery.isError ? (
              <GlassCard className="flex flex-col items-center gap-3 p-6 text-slate-500 dark:text-slate-300">
                <p>{t("creatorPage.listError")}</p>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    promptListQuery.refetch();
                  }}
                >
                  {t("creatorPage.retry")}
                </Button>
              </GlassCard>
            ) : (
              <>
                {promptListQuery.data && promptListQuery.data.items.length > 0 ? (
                  <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                    {promptListQuery.data.items.map((item) => (
                      <CreatorPromptCard
                        key={`list-${item.id}`}
                        item={item}
                        onSelect={handleSelectPrompt}
                      />
                    ))}
                  </div>
                ) : (
                  <GlassCard className="p-6 text-sm text-slate-500 dark:text-slate-300">
                    {t("creatorPage.gridEmpty")}
                  </GlassCard>
                )}
                <PaginationControls
                  page={page}
                  totalPages={promptListQuery.data?.meta.total_pages ?? 1}
                  currentCount={promptListQuery.data?.meta.current_count ?? 0}
                  onPrev={() => setPage((prev) => Math.max(prev - 1, 1))}
                  onNext={() =>
                    setPage((prev) =>
                      Math.min(
                        prev + 1,
                        promptListQuery.data?.meta.total_pages ?? prev + 1,
                      ),
                    )
                  }
                  prevLabel={t("publicPrompts.prevPage")}
                  nextLabel={t("publicPrompts.nextPage")}
                  pageLabel={t("publicPrompts.paginationInfo", {
                    page,
                    totalPages: promptListQuery.data?.meta.total_pages ?? 1,
                  })}
                  countLabel={t("publicPrompts.paginationCount", {
                    count: promptListQuery.data?.meta.current_count ?? 0,
                  })}
                />
              </>
            )}
          </section>
        </>
      )}
      <PromptDetailModal
        open={detailModalOpen}
        detail={selectedDetail ?? null}
        isLoading={isDetailLoading}
        isError={detailError}
        statusMeta={detailStatusMeta}
        updatedAt={detailUpdatedAt}
        onClose={handleCloseDetail}
        onRetry={() => detailQuery.refetch()}
        onCopyBody={handleCopyBody}
        headerActions={(detail) => (
          <>
            <div className="inline-flex h-10 items-center gap-2 rounded-full border border-slate-200 px-4 text-slate-500 dark:border-slate-700 dark:text-slate-400">
              <Heart className="h-4 w-4 text-rose-500" />
              <span className="font-semibold text-slate-700 dark:text-slate-200">{detail.like_count}</span>
            </div>
            <div className="inline-flex h-10 items-center gap-2 rounded-full border border-slate-200 px-4 text-slate-500 dark:border-slate-700 dark:text-slate-400">
              <Eye className="h-4 w-4 text-emerald-500" />
              <span className="font-semibold text-slate-700 dark:text-slate-200">{detail.visit_count}</span>
            </div>
            <div className="inline-flex h-10 items-center gap-2 rounded-full border border-slate-200 px-4 text-slate-500 dark:border-slate-700 dark:text-slate-400">
              <Download className="h-4 w-4 text-primary" />
              <span className="font-semibold text-slate-700 dark:text-slate-200">{detail.download_count}</span>
            </div>
          </>
        )}
      />
    </div>
  );
}

function CreatorPromptCard({
  item,
  onSelect,
}: {
  item: PublicPromptListItem;
  onSelect: (id: number) => void;
}): JSX.Element {
  const { t } = useTranslation();
  const summary = clampTextWithOverflow(item.summary ?? "", 140);
  const summaryText = formatOverflowLabel(summary.value, summary.overflow);
  return (
    <GlassCard
      role="button"
      tabIndex={0}
      className="group relative flex cursor-pointer flex-col gap-4 border border-white/60 bg-white/80 p-5 shadow-lg transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-[0_25px_45px_-20px_rgba(59,130,246,0.35)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/40 dark:border-slate-800/70 dark:bg-slate-900/70 dark:hover:border-primary/30"
      onClick={() => onSelect(item.id)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(item.id);
        }
      }}
    >
      <div className="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.35em] text-slate-400">
        <span>{item.topic}</span>
        <Badge className="rounded-full bg-primary/10 text-primary">
          {item.language.toUpperCase()}
        </Badge>
      </div>
      <div className="space-y-2">
        <h3 className="text-lg font-semibold text-slate-900 dark:text-white">{item.title}</h3>
        <p className="text-sm text-slate-500 dark:text-slate-400">{summaryText}</p>
      </div>
      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
        <span className="inline-flex items-center gap-1">
          <Download className="h-4 w-4 text-primary" />
          {item.download_count}
        </span>
        <span className="inline-flex items-center gap-1">
          <Heart className="h-4 w-4 text-rose-500" />
          {item.like_count}
        </span>
        <span className="inline-flex items-center gap-1">
          <Eye className="h-4 w-4 text-emerald-500" />
          {item.visit_count}
        </span>
      </div>
      <div className="flex items-center justify-between text-xs text-slate-400">
        <span>{new Date(item.updated_at).toLocaleDateString()}</span>
      </div>
    </GlassCard>
  );
}
