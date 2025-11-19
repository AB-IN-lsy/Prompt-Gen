import { CSSProperties, useEffect, useMemo, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { toast } from "sonner";
import {
  CornerUpLeft,
  Edit3,
  History,
  LoaderCircle,
  Share2,
  SlidersHorizontal,
  UploadCloud,
  X,
} from "lucide-react";

import { PageHeader } from "../components/layout/PageHeader";
import { GlassCard } from "../components/ui/glass-card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Input } from "../components/ui/input";
import { Textarea } from "../components/ui/textarea";
import { Select } from "../components/ui/select";
import { MoreActionsMenu, type MoreActionItem } from "../components/ui/more-actions-menu";
import {
  fetchPromptDetail,
  fetchPromptVersion,
  fetchPromptVersions,
  normaliseKeywordSource,
  submitPublicPrompt,
  sharePrompt,
  type SharePromptResponse,
  type PublicPromptSubmitPayload,
  type PublicPromptSubmitResult,
  type KeywordSource,
  PromptDetailResponse,
  PromptListKeyword,
  type Keyword,
  type PromptVersionSummary,
  type PromptVersionDetail,
} from "../lib/api";
import { cn, clampTextWithOverflow, formatOverflowLabel } from "../lib/utils";
import {
  PROMPT_KEYWORD_MAX_LENGTH,
  PROMPT_TAG_MAX_LENGTH,
  DEFAULT_KEYWORD_WEIGHT,
  PROMPT_GENERATE_TEMPERATURE_DEFAULT,
  PROMPT_GENERATE_TOP_P_DEFAULT,
  PROMPT_GENERATE_MAX_OUTPUT_DEFAULT,
} from "../config/prompt";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import { isLocalMode } from "../lib/runtimeMode";

const summaryParagraphLimitRaw =
  import.meta.env.VITE_PUBLIC_PROMPT_SUMMARY_PARAGRAPHS ?? "2";
const parsedSummaryLimit = Number.parseInt(summaryParagraphLimitRaw, 10);
const SUMMARY_PARAGRAPH_LIMIT = Number.isNaN(parsedSummaryLimit)
  ? 2
  : parsedSummaryLimit;
const DEFAULT_PUBLIC_PROMPT_LANGUAGE =
  import.meta.env.VITE_PUBLIC_PROMPT_DEFAULT_LANGUAGE ?? "zh-CN";
const summaryRowsRaw = import.meta.env.VITE_PUBLIC_PROMPT_SUMMARY_ROWS ?? "4";
const parsedSummaryRows = Number.parseInt(summaryRowsRaw, 10);
const PUBLIC_PROMPT_SUMMARY_ROWS = Number.isNaN(parsedSummaryRows)
  ? 4
  : parsedSummaryRows;

export default function PromptDetailPage(): JSX.Element {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const promptId = Number.parseInt(id ?? "", 10);
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null);
  const offlineMode = isLocalMode();
  const [showSubmitPanel, setShowSubmitPanel] = useState(false);
  const [submitTitle, setSubmitTitle] = useState("");
  const [submitSummary, setSubmitSummary] = useState("");
  const [submitTagsInput, setSubmitTagsInput] = useState("");
  const [submitLanguage, setSubmitLanguage] = useState(
    DEFAULT_PUBLIC_PROMPT_LANGUAGE,
  );
  const [submitUseSelectedVersion, setSubmitUseSelectedVersion] =
    useState(false);
  const [shareDialogOpen, setShareDialogOpen] = useState(false);
  const [shareResult, setShareResult] = useState<SharePromptResponse | null>(null);

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

  const detailQuery = useQuery({
    queryKey: ["prompt-detail", promptId],
    enabled: Number.isInteger(promptId) && promptId > 0,
    queryFn: () => fetchPromptDetail(promptId),
  });

  const detail = detailQuery.data;
  const canSubmitPublic =
    !offlineMode && detail?.status === "published";

  const versionsQuery = useQuery<PromptVersionSummary[]>({
    queryKey: ["prompt-versions", promptId],
    enabled: Number.isInteger(promptId) && promptId > 0,
    queryFn: () => fetchPromptVersions(promptId),
    staleTime: 1000 * 60 * 5,
  });

  const versions = versionsQuery.data ?? [];

  useEffect(() => {
    if (selectedVersion === null) {
      return;
    }
    if (!versions.some((entry) => entry.versionNo === selectedVersion)) {
      setSelectedVersion(null);
    }
  }, [versions, selectedVersion]);

  const versionDetailQuery = useQuery<PromptVersionDetail>({
    queryKey: ["prompt-version", promptId, selectedVersion],
    enabled:
      Number.isInteger(promptId) && promptId > 0 && selectedVersion !== null,
    queryFn: () => fetchPromptVersion(promptId, selectedVersion ?? 0),
  });

  const versionDetail = versionDetailQuery.data;

  const [showFullInstructions, setShowFullInstructions] = useState(false);
  const [profileDialogOpen, setProfileDialogOpen] = useState(false);
  const activeGenerationProfile = useMemo(() => {
    if (!detail) {
      return null;
    }
  const profileSource =
    selectedVersion && versionDetail?.generation_profile
      ? versionDetail.generation_profile
      : detail.generation_profile;
  if (!profileSource) {
    return null;
  }
  return {
    stepwiseReasoning: Boolean(profileSource.stepwise_reasoning),
    temperature:
      typeof profileSource.temperature === "number"
        ? profileSource.temperature
        : PROMPT_GENERATE_TEMPERATURE_DEFAULT,
    topP:
      typeof profileSource.top_p === "number"
        ? profileSource.top_p
        : PROMPT_GENERATE_TOP_P_DEFAULT,
    maxOutputTokens:
      typeof profileSource.max_output_tokens === "number"
        ? profileSource.max_output_tokens
        : PROMPT_GENERATE_MAX_OUTPUT_DEFAULT,
  };
  }, [detail, selectedVersion, versionDetail]);
  const decimalFormatter = useMemo(
    () =>
      new Intl.NumberFormat(i18n.language, {
        minimumFractionDigits: 0,
        maximumFractionDigits: 3,
      }),
    [i18n.language],
  );
  const integerFormatter = useMemo(
    () =>
      new Intl.NumberFormat(i18n.language, {
        maximumFractionDigits: 0,
      }),
    [i18n.language],
  );

  useEffect(() => {
    setShowFullInstructions(false);
  }, [selectedVersion, detail?.id]);

  useEffect(() => {
    if (!activeGenerationProfile) {
      setProfileDialogOpen(false);
    }
  }, [activeGenerationProfile]);

  const selectedVersionSummary = selectedVersion !== null ? versions.find((entry) => entry.versionNo === selectedVersion) : null;
  const latestVersionSummary = versions[0];
  const activeVersionNumber = selectedVersion !== null ? selectedVersion : latestVersionSummary?.versionNo ?? null;
  const activeVersionCreatedAtValue = selectedVersion !== null
    ? versionDetail?.created_at ?? selectedVersionSummary?.createdAt ?? null
    : latestVersionSummary?.createdAt ?? detail?.updated_at ?? detail?.created_at ?? null;

  const activeModel = selectedVersion !== null && versionDetail
    ? versionDetail.model || detail?.model || ""
    : detail?.model || "";

  const activeBodyRaw = (selectedVersion !== null && versionDetail ? versionDetail.body : detail?.body) ?? "";
  const normalizedBody = activeBodyRaw.replace(/\r\n/g, "\n").replace(/\n{3,}/g, "\n\n");

  const activeInstructionsRaw = (selectedVersion !== null && versionDetail ? versionDetail.instructions : detail?.instructions) ?? "";
  const activeInstructionsTrimmed = activeInstructionsRaw.trim();

  const activePositiveKeywords: PromptListKeyword[] =
    selectedVersion !== null && versionDetail
      ? versionDetail.positive_keywords ?? []
      : detail?.positive_keywords ?? [];

  const activeNegativeKeywords: PromptListKeyword[] =
    selectedVersion !== null && versionDetail
      ? versionDetail.negative_keywords ?? []
      : detail?.negative_keywords ?? [];

  const submitMutation = useMutation<
    PublicPromptSubmitResult,
    unknown,
    PublicPromptSubmitPayload
  >({
    mutationFn: (payload: PublicPromptSubmitPayload) => submitPublicPrompt(payload),
    onSuccess: () => {
      toast.success(t("promptDetail.publicSubmit.success"));
      setShowSubmitPanel(false);
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : t("promptDetail.publicSubmit.error");
      toast.error(message);
    },
  });

  const shareMutation = useMutation<SharePromptResponse, unknown, number>({
    mutationFn: (targetId: number) => sharePrompt(targetId),
    onSuccess: (data) => {
      setShareResult(data);
      setShareDialogOpen(true);
      toast.success(t("promptDetail.share.success"));
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : t("promptDetail.share.error");
      toast.error(message);
    },
  });

  const versionSelectValue = selectedVersion === null ? "" : String(selectedVersion);
  const showLoadButton = versions.length > 0;
  const loadButtonDisabled =
    selectedVersion === null || !versionDetail || versionDetailQuery.isLoading;

  const instructionsIsEmpty = activeInstructionsTrimmed.length === 0;
  const hasLongInstructions = activeInstructionsTrimmed.length > 160;
  const instructionsStyle: CSSProperties | undefined =
    !showFullInstructions && hasLongInstructions
      ? {
          display: "-webkit-box",
          WebkitLineClamp: 3,
          WebkitBoxOrient: "vertical",
          overflow: "hidden",
        }
      : undefined;

  const bodyIsEmpty = normalizedBody.trim().length === 0;
  const markdownContent = useMemo(() => normalizedBody, [normalizedBody]);

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

    const versionLabel =
      activeVersionNumber !== null
        ? t("promptDetail.versions.versionLabel", { version: activeVersionNumber })
        : t("promptDetail.versions.latestLabel");

    const versionCreatedLabel = activeVersionCreatedAtValue
      ? formatDate(activeVersionCreatedAtValue)
      : "—";

    return [
      {
        label: t("promptDetail.meta.status"),
        value: t(`myPrompts.statusBadge.${detail.status}`),
      },
      {
        label: t("promptDetail.meta.model"),
        value: activeModel || "—",
      },
      {
        label: t("promptDetail.meta.version"),
        value: versionLabel,
      },
      {
        label: t("promptDetail.meta.versionCreatedAt"),
        value: versionCreatedLabel,
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
  }, [activeModel, activeVersionCreatedAtValue, activeVersionNumber, detail, i18n.language, t]);

  const versionTimestampFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat(i18n.language, {
        dateStyle: "medium",
        timeStyle: "short",
      }),
    [i18n.language],
  );

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

  const handleCopyPlain = () => copyBody(activeBodyRaw, "promptDetail.copy.bodySuccess");
  const handleCopyMarkdown = () => {
    copyBody(activeBodyRaw, "promptDetail.copy.markdownSuccess");
  };

  const handleEdit = () => {
    if (!detail) return;
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
    navigate("/prompt-workbench");
  };

  const handleLoadVersionToWorkbench = () => {
    if (!detail || !versionDetail) {
      return;
    }
    resetWorkbench();
    setTopic(detail.topic);
    setPrompt(versionDetail.body);
    setModel(versionDetail.model || detail.model);
    setPromptId(String(detail.id));
    setWorkspaceToken(detail.workspace_token ?? null);
    setInstructions(versionDetail.instructions ?? detail.instructions ?? "");
    const positive = mapKeywords(
      versionDetail.positive_keywords ?? [],
      "positive",
    );
    const negative = mapKeywords(
      versionDetail.negative_keywords ?? [],
      "negative",
    );
    setCollections(positive, negative);
    setTags(detail.tags ?? []);
    const profileSource =
      versionDetail.generation_profile ?? detail.generation_profile;
    if (profileSource) {
      overwriteGenerationProfile({
        stepwiseReasoning: Boolean(profileSource.stepwise_reasoning),
        temperature:
          typeof profileSource.temperature === "number"
            ? profileSource.temperature
            : PROMPT_GENERATE_TEMPERATURE_DEFAULT,
        topP:
          typeof profileSource.top_p === "number"
            ? profileSource.top_p
            : PROMPT_GENERATE_TOP_P_DEFAULT,
        maxOutputTokens:
          typeof profileSource.max_output_tokens === "number"
            ? profileSource.max_output_tokens
            : PROMPT_GENERATE_MAX_OUTPUT_DEFAULT,
      });
    }
    toast.success(
      t("promptDetail.versions.loadSuccess", {
        version: versionDetail.versionNo,
      }),
    );
    navigate("/prompt-workbench");
  };

