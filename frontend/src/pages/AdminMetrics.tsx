import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  ResponsiveContainer,
  LineChart,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  Line,
  ComposedChart,
  Bar,
} from "recharts";

import { PageHeader } from "../components/layout/PageHeader";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { useAuth } from "../hooks/useAuth";
import {
  fetchAdminMetrics,
  type AdminMetricsSnapshot,
} from "../lib/api";

interface ChartDatum {
  date: string;
  rawDate: string;
  activeUsers: number;
  generateRequests: number;
  successRate: number;
  saveRequests: number;
  avgLatency: number;
}

const REFRESH_INTERVAL_MS = 5 * 60 * 1000;

export default function AdminMetricsPage() {
  const { t, i18n } = useTranslation();
  const isAdmin = useAuth((state) => state.profile?.user?.is_admin ?? false);

  const metricsQuery = useQuery<AdminMetricsSnapshot>({
    queryKey: ["admin", "metrics"],
    queryFn: fetchAdminMetrics,
    refetchInterval: REFRESH_INTERVAL_MS,
  });

  const locale = i18n.language;

  const numberFormatter = useMemo(
    () => new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }),
    [locale],
  );
  const latencyFormatter = useMemo(
    () => new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }),
    [locale],
  );
  const percentFormatter = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: "percent",
        maximumFractionDigits: 1,
      }),
    [locale],
  );
  const dateFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat(locale, {
        month: "2-digit",
        day: "2-digit",
      }),
    [locale],
  );

  const snapshot = metricsQuery.data;

  const summaryItems = useMemo(() => {
    const totals = snapshot?.totals;
    return [
      {
        key: "dau",
        label: t("adminMetricsPage.cards.dau"),
        value: totals
          ? numberFormatter.format(totals.active_users ?? 0)
          : "—",
      },
      {
        key: "requests",
        label: t("adminMetricsPage.cards.requests"),
        value: totals
          ? numberFormatter.format(totals.generate_requests ?? 0)
          : "—",
      },
      {
        key: "successRate",
        label: t("adminMetricsPage.cards.successRate"),
        value: totals
          ? percentFormatter.format(totals.generate_success_rate ?? 0)
          : "—",
      },
      {
        key: "avgLatency",
        label: t("adminMetricsPage.cards.avgLatency"),
        value: totals
          ? latencyFormatter.format(Math.round(totals.average_latency_ms ?? 0))
          : "—",
        suffix: totals ? "ms" : "",
      },
    ];
  }, [snapshot, t, numberFormatter, percentFormatter, latencyFormatter]);

  const chartData = useMemo<ChartDatum[]>(() => {
    if (!snapshot?.daily?.length) {
      return [];
    }
    return snapshot.daily.map((item) => {
      const rawDate = item.date ?? "";
      let dateLabel = rawDate;
      const parsed = rawDate ? new Date(`${rawDate}T00:00:00Z`) : null;
      if (parsed && !Number.isNaN(parsed.getTime())) {
        dateLabel = dateFormatter.format(parsed);
      }
      return {
        date: dateLabel,
        rawDate,
        activeUsers: item.active_users ?? 0,
        generateRequests: item.generate_requests ?? 0,
        successRate: item.generate_success_rate ?? 0,
        saveRequests: item.save_requests ?? 0,
        avgLatency: item.average_latency_ms ?? 0,
      };
    });
  }, [snapshot, dateFormatter]);

  const rangeDays = snapshot?.range_days ?? chartData.length;

  const refreshedLabel = useMemo(() => {
    const raw = snapshot?.refreshed_at;
    if (!raw) {
      return "";
    }
    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) {
      return raw;
    }
    return parsed.toLocaleString(locale);
  }, [snapshot?.refreshed_at, locale]);

  if (!isAdmin) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          eyebrow={t("adminMetricsPage.eyebrow")}
          title={t("adminMetricsPage.title")}
          description={t("adminMetricsPage.subtitle")}
        />
        <GlassCard className="flex flex-1 flex-col items-center justify-center gap-3 text-slate-500">
          <span className="text-lg font-semibold text-slate-600">
            {t("adminMetricsPage.accessDeniedTitle")}
          </span>
          <p className="text-sm text-slate-500">
            {t("ipGuardPage.noPermission.subtitle")}
          </p>
        </GlassCard>
      </div>
    );
  }

  let content: JSX.Element;

  if (metricsQuery.isLoading) {
    content = (
      <GlassCard className="flex h-64 items-center justify-center text-sm text-slate-500 dark:text-slate-400">
        {t("common.loading")}
      </GlassCard>
    );
  } else if (metricsQuery.isError) {
    const message =
      metricsQuery.error instanceof Error
        ? metricsQuery.error.message
        : t("errors.generic");
    content = (
      <GlassCard className="flex flex-col gap-4 p-6 text-slate-500 dark:text-slate-400">
        <span className="text-sm dark:text-slate-300">{message}</span>
        <Button
          variant="secondary"
          onClick={() => metricsQuery.refetch()}
        >
          {t("common.retry")}
        </Button>
      </GlassCard>
    );
  } else if (chartData.length === 0) {
    content = (
      <GlassCard className="flex h-64 flex-col items-center justify-center gap-2 text-slate-500 dark:text-slate-400">
        <span className="text-sm dark:text-slate-300">{t("adminMetricsPage.empty")}</span>
        <Button
          variant="secondary"
          onClick={() => metricsQuery.refetch()}
        >
          {t("adminMetricsPage.refreshButton")}
        </Button>
      </GlassCard>
    );
  } else {
    content = (
      <div className="flex flex-col gap-6">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {summaryItems.map((item) => (
            <GlassCard key={item.key} className="p-5">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
                {item.label}
              </span>
              <div className="mt-3 flex items-baseline gap-1">
                <span className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
                  {item.value}
                </span>
                {item.suffix ? (
                  <span className="text-xs text-slate-400 dark:text-slate-500">{item.suffix}</span>
                ) : null}
              </div>
            </GlassCard>
          ))}
        </div>

        <GlassCard className="p-6">
          <div className="flex flex-col gap-3">
            <div>
              <h3 className="text-base font-semibold text-slate-800 dark:text-slate-100">
                {t("adminMetricsPage.charts.activity.title")}
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                {t("adminMetricsPage.charts.activity.description", {
                  days: rangeDays,
                })}
              </p>
            </div>
            <div className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                  <XAxis dataKey="date" stroke="#94a3b8" />
                  <YAxis
                    yAxisId="left"
                    allowDecimals={false}
                    stroke="#94a3b8"
                    tickFormatter={(value) => numberFormatter.format(value)}
                  />
                  <YAxis
                    yAxisId="right"
                    orientation="right"
                    stroke="#94a3b8"
                    tickFormatter={(value) => percentFormatter.format(value)}
                    domain={[0, 1]}
                  />
                  <Tooltip
                    cursor={{ strokeDasharray: "3 3" }}
                    labelFormatter={(label, payload) => {
                      const raw = payload?.[0]?.payload?.rawDate;
                      return raw ?? label;
                    }}
                    formatter={(value: any, _name, entry) => {
                      if (typeof value !== "number") {
                        return value;
                      }
                      const key = entry?.dataKey as keyof ChartDatum;
                      if (key === "successRate") {
                        return [percentFormatter.format(value), entry.name];
                      }
                      return [numberFormatter.format(value), entry.name];
                    }}
                  />
                  <Legend />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="activeUsers"
                    name={t("adminMetricsPage.charts.activity.series.activeUsers")}
                    stroke="#6366f1"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="generateRequests"
                    name={t("adminMetricsPage.charts.activity.series.requests")}
                    stroke="#0ea5e9"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    yAxisId="right"
                    type="monotone"
                    dataKey="successRate"
                    name={t("adminMetricsPage.charts.activity.series.successRate")}
                    stroke="#10b981"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>
        </GlassCard>

        <GlassCard className="p-6">
          <div className="flex flex-col gap-3">
            <div>
              <h3 className="text-base font-semibold text-slate-800 dark:text-slate-100">
                {t("adminMetricsPage.charts.persistence.title")}
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                {t("adminMetricsPage.charts.persistence.description", {
                  days: rangeDays,
                })}
              </p>
            </div>
            <div className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <ComposedChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                  <XAxis dataKey="date" stroke="#94a3b8" />
                  <YAxis
                    yAxisId="left"
                    allowDecimals={false}
                    stroke="#94a3b8"
                    tickFormatter={(value) => numberFormatter.format(value)}
                  />
                  <YAxis
                    yAxisId="right"
                    orientation="right"
                    stroke="#94a3b8"
                    tickFormatter={(value) => latencyFormatter.format(value)}
                  />
                  <Tooltip
                    cursor={{ strokeDasharray: "3 3" }}
                    labelFormatter={(label, payload) => {
                      const raw = payload?.[0]?.payload?.rawDate;
                      return raw ?? label;
                    }}
                    formatter={(value: any, _name, entry) => {
                      if (typeof value !== "number") {
                        return value;
                      }
                      const key = entry?.dataKey as keyof ChartDatum;
                      if (key === "avgLatency") {
                        return [
                          `${latencyFormatter.format(Math.round(value))} ms`,
                          entry.name,
                        ];
                      }
                      return [numberFormatter.format(value), entry.name];
                    }}
                  />
                  <Legend />
                  <Bar
                    yAxisId="left"
                    dataKey="saveRequests"
                    name={t("adminMetricsPage.charts.persistence.series.saveRequests")}
                    fill="#f97316"
                    radius={[6, 6, 0, 0]}
                  />
                  <Line
                    yAxisId="right"
                    type="monotone"
                    dataKey="avgLatency"
                    name={t("adminMetricsPage.charts.persistence.series.avgLatency")}
                    stroke="#ec4899"
                    strokeWidth={2}
                    dot={false}
                  />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          </div>
        </GlassCard>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow={t("adminMetricsPage.eyebrow")}
        title={t("adminMetricsPage.title")}
        description={t("adminMetricsPage.subtitle")}
      />
      {refreshedLabel ? (
        <span className="text-xs text-slate-400 dark:text-slate-500">
          {t("adminMetricsPage.lastUpdated", { time: refreshedLabel })}
        </span>
      ) : null}
      {content}
    </div>
  );
}
