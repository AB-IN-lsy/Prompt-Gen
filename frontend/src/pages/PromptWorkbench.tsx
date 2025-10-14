/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:47:19
 * @FilePath: \electron-go-app\frontend\src\pages\PromptWorkbench.tsx
 * @LastEditTime: 2025-10-12 21:59:00
 */
import { ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  DndContext,
  DragEndEvent,
  PointerSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  GripVertical,
  LoaderCircle,
  Minus,
  Plus,
  Sparkles,
  X,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { nanoid } from "nanoid";
import { toast } from "sonner";

import { GlassCard } from "../components/ui/glass-card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Textarea } from "../components/ui/textarea";
import { cn } from "../lib/utils";
import { MarkdownEditor } from "../components/MarkdownEditor";
import { PROMPT_KEYWORD_LIMIT } from "../config/prompt";
import {
  augmentPromptKeywords,
  createManualPromptKeyword,
  fetchKeywords,
  fetchUserModels,
  generatePromptPreview,
  interpretPromptDescription,
  syncPromptWorkspaceKeywords,
  removePromptKeyword,
  savePrompt,
  updateCurrentUser,
  type AugmentPromptKeywordsResponse,
  type GeneratePromptResponse,
  type Keyword,
  type KeywordSource,
  type ManualPromptKeywordRequest,
  type PromptKeywordInput,
  type PromptKeywordResult,
  type SavePromptRequest,
  type UserModelCredential,
} from "../lib/api";
import { ApiError } from "../lib/errors";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import { useAuth } from "../hooks/useAuth";

const DEFAULT_KEYWORD_WEIGHT = 5;

const clampWeight = (value?: number): number => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  if (value < 0) {
    return 0;
  }
  if (value > 5) {
    return 5;
  }
  return Math.round(value);
};

const mapKeywordResultToKeyword = (item: PromptKeywordResult): Keyword => {
  const source = (item.source as KeywordSource) ?? "api";
  const weight = clampWeight(item.weight);
  return {
    id: item.keyword_id ? String(item.keyword_id) : nanoid(),
    word: item.word,
    polarity: item.polarity as Keyword["polarity"],
    source,
    weight,
  };
};

const keywordToInput = (keyword: Keyword): PromptKeywordInput => ({
  keyword_id: keyword.id,
  word: keyword.word,
  polarity: keyword.polarity,
  source: keyword.source,
  weight: clampWeight(keyword.weight),
});

const POSITIVE_CONTAINER_ID = "positive-keyword-container";
const NEGATIVE_CONTAINER_ID = "negative-keyword-container";

const dedupeKeywords = (keywords: Keyword[]): Keyword[] => {
  const seen = new Map<string, Keyword>();
  keywords.forEach((item) => {
    const key = `${item.polarity}:${item.word.toLowerCase()}`;
    if (!seen.has(key)) {
      seen.set(key, item);
    }
  });
  return Array.from(seen.values());
};