const handleOpenSubmit = () => {
    if (!detail) {
      return;
    }
    if (detail.status !== "published") {
      toast.error(
        t("promptDetail.publicSubmit.onlyPublished", {
          defaultValue: "仅已发布的 Prompt 才能投稿至公共库",
        }),
      );
      return;
    }
    setSubmitTitle(detail.topic);
    setSubmitSummary(buildDefaultSummary(activeInstructionsRaw, activeBodyRaw));
    setSubmitTagsInput(detail.tags.join(", "));
    setSubmitLanguage(i18n.language || DEFAULT_PUBLIC_PROMPT_LANGUAGE);
    setSubmitUseSelectedVersion(selectedVersion !== null);
    setShowSubmitPanel(true);
  };

  const handleCloseSubmit = () => {
    setShowSubmitPanel(false);
  };

  const handleSubmitPublic = () => {
    if (!detail) {
      return;
    }
    const wantSelected = submitUseSelectedVersion && selectedVersion !== null;
    if (wantSelected && !versionDetail) {
      toast.error(t("promptDetail.publicSubmit.versionMissing"));
      return;
    }
    const sourceBlock = wantSelected && versionDetail ? versionDetail : null;
    const instructionsForSubmission =
      (sourceBlock?.instructions ?? detail.instructions ?? "").toString();
    const bodyForSubmission = sourceBlock?.body ?? detail.body;
    const positiveKeywordsForSubmission =
      sourceBlock?.positive_keywords ?? detail.positive_keywords ?? [];
    const negativeKeywordsForSubmission =
      sourceBlock?.negative_keywords ?? detail.negative_keywords ?? [];
    const summaryForSubmission =
      submitSummary.trim() ||
      buildDefaultSummary(instructionsForSubmission, bodyForSubmission);
    const tagsForSubmission = normaliseTagsInput(
      submitTagsInput,
      detail.tags ?? [],
    );

    const payload: PublicPromptSubmitPayload = {
      sourcePromptId: detail.id,
      title: submitTitle.trim() || detail.topic,
      topic: detail.topic,
      summary: summaryForSubmission,
      body: bodyForSubmission,
      instructions: instructionsForSubmission,
      positiveKeywords: serializeKeywordsForSubmission(
        positiveKeywordsForSubmission,
      ),
      negativeKeywords: serializeKeywordsForSubmission(
        negativeKeywordsForSubmission,
      ),
      tags: JSON.stringify(tagsForSubmission),
      model: sourceBlock?.model || detail.model,
      language:
        submitLanguage.trim() ||
        i18n.language ||
        DEFAULT_PUBLIC_PROMPT_LANGUAGE,
    };
    submitMutation.mutate(payload);
  };

  const handleShare = () => {
    if (!detail || shareMutation.isPending) {
      return;
    }
    shareMutation.mutate(detail.id);
  };

  const handleCopyShare = async () => {
    if (!shareResult?.payload) {
      toast.error(t("promptDetail.share.error"));
      return;
    }
    try {
      await navigator.clipboard.writeText(shareResult.payload);
      toast.success(t("promptDetail.share.copied"));
    } catch (error) {
      console.error("copy share payload failed", error);
      toast.error(t("promptDetail.copy.failed"));
    }
  };

  const moreActionsItems: MoreActionItem[] = [
    {
      key: "back",
      label: t("promptDetail.actions.back", { defaultValue: "返回上一页" }),
      icon: <CornerUpLeft className="h-4 w-4" />,
      onSelect: handleBack,
    },
    {
      key: "share",
      label: t("promptDetail.actions.share", { defaultValue: "分享" }),
      icon: shareMutation.isPending ? (
        <LoaderCircle className="h-4 w-4 animate-spin" />
      ) : (
        <Share2 className="h-4 w-4" />
      ),
      onSelect: handleShare,
      disabled: !detail || shareMutation.isPending,
    },
  ];

  if (activeGenerationProfile) {
    moreActionsItems.push({
      key: "generationProfile",
      label: t("promptDetail.actions.viewGenerationProfile", {
        defaultValue: "查看生成配置",
      }),
      icon: <SlidersHorizontal className="h-4 w-4" />,
      onSelect: () => setProfileDialogOpen(true),
    });
  }

  if (canSubmitPublic) {
    moreActionsItems.push({
      key: "submitPublic",
      label: t("promptDetail.actions.submitPublic"),
      icon: submitMutation.isPending ? (
        <LoaderCircle className="h-4 w-4 animate-spin" />
      ) : (
        <UploadCloud className="h-4 w-4" />
      ),
      onSelect: handleOpenSubmit,
      disabled: submitMutation.isPending || !detail,
    });
  }

  return (
    <div className="flex h-full flex-col gap-6">
      <PageHeader
        eyebrow={t("promptDetail.eyebrow")}
        title={detail ? detail.topic : t("promptDetail.title")}
        description={t("promptDetail.subtitle")}
        actions={
          <div className="flex w-full flex-wrap items-center gap-3">
            <div className="ml-auto flex flex-wrap items-center gap-3">
              <Button
                variant="secondary"
                onClick={handleEdit}
                disabled={!detail}
                className="transition-transform hover:-translate-y-0.5"
              >
                <Edit3 className="mr-2 h-4 w-4" />
                {t("promptDetail.actions.edit", { defaultValue: "继续编辑" })}
              </Button>
              <MoreActionsMenu
                triggerLabel={t("promptDetail.moreActionsLabel", {
                  defaultValue: "更多操作",
                })}
                triggerSize="default"
                triggerClassName="transition-transform hover:-translate-y-0.5"
                items={moreActionsItems}
              />
            </div>
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
          <div className="space-y-4 self-start">
            {showSubmitPanel ? (
              <GlassCard className="space-y-4 border border-primary/30 bg-white/80 shadow-[0_25px_45px_-30px_rgba(59,130,246,0.65)] dark:border-primary/40 dark:bg-slate-900/70">
                <div className="flex items-start justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-xs font-semibold uppercase tracking-[0.28em] text-primary">
                      {t("promptDetail.publicSubmit.title")}
                    </p>
                    <p className="text-sm text-slate-600 dark:text-slate-300">
                      {t("promptDetail.publicSubmit.description")}
                    </p>
                  </div>
                  <button
                    type="button"
                    aria-label={t("promptDetail.publicSubmit.cancel")}
                    className="rounded-full border border-transparent p-1 text-slate-400 transition hover:border-slate-200 hover:text-slate-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:text-slate-500 dark:hover:text-slate-300"
                    onClick={handleCloseSubmit}
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>

                <div className="space-y-2">
                  <p className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
                    {t("promptDetail.publicSubmit.versionLabel")}
                  </p>
                  <div className="flex flex-col gap-2">
                    <label
                      className={cn(
                        "flex items-center gap-3 rounded-xl border px-3 py-2 text-sm transition-colors dark:border-slate-700",
                        submitUseSelectedVersion
                          ? "border-slate-200 text-slate-500 dark:text-slate-400"
                          : "border-primary/40 bg-primary/10 text-primary dark:border-primary/40 dark:bg-primary/20 dark:text-primary-foreground",
                      )}
                    >
                      <input
                        type="radio"
                        name="public-submit-version"
                        className="sr-only"
                        checked={!submitUseSelectedVersion}
                        onChange={() => setSubmitUseSelectedVersion(false)}
                      />
                      <span>{t("promptDetail.publicSubmit.useLatest")}</span>
                    </label>
                    <label
                      className={cn(
                        "flex items-center gap-3 rounded-xl border px-3 py-2 text-sm transition-colors dark:border-slate-700",
                        submitUseSelectedVersion
                          ? "border-primary/40 bg-primary/10 text-primary dark:border-primary/40 dark:bg-primary/20 dark:text-primary-foreground"
                          : "border-slate-200 text-slate-500 dark:text-slate-400",
                      )}
                    >
                      <input
                        type="radio"
                        name="public-submit-version"
                        className="sr-only"
                        checked={submitUseSelectedVersion}
                        onChange={() => setSubmitUseSelectedVersion(true)}
                        disabled={versions.length === 0}
                      />
                      <span>
                        {versions.length === 0
                          ? t("promptDetail.publicSubmit.useSelectedDisabled")
                          : t("promptDetail.publicSubmit.useSelected")}
                      </span>
                    </label>
                  </div>
                  {submitUseSelectedVersion && selectedVersion !== null ? (
                    versionDetailQuery.isLoading ? (
                      <p className="text-xs text-amber-500 dark:text-amber-300">
                        {t("promptDetail.publicSubmit.versionLoading")}
                      </p>
                    ) : !versionDetail ? (
                      <p className="text-xs text-rose-500 dark:text-rose-400">
                        {t("promptDetail.publicSubmit.versionMissing")}
                      </p>
                    ) : (
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        {t("promptDetail.publicSubmit.versionReady", {
                          version: versionDetail.versionNo,
                        })}
                      </p>
                    )
            ) : null}
          </div>

                <div className="space-y-2">
                  <label
                    htmlFor="public-submit-title"
                    className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
                  >
                    {t("promptDetail.publicSubmit.titleLabel")}
                  </label>
                  <Input
                    id="public-submit-title"
                    value={submitTitle}
                    onChange={(event) => setSubmitTitle(event.target.value)}
                    placeholder={t("promptDetail.publicSubmit.titlePlaceholder")}
                  />
                </div>

                <div className="space-y-2">
                  <label
                    htmlFor="public-submit-summary"
                    className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
                  >
                    {t("promptDetail.publicSubmit.summaryLabel")}
                  </label>
                  <Textarea
                    id="public-submit-summary"
                    value={submitSummary}
                    onChange={(event) => setSubmitSummary(event.target.value)}
                    placeholder={t("promptDetail.publicSubmit.summaryPlaceholder")}
                    rows={PUBLIC_PROMPT_SUMMARY_ROWS}
                  />
                </div>

                <div className="space-y-2">
                  <label
                    htmlFor="public-submit-tags"
                    className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
                  >
                    {t("promptDetail.publicSubmit.tagsLabel")}
                  </label>
                  <Input
                    id="public-submit-tags"
                    value={submitTagsInput}
                    onChange={(event) => setSubmitTagsInput(event.target.value)}
                    placeholder={t("promptDetail.publicSubmit.tagsPlaceholder")}
                  />
                  <p className="text-xs text-slate-400 dark:text-slate-500">
                    {t("promptDetail.publicSubmit.tagsHint")}
                  </p>
                </div>

                <div className="space-y-2">
                  <label
                    htmlFor="public-submit-language"
                    className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
                  >
                    {t("promptDetail.publicSubmit.languageLabel")}
                  </label>
                  <Input
                    id="public-submit-language"
                    value={submitLanguage}
                    onChange={(event) => setSubmitLanguage(event.target.value)}
                    placeholder={DEFAULT_PUBLIC_PROMPT_LANGUAGE}
                  />
                </div>

                <div className="flex items-center justify-end gap-3">
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={handleCloseSubmit}
                    className="transition-transform hover:-translate-y-0.5"
                  >
                    {t("promptDetail.publicSubmit.cancel")}
                  </Button>
                  <Button
                    type="button"
                    onClick={handleSubmitPublic}
                    disabled={
                      submitMutation.isPending ||
                      (submitUseSelectedVersion &&
                        selectedVersion !== null &&
                        !versionDetail)
                    }
                    className="transition-transform hover:-translate-y-0.5"
                  >
                    {submitMutation.isPending ? (
                      <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <UploadCloud className="mr-2 h-4 w-4" />
                    )}
                    {t("promptDetail.publicSubmit.submit")}
                  </Button>
                </div>
              </GlassCard>
            ) : null}

            <GlassCard className="space-y-4">
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
              <p
                className="text-sm text-slate-600 dark:text-slate-300"
                style={instructionsStyle}
                title={instructionsIsEmpty ? undefined : activeInstructionsTrimmed}
              >
                {instructionsIsEmpty ? t("promptDetail.empty") : activeInstructionsTrimmed}
              </p>
              {hasLongInstructions ? (
                <button
                  type="button"
                  className="text-xs font-medium text-primary transition hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                  onClick={() => setShowFullInstructions((prev) => !prev)}
                >
                  {showFullInstructions
                    ? t("promptDetail.versions.collapse")
                    : t("promptDetail.versions.expand")}
                </button>
              ) : null}
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
                keywords={activePositiveKeywords}
              />
              <KeywordGroup
                title={t("promptDetail.keywords.negative")}
                polarity="negative"
                keywords={activeNegativeKeywords}
              />
            </section>

            <div className="h-px w-full bg-slate-200 dark:bg-slate-800" />

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="flex items-center gap-2 text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
                    <History className="h-3.5 w-3.5" aria-hidden="true" />
                    {t("promptDetail.versions.title")}
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {t("promptDetail.versions.subtitle")}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {versionsQuery.isLoading ? (
                    <LoaderCircle className="h-4 w-4 animate-spin text-slate-400" />
                  ) : null}
                  {showLoadButton ? (
                    <Button
                      size="sm"
                      variant="secondary"
                      className="whitespace-nowrap transition-transform hover:-translate-y-0.5"
                      onClick={handleLoadVersionToWorkbench}
                      disabled={loadButtonDisabled}
                    >
                      {t("promptDetail.versions.load")}
                    </Button>
                  ) : null}
                </div>
              </div>

            {versionsQuery.isError ? (
              <p className="text-sm text-rose-500 dark:text-rose-400">
                {t("promptDetail.versions.loadError")}
              </p>
            ) : versions.length === 0 ? (
              <p className="text-sm text-slate-500 dark:text-slate-400">
                {t("promptDetail.versions.empty")}
              </p>
            ) : (
              <div className="space-y-2">
                <label
                  htmlFor="prompt-version-select"
                  className="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500"
                >
                  {t("promptDetail.versions.selectorLabel")}
                </label>
                <Select
                  id="prompt-version-select"
                  value={versionSelectValue}
                  onChange={(event) => {
                    const next = event.target.value;
                    if (next === "") {
                      setSelectedVersion(null);
                      return;
                    }
                    const parsed = Number.parseInt(next, 10);
                    setSelectedVersion(Number.isNaN(parsed) ? null : parsed);
                  }}
                >
                  <option value="">
                    {t("promptDetail.versions.optionLatest")}
                  </option>
                  {versions.map((item) => (
                    <option key={item.versionNo} value={item.versionNo}>
                      {t("promptDetail.versions.versionOption", {
                        version: item.versionNo,
                        time: item.createdAt
                          ? versionTimestampFormatter.format(new Date(item.createdAt))
                          : t("promptDetail.versions.noTimestamp"),
                      })}
                    </option>
                  ))}
                </Select>
              </div>
            )}

              <div className="rounded-2xl border border-dashed border-slate-200 p-4 text-sm transition-colors dark:border-slate-700">
              {versionsQuery.isError ? (
                <p className="text-rose-500 dark:text-rose-400">
                  {t("promptDetail.versions.loadError")}
                </p>
              ) : versions.length === 0 ? (
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  {t("promptDetail.versions.empty")}
                </p>
              ) : selectedVersion === null ? (
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  {t("promptDetail.versions.previewLatest")}
                </p>
              ) : versionDetailQuery.isLoading ? (
                <div className="flex items-center gap-2 text-slate-500 dark:text-slate-400">
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                  {t("common.loading")}
                </div>
              ) : versionDetailQuery.isError || !versionDetail ? (
                  <p className="text-rose-500 dark:text-rose-400">
                    {t("promptDetail.versions.loadFailed")}
                  </p>
                ) : (
                  <div className="space-y-4">
                    <div className="flex flex-col gap-2">
                      <div>
                        <p className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
                          {t("promptDetail.versions.previewTitle", {
                            version: versionDetail.versionNo,
                          })}
                        </p>
                        <p className="text-xs text-slate-500 dark:text-slate-400">
                          {t("promptDetail.versions.createdLabel", {
                            time: versionDetail.created_at
                              ? versionTimestampFormatter.format(
                                  new Date(versionDetail.created_at),
                                )
                              : "—",
                          })}
                        </p>
                      </div>
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        {t("promptDetail.versions.modelLabel", {
                          model: versionDetail.model || detail?.model || "—",
                        })}
                      </p>
                    </div>
                    <section className="space-y-2">
                      <h3 className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
                        {t("promptDetail.versions.instructions")}
                      </h3>
                      <p className="text-sm text-slate-600 dark:text-slate-300">
                        {versionDetail.instructions?.trim() || t("promptDetail.empty")}
                      </p>
                    </section>
                    <section className="space-y-2">
                      <h3 className="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
                        {t("promptDetail.versions.keywords")}
                      </h3>
                      <KeywordGroup
                        title={t("promptDetail.keywords.positive")}
                        polarity="positive"
                        keywords={versionDetail.positive_keywords ?? []}
                      />
                      <KeywordGroup
                        title={t("promptDetail.keywords.negative")}
                        polarity="negative"
                        keywords={versionDetail.negative_keywords ?? []}
                      />
                    </section>
                  </div>
                )}
              </div>
            </section>
          </GlassCard>

          </div>
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
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={handleCopyPlain}
                  className="transition-transform hover:-translate-y-0.5"
                >
                  {t("promptDetail.actions.copyBody")}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCopyMarkdown}
                  className="transition-transform hover:-translate-y-0.5"
                >
                  {t("promptDetail.actions.copyMarkdown")}
                </Button>
              </div>
            </div>

            <div className="markdown-preview h-full overflow-auto rounded-3xl border border-white/60 bg-white/80 p-6 text-sm text-slate-700 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200">
              {!bodyIsEmpty ? (
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{markdownContent}</ReactMarkdown>
              ) : (
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  {t("promptDetail.empty")}
                </p>
              )}
            </div>
          </GlassCard>
        </div>
      )}
      {profileDialogOpen && activeGenerationProfile ? (
        <div className="fixed inset-0 z-[140] flex items-center justify-center px-6 py-10">
          <div
            className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
            onClick={() => setProfileDialogOpen(false)}
          />
          <GlassCard className="relative z-[141] w-full max-w-md space-y-4 border border-slate-200 bg-white/95 p-6 text-slate-700 shadow-[0_30px_60px_-35px_rgba(15,23,42,0.75)] dark:border-slate-700 dark:bg-slate-900/90 dark:text-slate-200">
            <div className="space-y-1">
              <p className="text-xs font-semibold uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
                {t("promptDetail.generationProfileCard.eyebrow", {
                  defaultValue: "生成配置",
                })}
              </p>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <h3 className="text-base font-semibold text-slate-800 dark:text-slate-100">
                    {t("promptDetail.generationProfileCard.title", {
                      defaultValue: "当前参数",
                    })}
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {t("promptDetail.generationProfileCard.subtitle", {
                      defaultValue: "展示最近一次生成时使用的高级参数。",
                    })}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  className="flex h-8 w-8 items-center justify-center rounded-full text-slate-400 hover:text-slate-600 focus-visible:ring-slate-300 dark:text-slate-500 dark:hover:text-slate-200"
                  onClick={() => setProfileDialogOpen(false)}
                  aria-label={t("promptDetail.generationProfileCard.close", {
                    defaultValue: "关闭生成配置",
                  })}
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="grid gap-2 text-sm text-slate-600 dark:text-slate-300">
              <span>
                {t(
                  activeGenerationProfile.stepwiseReasoning
                    ? "promptDetail.generationProfileCard.stepwiseOn"
                    : "promptDetail.generationProfileCard.stepwiseOff",
                  {
                    defaultValue: activeGenerationProfile.stepwiseReasoning
                      ? "逐步推理：已开启"
                      : "逐步推理：已关闭",
                  },
                )}
              </span>
              <span>
                {t("promptDetail.generationProfileCard.temperature", {
                  value: decimalFormatter.format(
                    activeGenerationProfile.temperature,
                  ),
                  defaultValue: `采样温度：${decimalFormatter.format(
                    activeGenerationProfile.temperature,
                  )}`,
                })}
              </span>
              <span>
                {t("promptDetail.generationProfileCard.topP", {
                  value: decimalFormatter.format(activeGenerationProfile.topP),
                  defaultValue: `Top P：${decimalFormatter.format(
                    activeGenerationProfile.topP,
                  )}`,
                })}
              </span>
              <span>
                {t("promptDetail.generationProfileCard.maxTokens", {
                  value: integerFormatter.format(
                    activeGenerationProfile.maxOutputTokens,
                  ),
                  defaultValue: `最大输出 Tokens：${integerFormatter.format(
                    activeGenerationProfile.maxOutputTokens,
                  )}`,
                })}
              </span>
            </div>
          </GlassCard>
        </div>
      ) : null}
      <ShareDialog
        open={shareDialogOpen}
        topic={shareResult?.topic ?? detail?.topic ?? ""}
        payload={shareResult?.payload ?? ""}
        generatedAt={shareResult?.generated_at}
        payloadSize={shareResult?.payload_size}
        onCopy={handleCopyShare}
        onClose={() => setShareDialogOpen(false)}
      />
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
      source: normaliseKeywordSource(item.source),
      weight: normalizeWeight(item.weight),
      overflow,
    };
  });
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

