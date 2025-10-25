/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 13:22:30
 * @FilePath: \electron-go-app\frontend\src\pages\Logs.tsx
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Badge } from "../components/ui/badge";
import { GlassCard } from "../components/ui/glass-card";
import { fetchChangelogEntries, type ChangelogEntry } from "../lib/api";
import { PageHeader } from "../components/layout/PageHeader";

export default function LogsPage() {
  const { t, i18n } = useTranslation();
  const locale = useMemo(
    () => (i18n.language.startsWith("zh") ? "zh-CN" : "en"),
    [i18n.language],
  );

  const entriesQuery = useQuery({
    queryKey: ["changelog", locale],
    queryFn: () => fetchChangelogEntries(locale),
    staleTime: 1000 * 60 * 5,
  });

  const entries = entriesQuery.data ?? [];
  const isLoading = entriesQuery.isLoading;

  return (
    <div className="space-y-10 text-slate-700 transition-colors dark:text-slate-200">
      <PageHeader
        eyebrow={t("logsPage.eyebrow")}
        title={t("logsPage.title")}
        description={t("logsPage.subtitle")}
      />
      <div className="space-y-6">
        {isLoading && entries.length === 0
          ? Array.from({ length: 3 }).map((_, index) => (
              <GlassCard
                key={`changelog-skeleton-${index}`}
                className="relative overflow-hidden border-dashed border-slate-200 bg-white/70 p-6 dark:border-slate-800/60 dark:bg-slate-900/60"
              >
                <div className="absolute inset-0 animate-pulse bg-gradient-to-br from-slate-100/70 via-white/30 to-slate-50/20 dark:from-slate-800/40 dark:via-slate-900/30 dark:to-slate-800/30" />
                <div className="relative space-y-4">
                  <div className="h-4 w-24 rounded bg-slate-200/80 dark:bg-slate-700/60" />
                  <div className="h-6 w-3/4 rounded bg-slate-200/80 dark:bg-slate-700/60" />
                  <div className="h-3 w-full rounded bg-slate-200/70 dark:bg-slate-700/50" />
                  <div className="space-y-2">
                    <div className="h-3 w-5/6 rounded bg-slate-200/70 dark:bg-slate-700/50" />
                    <div className="h-3 w-2/3 rounded bg-slate-200/70 dark:bg-slate-700/50" />
                    <div className="h-3 w-4/5 rounded bg-slate-200/70 dark:bg-slate-700/50" />
                  </div>
                </div>
              </GlassCard>
            ))
          : null}

        {entries.length > 0
          ? entries.map((entry: ChangelogEntry) => (
              <GlassCard
                key={entry.id}
                className="relative overflow-hidden border-white/60 bg-white/85 p-6 shadow-xl backdrop-blur-xl transition-all dark:border-slate-800/60 dark:bg-slate-900/70"
              >
                <div className="pointer-events-none absolute inset-0">
                  <div className="absolute -top-24 left-12 h-48 w-48 rounded-full bg-gradient-to-br from-primary/25 via-sky-300/20 to-transparent blur-[120px]" />
                  <div className="absolute -bottom-36 right-10 h-56 w-56 rounded-full bg-gradient-to-tr from-rose-200/25 via-violet-200/20 to-transparent blur-[140px]" />
                  <div className="absolute inset-0 bg-noise opacity-[0.06]" />
                </div>
                <div className="relative flex flex-col gap-5">
                  <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                    <div className="space-y-2">
                      <Badge
                        className="w-fit rounded-full border-transparent bg-gradient-to-r from-primary/80 via-primary/70 to-primary/60 px-3 py-1 text-[11px] font-semibold text-white shadow-sm"
                      >
                        {entry.badge}
                      </Badge>
                      <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">
                        {entry.title}
                      </h2>
                      <p className="max-w-2xl text-sm text-slate-500 dark:text-slate-300">
                        {entry.summary}
                      </p>
                    </div>
                    <span className="whitespace-nowrap text-xs uppercase tracking-[0.3em] text-slate-400 dark:text-slate-500">
                      {new Date(entry.published_at).toLocaleDateString()}
                    </span>
                  </div>
                  <ul className="space-y-3 text-sm leading-relaxed text-slate-600 dark:text-slate-200">
                    {entry.items.map((item, itemIndex) => (
                      <li key={`${entry.id}-${itemIndex}`} className="flex items-start gap-3">
                        <span className="mt-[7px] h-2 w-2 rounded-full bg-primary/70" />
                        <span>{item}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </GlassCard>
            ))
          : null}

        {!isLoading && entries.length === 0 ? (
          <GlassCard className="border-dashed border-slate-200 bg-white/70 text-sm text-slate-500 dark:border-slate-800/60 dark:bg-slate-900/60 dark:text-slate-400">
            {t("logsPage.empty")}
          </GlassCard>
        ) : null}
      </div>
    </div>
  );
}
