import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { LoaderCircle, RefreshCcw } from "lucide-react";

import { PageHeader } from "../components/layout/PageHeader";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { PaginationControls } from "../components/ui/pagination-controls";
import { useAuth } from "../hooks/useAuth";
import {
  fetchAdminUsers,
  type AdminUserOverviewItem,
  type AdminUserOverviewResponse,
} from "../lib/api";
import { cn } from "../lib/utils";
import { SpotlightSearch } from "../components/ui/spotlight-search";

const SEARCH_DEBOUNCE_MS = 280;

export default function AdminUsersPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const profile = useAuth((state) => state.profile);
  const isAdmin = Boolean(profile?.user?.is_admin);

  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedSearch(searchInput.trim());
      setPage(1);
    }, SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(handle);
  }, [searchInput]);

  const overviewQuery = useQuery<AdminUserOverviewResponse>({
    queryKey: ["admin-users", page, debouncedSearch],
    enabled: isAdmin,
    queryFn: () =>
      fetchAdminUsers({
        page,
        query: debouncedSearch ? debouncedSearch : undefined,
      }),
    placeholderData: (previous) => previous,
  });

  const overview = overviewQuery.data;
  const items: AdminUserOverviewItem[] = overview?.items ?? [];

  const locale = i18n.language;
  const numberFormatter = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        maximumFractionDigits: 0,
      }),
    [locale],
  );
  const minutesFormatter = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        maximumFractionDigits: 1,
      }),
    [locale],
  );
  const dateTimeFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat(locale, {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      }),
    [locale],
  );

  const formatDateTime = useCallback(
    (value?: string | null) => {
      if (!value) {
        return t("adminUsersPage.table.never");
      }
      const parsed = new Date(value);
      if (Number.isNaN(parsed.getTime())) {
        return value;
      }
      return dateTimeFormatter.format(parsed);
    },
    [dateTimeFormatter, t],
  );

  const summary = useMemo(() => {
    let online = 0;
    let promptTotal = 0;
    let published = 0;
    items.forEach((item) => {
      if (item.is_online) {
        online += 1;
      }
      promptTotal += item.prompt_totals?.total ?? 0;
      published += item.prompt_totals?.published ?? 0;
    });
    return { online, promptTotal, published };
  }, [items]);

  const totalUsers = overview?.total ?? items.length;
  const statusLabels = useMemo(
    () => ({
      draft: t("myPrompts.statusFilter.draft"),
      published: t("myPrompts.statusFilter.published"),
      archived: t("myPrompts.statusFilter.archived"),
    }),
    [t],
  );

  const thresholdSeconds = overview?.online_threshold_seconds ?? 0;
  const thresholdMinutes =
    thresholdSeconds > 0 ? thresholdSeconds / 60 : thresholdSeconds;
  const thresholdLabel = minutesFormatter.format(thresholdMinutes);

  const effectivePageSize =
    overview?.page_size && overview.page_size > 0
      ? overview.page_size
      : items.length || 1;
  const totalPages = Math.max(
    1,
    Math.ceil(((overview?.total ?? items.length) || 1) / effectivePageSize),
  );

  const handleClearSearch = useCallback(() => {
    setSearchInput("");
    setDebouncedSearch("");
    setPage(1);
  }, []);

  const buildStatusBadgeClass = useCallback(
    (status: string) => {
      switch (status) {
        case "published":
          return "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200";
        case "draft":
          return "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-100";
        case "archived":
          return "bg-slate-200 text-slate-600 dark:bg-slate-700/60 dark:text-slate-300";
        default:
          return "bg-slate-200 text-slate-600 dark:bg-slate-700/60 dark:text-slate-300";
      }
    },
    [],
  );

  if (!isAdmin) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          eyebrow={t("adminUsersPage.eyebrow")}
          title={t("adminUsersPage.title")}
          description={t("adminUsersPage.subtitle")}
        />
        <GlassCard className="flex flex-1 flex-col items-center justify-center gap-3 text-slate-500 dark:text-slate-300">
          <span className="text-lg font-semibold text-slate-600 dark:text-slate-200">
            {t("adminUsersPage.accessDeniedTitle")}
          </span>
          <p className="text-sm">{t("ipGuardPage.noPermission.subtitle")}</p>
        </GlassCard>
      </div>
    );
  }

  if (overviewQuery.isLoading && !overviewQuery.data) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          eyebrow={t("adminUsersPage.eyebrow")}
          title={t("adminUsersPage.title")}
          description={t("adminUsersPage.subtitle")}
        />
        <GlassCard className="flex h-64 items-center justify-center text-sm text-slate-500 dark:text-slate-400">
          {t("common.loading")}
        </GlassCard>
      </div>
    );
  }

  if (overviewQuery.isError) {
    const message =
      overviewQuery.error instanceof Error
        ? overviewQuery.error.message
        : t("errors.generic");
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          eyebrow={t("adminUsersPage.eyebrow")}
          title={t("adminUsersPage.title")}
          description={t("adminUsersPage.subtitle")}
        />
        <GlassCard className="flex flex-col gap-4 p-6 text-slate-500 dark:text-slate-300">
          <span className="text-sm">{message}</span>
          <Button
            variant="secondary"
            onClick={() => overviewQuery.refetch()}
            className="w-fit"
          >
            <RefreshCcw className="mr-2 h-4 w-4" />
            {t("common.retry")}
          </Button>
        </GlassCard>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow={t("adminUsersPage.eyebrow")}
        title={t("adminUsersPage.title")}
        description={t("adminUsersPage.subtitle")}
      />

      <GlassCard className="flex flex-col gap-5 p-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex w-full flex-col gap-2 md:flex-row md:items-center md:gap-3">
            <SpotlightSearch
              className="w-full max-w-md"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder={t("adminUsersPage.searchPlaceholder")}
            />
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleClearSearch}
                disabled={!searchInput}
              >
                {t("adminUsersPage.actions.clear")}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => overviewQuery.refetch()}
                disabled={overviewQuery.isFetching}
              >
                {overviewQuery.isFetching ? (
                  <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <RefreshCcw className="mr-2 h-4 w-4" />
                )}
                {t("adminUsersPage.actions.refresh")}
              </Button>
            </div>
          </div>
          <span className="text-xs text-slate-500 dark:text-slate-400">
            {t("adminUsersPage.meta.onlineThreshold", {
              minutes: thresholdLabel,
            })}
          </span>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <SummaryCard
            label={t("adminUsersPage.summary.totalUsers")}
            value={numberFormatter.format(totalUsers)}
          />
          <SummaryCard
            label={t("adminUsersPage.summary.onlineUsers")}
            value={numberFormatter.format(summary.online)}
          />
          <SummaryCard
            label={t("adminUsersPage.summary.totalPrompts")}
            value={numberFormatter.format(summary.promptTotal)}
          />
          <SummaryCard
            label={t("adminUsersPage.summary.publishedPrompts")}
            value={numberFormatter.format(summary.published)}
          />
        </div>
      </GlassCard>

      <GlassCard className="p-0">
        {items.length === 0 ? (
          <div className="flex h-60 flex-col items-center justify-center gap-2 text-sm text-slate-500 dark:text-slate-400">
            {t("adminUsersPage.table.empty")}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-white/60 text-sm dark:divide-slate-800">
              <thead className="bg-white/80 text-left text-xs font-semibold uppercase tracking-[0.24em] text-slate-400 dark:bg-slate-900/70">
                <tr>
                  <th className="px-6 py-4">{t("adminUsersPage.table.username")}</th>
                  <th className="px-4 py-4">{t("adminUsersPage.table.email")}</th>
                  <th className="px-4 py-4">{t("adminUsersPage.table.online")}</th>
                  <th className="px-4 py-4">{t("adminUsersPage.table.lastLogin")}</th>
                  <th className="px-4 py-4">
                    {t("adminUsersPage.table.promptOverview")}
                  </th>
                  <th className="px-4 py-4">
                    {t("adminUsersPage.table.recentPrompt")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/60 dark:divide-slate-800">
                {items.map((item) => {
                  const onlineBadge = item.is_online
                    ? cn(
                        "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200",
                      )
                    : cn(
                        "bg-slate-200 text-slate-600 dark:bg-slate-700/60 dark:text-slate-300",
                      );
                  return (
                    <tr
                      key={item.id}
                      className="bg-white/60 transition-colors hover:bg-white/80 dark:bg-slate-900/40 dark:hover:bg-slate-900/60"
                    >
                      <td className="px-6 py-4">
                        <div className="flex flex-col">
                          <span className="font-medium text-slate-700 dark:text-slate-200">
                            {item.username || "-"}
                          </span>
                          <span className="text-xs text-slate-400">
                            #{item.id}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-4 text-slate-600 dark:text-slate-300">
                        {item.email || "-"}
                      </td>
                      <td className="px-4 py-4">
                        <Badge className={onlineBadge}>
                          {item.is_online
                            ? t("adminUsersPage.table.onlineLabel")
                            : t("adminUsersPage.table.offline")}
                        </Badge>
                      </td>
                      <td className="px-4 py-4 text-slate-600 dark:text-slate-300">
                        {formatDateTime(item.last_login_at)}
                      </td>
                      <td className="px-4 py-4">
                        <div className="space-y-1 text-xs text-slate-500 dark:text-slate-400">
                          <div className="flex items-center gap-2 font-medium text-slate-700 dark:text-slate-100">
                            {t("adminUsersPage.promptTotals.total")}:{" "}
                            {numberFormatter.format(
                              item.prompt_totals?.total ?? 0,
                            )}
                          </div>
                          <div className="flex flex-wrap gap-3">
                            <span>
                              {t("adminUsersPage.promptTotals.draft")}:{" "}
                              {numberFormatter.format(
                                item.prompt_totals?.draft ?? 0,
                              )}
                            </span>
                            <span>
                              {t("adminUsersPage.promptTotals.published")}:{" "}
                              {numberFormatter.format(
                                item.prompt_totals?.published ?? 0,
                              )}
                            </span>
                            <span>
                              {t("adminUsersPage.promptTotals.archived")}:{" "}
                              {numberFormatter.format(
                                item.prompt_totals?.archived ?? 0,
                              )}
                            </span>
                          </div>
                          <div className="text-xs text-slate-400">
                            {t("adminUsersPage.table.lastPromptUpdated", {
                              time: formatDateTime(item.latest_prompt_at),
                            })}
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-4">
                        {item.recent_prompts.length === 0 ? (
                          <span className="text-xs text-slate-400">
                            {t("adminUsersPage.table.noRecentPrompts")}
                          </span>
                        ) : (
                          <ul className="space-y-2 text-xs text-slate-500 dark:text-slate-300">
                            {item.recent_prompts.map((prompt) => (
                              <li key={prompt.id}>
                                <div className="flex flex-col gap-1">
                                  <span className="font-medium text-slate-700 dark:text-slate-100">
                                    {prompt.topic || t("common.untitled")}
                                  </span>
                                  <div className="flex items-center gap-2">
                                    <Badge
                                      className={cn(
                                        "border-transparent",
                                        buildStatusBadgeClass(prompt.status),
                                      )}
                                    >
                                      {statusLabels[
                                        prompt.status as keyof typeof statusLabels
                                      ] ?? prompt.status}
                                    </Badge>
                                    <span>
                                      {t("adminUsersPage.table.recentUpdatedAt", {
                                        time: formatDateTime(prompt.updated_at),
                                      })}
                                    </span>
                                  </div>
                                </div>
                              </li>
                            ))}
                          </ul>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </GlassCard>

      <PaginationControls
        page={page}
        totalPages={totalPages}
        currentCount={items.length}
        onPrev={() => setPage((current) => Math.max(1, current - 1))}
        onNext={() => setPage((current) => Math.min(totalPages, current + 1))}
        prevLabel={t("adminUsersPage.pagination.prev")}
        nextLabel={t("adminUsersPage.pagination.next")}
        pageLabel={t("adminUsersPage.pagination.page", {
          page,
          total: totalPages,
        })}
        countLabel={t("adminUsersPage.pagination.count", {
          count: items.length,
        })}
      />
    </div>
  );
}

interface SummaryCardProps {
  label: string;
  value: string;
}

function SummaryCard({ label, value }: SummaryCardProps) {
  return (
    <div className="flex flex-col gap-1 rounded-2xl border border-white/60 bg-white/75 px-5 py-4 text-slate-600 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200">
      <span className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
        {label}
      </span>
      <span className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
        {value}
      </span>
    </div>
  );
}