interface ShareDialogProps {
  open: boolean;
  payload: string;
  topic: string;
  generatedAt?: string;
  payloadSize?: number;
  onCopy: () => void;
  onClose: () => void;
}

function ShareDialog({
  open,
  payload,
  topic,
  generatedAt,
  payloadSize,
  onCopy,
  onClose,
}: ShareDialogProps) {
  const { t, i18n } = useTranslation();

  useEffect(() => {
    if (!open) {
      return;
    }
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  const formatter = new Intl.DateTimeFormat(i18n.language, {
    dateStyle: "medium",
    timeStyle: "short",
  });
  const formattedTime = generatedAt ? formatter.format(new Date(generatedAt)) : null;
  const charCount = typeof payloadSize === "number" ? payloadSize : payload.length;

  return (
    <div className="fixed inset-0 z-[130] flex items-center justify-center px-6 py-10">
      <div
        className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
        onClick={onClose}
        role="presentation"
      />
      <GlassCard className="relative z-[131] w-full max-w-2xl space-y-5 bg-white/95 text-slate-700 dark:bg-slate-900/80 dark:text-slate-200">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.3em] text-primary">
            {t("promptDetail.share.title")}
          </p>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
            {topic || t("common.untitled")}
          </h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {t("promptDetail.share.description")}
          </p>
        </div>
        <Textarea
          readOnly
          value={payload}
          rows={4}
          className="h-auto min-h-[120px] rounded-2xl border border-slate-200 bg-white/70 font-mono text-sm text-slate-700 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-100"
        />
        <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500 dark:text-slate-400">
          <span>
            {t("promptDetail.share.payloadLength", {
              count: charCount,
            })}
          </span>
          {formattedTime ? (
            <span>
              {t("promptDetail.share.generatedAt", {
                time: formattedTime,
              })}
            </span>
          ) : null}
        </div>
        <div className="flex items-center justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            className="border-slate-200 text-slate-600 hover:border-primary/40 hover:bg-primary/10 hover:text-primary dark:border-slate-700 dark:text-slate-300"
          >
            {t("common.close")}
          </Button>
          <Button
            type="button"
            onClick={onCopy}
            disabled={!payload}
            className="bg-primary text-primary-foreground hover:bg-primary/90"
          >
            {t("promptDetail.share.copy")}
          </Button>
        </div>
      </GlassCard>
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

function buildDefaultSummary(
  instructionsValue: string | null | undefined,
  bodyValue: string,
): string {
  const paragraphsFromInstructions = splitIntoParagraphs(
    instructionsValue ?? "",
  );
  if (paragraphsFromInstructions.length > 0) {
    return paragraphsFromInstructions
      .slice(0, SUMMARY_PARAGRAPH_LIMIT)
      .join(" ");
  }
  const bodyParagraphs = splitIntoParagraphs(bodyValue);
  if (bodyParagraphs.length === 0) {
    return bodyValue;
  }
  return bodyParagraphs.slice(0, SUMMARY_PARAGRAPH_LIMIT).join(" ");
}

function splitIntoParagraphs(source: string): string[] {
  return source
    .split(/\r?\n+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function normaliseTagsInput(input: string, fallback: string[]): string[] {
  const trimmed = input.trim();
  if (!trimmed) {
    return Array.from(new Set(fallback));
  }
  const segments = trimmed
    .split(/[,，\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
  if (segments.length === 0) {
    return Array.from(new Set(fallback));
  }
  return Array.from(new Set(segments));
}

function serializeKeywordsForSubmission(
  keywords: PromptListKeyword[],
): string {
  const mapped = keywords
    .map((item) => {
      const word = typeof item.word === "string" ? item.word.trim() : "";
      if (!word) {
        return null;
      }
      const weight =
        typeof item.weight === "number" && Number.isFinite(item.weight)
          ? item.weight
          : DEFAULT_KEYWORD_WEIGHT;
      return {
        word,
        weight,
        source: normaliseKeywordSource(item.source),
      };
    })
    .filter(
      (item): item is {
        word: string;
        weight: number;
        source: KeywordSource;
      } => Boolean(item),
    );
  return JSON.stringify(mapped);
}