export default function PromptWorkbenchPage() {
  const { t } = useTranslation();
  const {
    topic,
    setTopic,
    model,
    setModel,
    positiveKeywords,
    negativeKeywords,
    setKeywords,
    setCollections,
    upsertKeyword,
    updateKeyword,
    removeKeyword,
    prompt,
    setPrompt,
    promptId,
    setPromptId,
    workspaceToken,
    setWorkspaceToken,
    isSaving,
    setSaving,
    reset,
  } = usePromptWorkbench();
  const profile = useAuth((state) => state.profile);
  const setProfile = useAuth((state) => state.setProfile);

  const [description, setDescription] = useState("");
  const [instructions, setInstructions] = useState("");
  const [newKeyword, setNewKeyword] = useState("");
  const [polarity, setPolarity] = useState<"positive" | "negative">("positive");
  const [confidence, setConfidence] = useState<number | null>(null);
  const keywordLimit = PROMPT_KEYWORD_LIMIT;

  const limitKeywordCollections = useCallback(
    (keywords: Keyword[]) => {
      const positives = keywords.filter((item) => item.polarity === "positive");
      const negatives = keywords.filter((item) => item.polarity === "negative");
      const trimmedPositive = positives.length > keywordLimit;
      const trimmedNegative = negatives.length > keywordLimit;
      const limited = [
        ...positives.slice(0, keywordLimit),
        ...negatives.slice(0, keywordLimit),
      ];
      return { limited, trimmedPositive, trimmedNegative };
    },
    [keywordLimit],
  );

  const notifyKeywordTrim = useCallback(
    (trimmedPositive: boolean, trimmedNegative: boolean) => {
      if (trimmedPositive) {
        toast.info(
          t("promptWorkbench.positiveAutoTrimmed", {
            limit: keywordLimit,
            defaultValue: "已自动保留前 {{limit}} 个正向关键词",
          }),
        );
      }
      if (trimmedNegative) {
        toast.info(
          t("promptWorkbench.negativeAutoTrimmed", {
            limit: keywordLimit,
            defaultValue: "已自动保留前 {{limit}} 个负向关键词",
          }),
        );
      }
    },
    [keywordLimit, t],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 6 },
    }),
  );

  const positiveIdSet = useMemo(
    () => new Set(positiveKeywords.map((item) => item.id)),
    [positiveKeywords],
  );
  const negativeIdSet = useMemo(
    () => new Set(negativeKeywords.map((item) => item.id)),
    [negativeKeywords],
  );

  const handleWeightChange = useCallback(
    (id: string, nextWeight: number) => {
      updateKeyword(id, (keyword) => ({
        ...keyword,
        weight: clampWeight(nextWeight),
      }));
    },
    [updateKeyword],
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over) {
        return;
      }
      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) {
        return;
      }

      const sourcePolarity: "positive" | "negative" | null = positiveIdSet.has(
        activeId,
      )
        ? "positive"
        : negativeIdSet.has(activeId)
          ? "negative"
          : null;
      if (!sourcePolarity) {
        return;
      }

      let destinationPolarity: "positive" | "negative";
      if (overId === POSITIVE_CONTAINER_ID) {
        destinationPolarity = "positive";
      } else if (overId === NEGATIVE_CONTAINER_ID) {
        destinationPolarity = "negative";
      } else if (positiveIdSet.has(overId)) {
        destinationPolarity = "positive";
      } else if (negativeIdSet.has(overId)) {
        destinationPolarity = "negative";
      } else {
        return;
      }

      const nextPositive = [...positiveKeywords];
      const nextNegative = [...negativeKeywords];

      const getDestinationIndex = (
        target: Keyword[],
        targetContainerId: string,
      ) => {
        if (
          overId === targetContainerId ||
          (!positiveIdSet.has(overId) && !negativeIdSet.has(overId))
        ) {
          return target.length;
        }
        const idx = target.findIndex((item) => item.id === overId);
        return idx === -1 ? target.length : idx;
      };

      if (sourcePolarity === destinationPolarity) {
        const targetArray =
          sourcePolarity === "positive" ? nextPositive : nextNegative;
        const containerId =
          sourcePolarity === "positive"
            ? POSITIVE_CONTAINER_ID
            : NEGATIVE_CONTAINER_ID;
        const activeIndex = targetArray.findIndex(
          (item) => item.id === activeId,
        );
        if (activeIndex === -1) {
          return;
        }
        let overIndex = getDestinationIndex(targetArray, containerId);
        if (overIndex >= targetArray.length) {
          overIndex = targetArray.length - 1;
        }
        const reordered = arrayMove(targetArray, activeIndex, overIndex);
        if (sourcePolarity === "positive") {
          setCollections(reordered, nextNegative);
        } else {
          setCollections(nextPositive, reordered);
        }
        return;
      }

      const sourceArray =
        sourcePolarity === "positive" ? nextPositive : nextNegative;
      const destinationArray =
        destinationPolarity === "positive" ? nextPositive : nextNegative;
      const sourceIndex = sourceArray.findIndex((item) => item.id === activeId);
      if (sourceIndex === -1) {
        return;
      }
      const [moved] = sourceArray.splice(sourceIndex, 1);
      const destinationIndex = getDestinationIndex(
        destinationArray,
        destinationPolarity === "positive"
          ? POSITIVE_CONTAINER_ID
          : NEGATIVE_CONTAINER_ID,
      );
      const insertIndex = Math.min(destinationIndex, destinationArray.length);
      destinationArray.splice(insertIndex, 0, {
        ...moved,
        polarity: destinationPolarity,
      });
      setCollections(nextPositive, nextNegative);
    },
    [
      negativeIdSet,
      negativeKeywords,
      positiveIdSet,
      positiveKeywords,
      setCollections,
    ],
  );

  const syncMutation = useMutation({
    mutationFn: syncPromptWorkspaceKeywords,
    onError: () => {
      toast.error(
        t("promptWorkbench.keywordSyncFailed", {
          defaultValue: "关键词排序同步失败，请稍后重试",
        }),
      );
    },
  });

  useEffect(() => {
    if (!workspaceToken) {
      return;
    }
    const timer = window.setTimeout(() => {
      syncMutation.mutate({
        workspace_token: workspaceToken,
        positive_keywords: positiveKeywords.map(keywordToInput),
        negative_keywords: negativeKeywords.map(keywordToInput),
      });
    }, 400);
    return () => window.clearTimeout(timer);
  }, [workspaceToken, positiveKeywords, negativeKeywords, syncMutation]);

  const extractKeywordError = useCallback(
    (error: unknown) => {
      if (!(error instanceof ApiError)) {
        return null;
      }
      const details = error.details as {
        code?: string;
        polarity?: string;
        limit?: number;
        word?: string;
      } | null;
      if (!details || details.code !== "KEYWORD_LIMIT") {
        if (details && details.code === "KEYWORD_DUPLICATE") {
          return t("promptWorkbench.keywordDuplicate", {
            defaultValue: "该关键词已存在",
          });
        }
        return null;
      }
      const limitValue =
        typeof details.limit === "number" ? details.limit : keywordLimit;
      if (details.polarity === "negative") {
        return t("promptWorkbench.negativeLimitReached", {
          limit: limitValue,
          defaultValue: "负向关键词已达上限 {{limit}} 个",
        });
      }
      return t("promptWorkbench.positiveLimitReached", {
        limit: limitValue,
        defaultValue: "正向关键词已达上限 {{limit}} 个",
      });
    },
    [keywordLimit, t],
  );

  const keywordQuery = useQuery<Keyword[]>({
    queryKey: ["keywords", topic],
    queryFn: () => fetchKeywords({ topic }),
    enabled: Boolean(topic),
  });

  useEffect(() => {
    if (keywordQuery.data) {
      const { limited, trimmedPositive, trimmedNegative } =
        limitKeywordCollections(keywordQuery.data);
      setKeywords(limited);
      notifyKeywordTrim(trimmedPositive, trimmedNegative);
    }
  }, [
    keywordQuery.data,
    limitKeywordCollections,
    notifyKeywordTrim,
    setKeywords,
  ]);

  const modelsQuery = useQuery<UserModelCredential[]>({
    queryKey: ["models"],
    queryFn: fetchUserModels,
  });

  const modelOptions = useMemo(() => {
    if (!modelsQuery.data) {
      return [];
    }
    return modelsQuery.data.map((item) => ({
      key: item.model_key,
      label: item.display_name || item.model_key,
      disabled: String(item.status ?? "").toLowerCase() === "disabled",
    }));
  }, [modelsQuery.data]);

  useEffect(() => {
    if (modelsQuery.isLoading || modelsQuery.isFetching) {
      return;
    }
    if (modelOptions.length === 0) {
      return;
    }
    const currentOption = modelOptions.find(
      (option) => option.key === model && !option.disabled,
    );
    if (currentOption) {
      return;
    }

    const enabledOptions = modelOptions.filter((option) => !option.disabled);
    const preferredKey = profile?.settings?.preferred_model ?? "";
    const preferredOption = enabledOptions.find(
      (option) => option.key === preferredKey,
    );
    const fallbackOption = preferredOption || enabledOptions[0];

    if (fallbackOption) {
      if (model !== fallbackOption.key) {
        setModel(fallbackOption.key);
        if (!preferredOption && preferredKey) {
          toast.info(
            t("promptWorkbench.modelFallback", {
              defaultValue: "已切换到可用模型 {{model}}",
              model: fallbackOption.label,
            }),
          );
        }
      }
      return;
    }

    const firstOption = modelOptions[0];
    if (firstOption && model !== firstOption.key) {
      setModel(firstOption.key);
      toast.info(
        t("promptWorkbench.modelFallback", {
          defaultValue: "已切换到可用模型 {{model}}",
          model: firstOption.label,
        }),
      );
    }
  }, [
    modelOptions,
    model,
    profile?.settings?.preferred_model,
    setModel,
    t,
    modelsQuery.isFetching,
    modelsQuery.isLoading,
  ]);

  const isModelSelectable = useCallback(
    (key: string) => {
      const option = modelOptions.find((item) => item.key === key);
      if (!option) return true;
      return !option.disabled;
    },
    [modelOptions],
  );

  const interpretMutation = useMutation({
    mutationFn: async () => {
      const descriptionText = description.trim();
      if (!descriptionText) {
        throw new ApiError({
          message: t("promptWorkbench.descriptionRequired", {
            defaultValue: "请先填写需求描述",
          }),
        });
      }
      return interpretPromptDescription({
        description: descriptionText,
        model_key: model,
        language: "zh",
        workspace_token: workspaceToken ?? undefined,
      });
    },
    onSuccess: (data) => {
      if (data.topic) {
        setTopic(data.topic);
      }
      const mapped = [
        ...data.positive_keywords.map(mapKeywordResultToKeyword),
        ...data.negative_keywords.map(mapKeywordResultToKeyword),
      ];
      const deduped = dedupeKeywords(mapped);
      const { limited, trimmedPositive, trimmedNegative } =
        limitKeywordCollections(deduped);
      setKeywords(limited);
      notifyKeywordTrim(trimmedPositive, trimmedNegative);
      setInstructions(data.instructions ?? "");
      setConfidence(data.confidence ?? null);
      setPrompt("");
      setPromptId(null);
      setWorkspaceToken(data.workspace_token ?? null);
      toast.success(
        t("promptWorkbench.interpretSuccess", { defaultValue: "解析完成" }),
      );
    },
    onError: (error: unknown) => {
      const limitMessage = extractKeywordError(error);
      if (limitMessage) {
        toast.warning(limitMessage);
        return;
      }
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const augmentMutation = useMutation({
    mutationFn: async () => {
      if (!topic) {
        throw new ApiError({
          message: t("promptWorkbench.topicMissing", {
            defaultValue: "请先填写主题",
          }),
        });
      }
      return augmentPromptKeywords({
        topic,
        model_key: model,
        existing_positive: positiveKeywords.map(keywordToInput),
        existing_negative: negativeKeywords.map(keywordToInput),
        workspace_token: workspaceToken ?? undefined,
      });
    },
    onSuccess: (data: AugmentPromptKeywordsResponse) => {
      const nextKeywords = [
        ...positiveKeywords,
        ...negativeKeywords,
        ...data.positive.map(mapKeywordResultToKeyword),
        ...data.negative.map(mapKeywordResultToKeyword),
      ];
      const deduped = dedupeKeywords(nextKeywords);
      const { limited, trimmedPositive, trimmedNegative } =
        limitKeywordCollections(deduped);
      setKeywords(limited);
      notifyKeywordTrim(trimmedPositive, trimmedNegative);
      toast.success(
        t("promptWorkbench.augmentSuccess", { defaultValue: "已补充关键词" }),
      );
    },
    onError: (error: unknown) => {
      const limitMessage = extractKeywordError(error);
      if (limitMessage) {
        toast.warning(limitMessage);
        return;
      }
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const manualKeywordMutation = useMutation({
    mutationFn: async (word: string) => {
      if (!topic) {
        throw new ApiError({
          message: t("promptWorkbench.topicMissing", {
            defaultValue: "请先填写主题",
          }),
        });
      }
      const payload: ManualPromptKeywordRequest = {
        topic,
        word,
        polarity,
        workspace_token: workspaceToken ?? undefined,
        prompt_id:
          promptId && Number(promptId) > 0 ? Number(promptId) : undefined,
        weight: DEFAULT_KEYWORD_WEIGHT,
      };
      return createManualPromptKeyword(payload);
    },
    onSuccess: (result) => {
      upsertKeyword(mapKeywordResultToKeyword(result));
      setNewKeyword("");
      toast.success(
        t("promptWorkbench.keywordAdded", { defaultValue: "关键词已添加" }),
      );
    },
    onError: (error: unknown) => {
      const limitMessage = extractKeywordError(error);
      if (limitMessage) {
        toast.warning(limitMessage);
        return;
      }
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const removeKeywordMutation = useMutation({
    mutationFn: async (keyword: Keyword) => {
      if (!workspaceToken) {
        return;
      }
      await removePromptKeyword({
        word: keyword.word,
        polarity: keyword.polarity,
        workspace_token: workspaceToken,
      });
    },
    onSuccess: (_, keyword) => {
      removeKeyword(keyword.id);
    },
    onError: (error: unknown) => {
      const message =
        error instanceof ApiError
          ? (error.message ??
            t("promptWorkbench.keywordRemoveFailed", {
              defaultValue: "移除关键词失败，请稍后再试。",
            }))
          : t("promptWorkbench.keywordRemoveFailed", {
              defaultValue: "移除关键词失败，请稍后再试。",
            });
      toast.error(message);
    },
  });

  const generateMutation = useMutation({
    mutationFn: async () => {
      if (!topic) {
        throw new ApiError({
          message: t("promptWorkbench.topicMissing", {
            defaultValue: "请先填写主题",
          }),
        });
      }
      return generatePromptPreview({
        topic,
        model_key: model,
        positive_keywords: positiveKeywords.map(keywordToInput),
        negative_keywords: negativeKeywords.map(keywordToInput),
        prompt_id:
          promptId && Number(promptId) > 0 ? Number(promptId) : undefined,
        instructions: instructions.trim() || undefined,
        language: "zh",
        workspace_token: workspaceToken ?? undefined,
      });
    },
    onSuccess: (response: GeneratePromptResponse) => {
      setPrompt(response.prompt);
      setWorkspaceToken(response.workspace_token ?? workspaceToken ?? null);
      if (response.positive_keywords || response.negative_keywords) {
        const nextKeywords = [
          ...positiveKeywords,
          ...negativeKeywords,
          ...(response.positive_keywords ?? []).map(mapKeywordResultToKeyword),
          ...(response.negative_keywords ?? []).map(mapKeywordResultToKeyword),
        ];
        const deduped = dedupeKeywords(nextKeywords);
        const { limited, trimmedPositive, trimmedNegative } =
          limitKeywordCollections(deduped);
        setKeywords(limited);
        notifyKeywordTrim(trimmedPositive, trimmedNegative);
      }
      toast.success(
        t("promptWorkbench.generateSuccess", { defaultValue: "生成完成" }),
      );
    },
    onError: (error: unknown) => {
      const limitMessage = extractKeywordError(error);
      if (limitMessage) {
        toast.warning(limitMessage);
        return;
      }
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const savePromptMutation = useMutation({
    mutationFn: async (publish: boolean) => {
      if (!topic) {
        throw new ApiError({
          message: t("promptWorkbench.topicMissing", {
            defaultValue: "请先填写主题",
          }),
        });
      }
      if (!prompt.trim()) {
        throw new ApiError({
          message: t("promptWorkbench.promptEmpty", {
            defaultValue: "提示词内容不能为空",
          }),
        });
      }
      setSaving(true);
      const payload: SavePromptRequest = {
        prompt_id:
          promptId && Number(promptId) > 0 ? Number(promptId) : undefined,
        topic,
        body: prompt,
        model,
        publish,
        status: publish ? "published" : "draft",
        positive_keywords: positiveKeywords.map(keywordToInput),
        negative_keywords: negativeKeywords.map(keywordToInput),
        workspace_token: workspaceToken ?? undefined,
      };
      return savePrompt(payload);
    },
    onSuccess: (response) => {
      if (response.prompt_id && response.prompt_id > 0) {
        setPromptId(String(response.prompt_id));
      }
      setWorkspaceToken(response.workspace_token ?? workspaceToken ?? null);
      if (response.task_id) {
        toast.success(
          t("promptWorkbench.saveQueued", {
            defaultValue: "已提交保存任务，稍后自动落库",
          }),
        );
      } else {
        toast.success(
          t("promptWorkbench.saveSuccess", { defaultValue: "保存成功" }),
        );
      }
    },
    onError: (error: unknown) => {
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
    onSettled: () => setSaving(false),
  });

  const modelPreferenceMutation = useMutation({
    mutationFn: (nextModel: string) =>
      updateCurrentUser({ preferred_model: nextModel }),
    onSuccess: (updatedProfile) => setProfile(updatedProfile),
    onError: (error: unknown) => {
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const handleModelSelect = (nextModel: string) => {
    if (model === nextModel) return;
    if (!isModelSelectable(nextModel)) {
      toast.error(
        t("promptWorkbench.modelDisabled", { defaultValue: "该模型不可用" }),
      );
      return;
    }
    setModel(nextModel);
    const preferred = profile?.settings?.preferred_model;
    if (profile && preferred !== nextModel) {
      modelPreferenceMutation.mutate(nextModel);
    }
  };

  const handleAddKeyword = () => {
    const word = newKeyword.trim();
    if (!word) return;
    const normalized = word.toLowerCase();
    const collection =
      polarity === "positive" ? positiveKeywords : negativeKeywords;
    const duplicated = collection.some(
      (item) => item.word.trim().toLowerCase() === normalized,
    );
    if (duplicated) {
      toast.warning(
        t("promptWorkbench.keywordDuplicate", {
          defaultValue: "该关键词已存在",
        }),
      );
      return;
    }
    if (polarity === "positive" && positiveKeywords.length >= keywordLimit) {
      toast.warning(
        t("promptWorkbench.positiveLimitReached", {
          limit: keywordLimit,
          defaultValue: "正向关键词已达上限 {{limit}} 个",
        }),
      );
      return;
    }
    if (polarity === "negative" && negativeKeywords.length >= keywordLimit) {
      toast.warning(
        t("promptWorkbench.negativeLimitReached", {
          limit: keywordLimit,
          defaultValue: "负向关键词已达上限 {{limit}} 个",
        }),
      );
      return;
    }
    manualKeywordMutation.mutate(word);
  };

  const handleInterpret = () => interpretMutation.mutate();
  const handleAugment = () => augmentMutation.mutate();
  const handleGenerate = () => generateMutation.mutate();
  const handleSaveDraft = () => savePromptMutation.mutate(false);
  const handlePublish = () => savePromptMutation.mutate(true);

  const handleRemoveKeyword = (keyword: Keyword) => {
    if (keyword.polarity === "positive" && positiveKeywords.length <= 1) {
      toast.warning(
        t("promptWorkbench.keywordPositiveHint", {
          count: positiveKeywords.length,
          limit: keywordLimit,
          defaultValue:
            "已选 {{count}} / {{limit}} 个，点击标签可移除，至少保留 1 个关键词",
        }),
      );
      return;
    }
    if (!workspaceToken) {
      removeKeyword(keyword.id);
      return;
    }
    removeKeywordMutation.mutate(keyword);
  };

  const handleCancel = () => {
    reset();
    setDescription("");
    setInstructions("");
    setConfidence(null);
  };

  const hasPositive = positiveKeywords.length > 0;

  const totalKeywords = positiveKeywords.length + negativeKeywords.length;
  const promptEditorMinHeight = useMemo(() => {
    const base = 360;
    const startGrowth = 6;
    const extra = Math.max(0, totalKeywords - startGrowth) * 32;
    return Math.min(1040, base + extra);
  }, [totalKeywords]);
  const promptCardMinHeight = useMemo(() => {
    const base = 720;
    return Math.max(base, promptEditorMinHeight + 240);
  }, [promptEditorMinHeight]);

  const isGenerating = generateMutation.isPending;
  const isAugmenting = augmentMutation.isPending;

  return (
    <div className="grid grid-cols-1 gap-6 text-slate-700 transition-colors dark:text-slate-200 xl:grid-cols-[420px_minmax(0,1fr)] xl:items-start">
      <GlassCard className="flex flex-col gap-6">
        <header className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
              {t("promptWorkbench.modelEyebrow", { defaultValue: "模型设置" })}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">
              {t("promptWorkbench.modelTitle", {
                defaultValue: "选择生成模型",
              })}
            </h2>
          </div>
        </header>
        <div className="rounded-2xl border border-white/60 bg-white/80 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <p className="text-xs text-slate-500 dark:text-slate-400">
            {t("promptWorkbench.modelHint", {
              defaultValue: "解析需求、补词与生成都会使用该模型。",
            })}
          </p>
          <div className="mt-3 flex flex-wrap gap-2 text-xs">
            {modelOptions.map((option) => {
              const selected = model === option.key;
              return (
                <Badge
                  key={option.key}
                  className={cn(
                    "cursor-pointer px-3 py-1 transition",
                    option.disabled && "cursor-not-allowed opacity-60",
                    selected && "border-transparent bg-primary text-white",
                  )}
                  variant={selected ? "default" : "outline"}
                  onClick={() =>
                    !option.disabled && handleModelSelect(option.key)
                  }
                  aria-disabled={option.disabled}
                >
                  {option.label}
                </Badge>
              );
            })}
            {modelOptions.length === 0 ? (
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("promptWorkbench.modelEmptyHint", {
                  defaultValue: "暂无可用模型，请先在设置页配置。",
                })}
              </span>
            ) : null}
          </div>
        </div>

        <header className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
              {t("promptWorkbench.descriptionEyebrow", {
                defaultValue: "需求解析",
              })}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">
              {t("promptWorkbench.descriptionTitle", {
                defaultValue: "自然语言描述",
              })}
            </h2>
          </div>
        </header>
        <p className="text-xs text-slate-500 dark:text-slate-400">
          {t("promptWorkbench.descriptionHelper", {
            defaultValue:
              "解析后会自动填写主题并补充首批关键词，可随时手动调整。",
          })}
        </p>
        <div className="rounded-2xl border border-white/60 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <Textarea
            value={description}
            onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
              setDescription(event.target.value)
            }
            placeholder={t("promptWorkbench.descriptionPlaceholder", {
              defaultValue: "例如：生成一份针对 React Hooks 的技术面试问答",
            })}
            className="min-h-[160px]"
          />
          <div className="mt-3 flex items-center justify-between gap-3 text-xs text-slate-400 dark:text-slate-500">
            {confidence !== null ? (
              <span>
                {t("promptWorkbench.confidence", {
                  defaultValue: "置信度：{{value}}",
                  value: Math.round(confidence * 100) / 100,
                })}
              </span>
            ) : (
              <span>
                {t("promptWorkbench.confidenceHint", {
                  defaultValue: "点击下方按钮让 AI 解析需求",
                })}
              </span>
            )}
            <Button
              variant="secondary"
              size="sm"
              onClick={handleInterpret}
              disabled={interpretMutation.isPending || !description.trim()}
            >
              {interpretMutation.isPending ? (
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Sparkles className="mr-2 h-4 w-4" />
              )}
              {t("promptWorkbench.interpret", { defaultValue: "解析描述" })}
            </Button>
          </div>
        </div>

        <header className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
              {t("promptWorkbench.keywordsTitle", { defaultValue: "关键词" })}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">
              {t("promptWorkbench.keywordsSubtitle", {
                defaultValue: "关键词治理",
              })}
            </h2>
          </div>
          <Button
            variant="default"
            size="sm"
            className="shadow-md"
            onClick={handleAugment}
            disabled={isAugmenting || !topic}
          >
            {isAugmenting ? (
              <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="mr-2 h-4 w-4" />
            )}
            {t("promptWorkbench.augmentKeywords", {
              defaultValue: "AI 补充关键词",
            })}
          </Button>
        </header>

        <div className="rounded-2xl border border-white/60 bg-white/80 p-4 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <p className="text-sm font-medium text-slate-600 dark:text-slate-300">
            {t("promptWorkbench.addKeyword", {
              defaultValue: "手动添加关键词",
            })}
          </p>
          <div className="mt-3 flex gap-2">
            <Input
              value={newKeyword}
              onChange={(event: ChangeEvent<HTMLInputElement>) =>
                setNewKeyword(event.target.value)
              }
              placeholder={t("promptWorkbench.inputPlaceholder", {
                defaultValue: "输入关键词",
              })}
            />
            <Button
              variant="secondary"
              size="default"
              className="min-w-[132px] justify-center px-5"
              onClick={handleAddKeyword}
              disabled={manualKeywordMutation.isPending}
            >
              {manualKeywordMutation.isPending ? (
                <LoaderCircle className="mr-1.5 h-4 w-4 animate-spin" />
              ) : (
                <Plus className="mr-1.5 h-4 w-4" />
              )}
              {t("promptWorkbench.addKeyword", { defaultValue: "添加" })}
            </Button>
          </div>
          <p className="mt-2 text-xs text-slate-400 dark:text-slate-500">
            {t("promptWorkbench.keywordLimitNote", {
              limit: keywordLimit,
              defaultValue: "提示：正向与负向各最多 {{limit}} 个关键词",
            })}
          </p>
          <div className="mt-3 flex gap-2 text-xs">
            <Badge
              className={cn(
                "cursor-pointer px-3 py-1",
                polarity === "positive" &&
                  "border-transparent bg-primary text-white",
              )}
              variant={polarity === "positive" ? "default" : "outline"}
              onClick={() => setPolarity("positive")}
            >
              {t("promptWorkbench.positive", { defaultValue: "正向" })}
            </Badge>
            <Badge
              className={cn(
                "cursor-pointer px-3 py-1",
                polarity === "negative" &&
                  "border-transparent bg-secondary text-white",
              )}
              variant={polarity === "negative" ? "default" : "outline"}
              onClick={() => setPolarity("negative")}
            >
              {t("promptWorkbench.negative", { defaultValue: "负向" })}
            </Badge>
          </div>
        </div>

        <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
          <KeywordSection
            title={t("promptWorkbench.positiveSectionTitle", {
              defaultValue: "正向关键词",
            })}
            hint={t("promptWorkbench.keywordPositiveHint", {
              count: positiveKeywords.length,
              limit: keywordLimit,
              defaultValue:
                "已选 {{count}} / {{limit}} 个，点击标签可移除，至少保留 1 个关键词",
            })}
            keywords={positiveKeywords}
            polarity="positive"
            droppableId={POSITIVE_CONTAINER_ID}
            onWeightChange={handleWeightChange}
            onRemove={handleRemoveKeyword}
          />
          <KeywordSection
            title={t("promptWorkbench.negativeSectionTitle", {
              defaultValue: "负向关键词",
            })}
            hint={t("promptWorkbench.keywordNegativeHint", {
              count: negativeKeywords.length,
              limit: keywordLimit,
              defaultValue: "已选 {{count}} / {{limit}} 个，点击标签可移除",
            })}
            keywords={negativeKeywords}
            polarity="negative"
            droppableId={NEGATIVE_CONTAINER_ID}
            onWeightChange={handleWeightChange}
            onRemove={handleRemoveKeyword}
          />
        </DndContext>
      </GlassCard>

      <GlassCard
        className="flex h-full flex-col gap-6"
        style={{ minHeight: promptCardMinHeight }}
      >
        <header className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
              {t("promptWorkbench.workbenchEyebrow", {
                defaultValue: "提示词",
              })}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">
              {t("promptWorkbench.workbenchTitle", {
                defaultValue: "提示词工作台",
              })}
            </h2>
          </div>
        </header>

        <div className="rounded-3xl border border-white/60 bg-white/80 p-5 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <label className="text-sm font-medium text-slate-600 dark:text-slate-300">
            {t("promptWorkbench.topicLabel", { defaultValue: "主题" })}
          </label>
          <Input
            className="mt-3"
            value={topic}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setTopic(event.target.value)
            }
            placeholder={t("promptWorkbench.topicPlaceholder", {
              defaultValue: "例如：前端面试技术提示词",
            })}
          />
        </div>

        <div className="rounded-3xl border border-white/60 bg-white/80 p-5 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <label className="text-sm font-medium text-slate-600 dark:text-slate-300">
            {t("promptWorkbench.instructionsLabel", {
              defaultValue: "补充要求",
            })}
          </label>
          <Textarea
            className="mt-3"
            value={instructions}
            onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
              setInstructions(event.target.value)
            }
            placeholder={t("promptWorkbench.instructionsPlaceholder", {
              defaultValue: "可选：语气、结构、输出格式等",
            })}
          />
        </div>

        <div className="flex flex-1 flex-col rounded-3xl border border-white/60 bg-white/80 p-5 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/70">
          <label className="text-sm font-medium text-slate-600 dark:text-slate-300">
            {t("promptWorkbench.draftLabel", { defaultValue: "Prompt 草稿" })}
          </label>
          <div className="mt-3 flex-1">
            <MarkdownEditor
              value={prompt}
              onChange={setPrompt}
              minHeight={promptEditorMinHeight}
              placeholder={t("promptWorkbench.draftPlaceholder", {
                defaultValue: "在此粘贴或编辑生成的 Prompt",
              })}
              hint={`${t("promptWorkbench.autosave", {
                defaultValue: "自动保存",
              })}: ${
                isSaving
                  ? t("promptWorkbench.autosaveSaving", {
                      defaultValue: "保存中",
                    })
                  : t("promptWorkbench.autosaveSaved", {
                      defaultValue: "已保存",
                    })
              }`}
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-end gap-3">
          <Button variant="ghost" onClick={handleCancel}>
            {t("promptWorkbench.cancel", { defaultValue: "重置" })}
          </Button>
          <Button
            variant="outline"
            onClick={handleSaveDraft}
            disabled={savePromptMutation.isPending}
          >
            {savePromptMutation.isPending && !savePromptMutation.variables ? (
              <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
            ) : null}
            {t("promptWorkbench.saveDraft", { defaultValue: "保存草稿" })}
          </Button>
          <Button
            variant="secondary"
            onClick={handleGenerate}
            disabled={isGenerating || !hasPositive}
          >
            {isGenerating ? (
              <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="mr-2 h-4 w-4" />
            )}
            {t("promptWorkbench.generate", { defaultValue: "AI 生成" })}
          </Button>
          <Button
            variant="secondary"
            size="lg"
            disabled={
              !hasPositive || !prompt.trim() || savePromptMutation.isPending
            }
            onClick={handlePublish}
          >
            {savePromptMutation.isPending && savePromptMutation.variables ? (
              <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
            ) : null}
            {t("promptWorkbench.publish", { defaultValue: "发布" })}
          </Button>
        </div>
      </GlassCard>
    </div>
  );
}

interface KeywordSectionProps {
  title: string;
  hint?: string;
  keywords: Keyword[];
  polarity: "positive" | "negative";
  droppableId: string;
  onWeightChange: (id: string, weight: number) => void;
  onRemove: (keyword: Keyword) => void;
}

function KeywordSection({
  title,
  hint,
  keywords,
  polarity,
  droppableId,
  onWeightChange,
  onRemove,
}: KeywordSectionProps) {
  const { t } = useTranslation();
  const { setNodeRef, isOver } = useDroppable({ id: droppableId });

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-sm text-slate-500 dark:text-slate-400">
        <span>{title}</span>
        {hint ? (
          <span className="text-xs text-slate-400 dark:text-slate-500">
            {hint}
          </span>
        ) : null}
      </div>
      <div
        ref={setNodeRef}
        className={cn(
          "rounded-2xl border border-white/60 bg-white/80 p-3 transition-colors dark:border-slate-800 dark:bg-slate-900/70",
          isOver && "border-primary/60 bg-primary/5 dark:border-primary/60",
        )}
      >
        <SortableContext
          items={keywords.map((keyword) => keyword.id)}
          strategy={verticalListSortingStrategy}
        >
          {keywords.length === 0 ? (
            <div className="flex h-16 items-center justify-center text-xs text-slate-400 dark:text-slate-500">
              {t("promptWorkbench.keywordDropPlaceholder", {
                defaultValue: "将关键词拖拽到此处",
              })}
            </div>
          ) : (
            <div className="space-y-2">
              {keywords.map((keyword) => (
                <SortableKeywordChip
                  key={keyword.id}
                  keyword={keyword}
                  polarity={polarity}
                  onWeightChange={onWeightChange}
                  onRemove={onRemove}
                />
              ))}
            </div>
          )}
        </SortableContext>
      </div>
    </div>
  );
}

interface SortableKeywordChipProps {
  keyword: Keyword;
  polarity: "positive" | "negative";
  onWeightChange: (id: string, weight: number) => void;
  onRemove: (keyword: Keyword) => void;
}

function SortableKeywordChip({
  keyword,
  polarity,
  onWeightChange,
  onRemove,
}: SortableKeywordChipProps) {
  const { t } = useTranslation();
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: keyword.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };
  const isPositive = polarity === "positive";
  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        "group relative flex items-center gap-3 rounded-2xl border px-3 py-2 text-sm shadow-sm transition-colors",
        isPositive
          ? "border-primary/30 bg-primary/10 text-primary"
          : "border-secondary/30 bg-secondary/10 text-secondary",
        isDragging && "opacity-70",
      )}
    >
      <button
        type="button"
        className="cursor-grab rounded-full bg-white/70 p-1 text-slate-500 transition hover:text-slate-700 focus:outline-none dark:bg-slate-800/60 dark:text-slate-300"
        aria-label={t("promptWorkbench.dragHandle", {
          defaultValue: "拖拽关键词",
        })}
        {...listeners}
        {...attributes}
      >
        <GripVertical className="h-3.5 w-3.5" />
      </button>
      <div className="flex flex-1 flex-col gap-1 text-left">
        <span className="text-sm font-medium text-slate-700 dark:text-slate-100">
          {keyword.word}
        </span>
        <div className="flex flex-wrap items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
          <div className="flex items-center gap-1">
            <button
              type="button"
              className="rounded-full border border-white/60 bg-white/70 p-1 text-slate-500 transition hover:bg-white focus:outline-none disabled:opacity-40 dark:border-slate-700 dark:bg-slate-800/60"
              onClick={() => onWeightChange(keyword.id, keyword.weight - 1)}
              disabled={keyword.weight <= 0}
              aria-label={t("promptWorkbench.decreaseWeight", {
                defaultValue: "降低权重",
              })}
            >
              <Minus className="h-3 w-3" />
            </button>
            <span className="min-w-[48px] text-center text-[11px] font-semibold text-slate-600 dark:text-slate-200">
              {keyword.weight}/5
            </span>
            <button
              type="button"
              className="rounded-full border border-white/60 bg-white/70 p-1 text-slate-500 transition hover:bg-white focus:outline-none disabled:opacity-40 dark:border-slate-700 dark:bg-slate-800/60"
              onClick={() => onWeightChange(keyword.id, keyword.weight + 1)}
              disabled={keyword.weight >= 5}
              aria-label={t("promptWorkbench.increaseWeight", {
                defaultValue: "提升权重",
              })}
            >
              <Plus className="h-3 w-3" />
            </button>
          </div>
          <span className="rounded-full bg-white/80 px-2 py-0.5 text-[10px] uppercase tracking-wide text-slate-500 dark:bg-slate-800/60 dark:text-slate-200">
            {keyword.source}
          </span>
        </div>
      </div>
      <button
        type="button"
        className="rounded-full bg-white/70 p-1 text-slate-400 transition hover:bg-rose-100 hover:text-rose-500 focus:outline-none dark:bg-slate-800/60 dark:text-slate-300 dark:hover:bg-rose-500/10 dark:hover:text-rose-400"
        onClick={() => onRemove(keyword)}
        aria-label={t("promptWorkbench.removeKeywordAria", {
          defaultValue: "移除关键词",
        })}
      >
        <X className="h-3 w-3" />
      </button>
    </div>
  );
}
