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

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow={t("logsPage.eyebrow")}
        title={t("logsPage.title")}
        description={t("logsPage.subtitle")}
      />
      <div className="grid gap-4">
        {entries.length === 0 ? (
          <GlassCard className="text-sm text-slate-500">
            {entriesQuery.isLoading
              ? t("ipGuardPage.states.loading")
              : t("logsPage.empty")}
          </GlassCard>
        ) : (
          entries.map((entry: ChangelogEntry) => (
            <GlassCard
              key={entry.id}
              className="space-y-4 border-white/50 bg-white/60 dark:border-slate-800 dark:bg-slate-900/50"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex flex-col gap-1">
                  <Badge
                    className="w-fit rounded-lg border-transparent bg-gradient-to-r from-primary/80 via-primary/70 to-primary/60 px-3 py-1 text-[11px] font-semibold text-white shadow-sm"
                  >
                    {entry.badge}
                  </Badge>
                  <h2 className="text-xl font-semibold text-slate-800 dark:text-slate-100">
                    {entry.title}
                  </h2>
                  <p className="text-sm text-slate-500 dark:text-slate-300">
                    {entry.summary}
                  </p>
                </div>
                <span className="text-xs text-slate-400">
                  {new Date(entry.published_at).toLocaleDateString()}
                </span>
              </div>
              <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-200">
                {entry.items.map((item, index) => (
                  <li key={`${entry.id}-${index}`} className="flex items-start gap-2">
                    <span className="mt-1 h-2 w-2 rounded-full bg-primary/80" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </GlassCard>
          ))
        )}
      </div>
    </div>
  );
}
