/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:47:19
 * @FilePath: \electron-go-app\frontend\src\pages\PromptWorkbench.tsx
 * @LastEditTime: 2025-10-22 13:44:52
 */
import {
  ChangeEvent,
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  DndContext,
  DragCancelEvent,
  DragEndEvent,
  DragOverEvent,
  DragOverlay,
  DragStartEvent,
  PointerSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import type { Modifier } from "@dnd-kit/core";
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
import { nanoid } from "nanoid";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { GlassCard } from "../components/ui/glass-card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Textarea } from "../components/ui/textarea";
import { cn, clampTextWithOverflow, formatOverflowLabel } from "../lib/utils";
import { MarkdownEditor } from "../components/MarkdownEditor";
import {
  PROMPT_KEYWORD_LIMIT,
  PROMPT_KEYWORD_MAX_LENGTH,
  PROMPT_TAG_LIMIT,
  PROMPT_TAG_MAX_LENGTH,
  PROMPT_AI_GENERATE_MIN_DURATION_MS,
  PROMPT_AUTOSAVE_DELAY_MS,
  DEFAULT_KEYWORD_WEIGHT,
} from "../config/prompt";
import {
  augmentPromptKeywords,
  createManualPromptKeyword,
  fetchUserModels,
  generatePromptPreview,
  interpretPromptDescription,
  normaliseKeywordSource,
  syncPromptWorkspaceKeywords,
  removePromptKeyword,
  savePrompt,
  updateCurrentUser,
  type AugmentPromptKeywordsResponse,
  type GeneratePromptResponse,
  type Keyword,
  type ManualPromptKeywordRequest,
  type PromptKeywordInput,
  type PromptKeywordResult,
  type SavePromptRequest,
  type UserModelCredential,
} from "../lib/api";
import { ApiError } from "../lib/errors";
import { usePromptWorkbench } from "../hooks/usePromptWorkbench";
import { useAuth } from "../hooks/useAuth";
import { PageHeader } from "../components/layout/PageHeader";

const clampWeight = (value?: number): number => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  if (value < 0) {
    return 0;
  }
  if (value > DEFAULT_KEYWORD_WEIGHT) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  return Math.round(value);
};

const mapKeywordResultToKeyword = (
  item: PromptKeywordResult,
): Keyword => {
  const polarity = (item.polarity as Keyword["polarity"]) ?? "positive";
  const source = normaliseKeywordSource(item.source);
  const weight = clampWeight(item.weight);
  const { value, overflow } = clampTextWithOverflow(
    item.word ?? "",
    PROMPT_KEYWORD_MAX_LENGTH,
  );
  const backendId =
    typeof item.keyword_id === "number" ? item.keyword_id : undefined;
  return {
    id: nanoid(),
    keywordId: backendId,
    word: value,
    polarity,
    source,
    weight,
    overflow,
  };
};

const keywordToInput = (keyword: Keyword): PromptKeywordInput => {
  const { value } = clampTextWithOverflow(
    keyword.word,
    PROMPT_KEYWORD_MAX_LENGTH,
  );
  const backendId =
    typeof keyword.keywordId === "number" ? keyword.keywordId : undefined;
  return {
    keyword_id: backendId,
    word: value,
    polarity: keyword.polarity,
    source: keyword.source,
    weight: clampWeight(keyword.weight),
  };
};

const POSITIVE_CONTAINER_ID = "positive-keyword-container";
const NEGATIVE_CONTAINER_ID = "negative-keyword-container";

interface ClientCoordinates {
  x: number;
  y: number;
}

const getEventClientPoint = (event: Event): ClientCoordinates | null => {
  if ("touches" in event) {
    const touchEvent = event as TouchEvent;
    const touch =
      touchEvent.touches[0] ||
      (touchEvent.changedTouches && touchEvent.changedTouches[0]);
    if (touch) {
      return { x: touch.clientX, y: touch.clientY };
    }
    return null;
  }

  if ("clientX" in event && "clientY" in event) {
    const pointerEvent = event as MouseEvent;
    return { x: pointerEvent.clientX, y: pointerEvent.clientY };
  }

  return null;
};

const snapOverlayToCursor: Modifier = ({
  activatorEvent,
  draggingNodeRect,
  transform,
}) => {
  if (!activatorEvent || !draggingNodeRect) {
    return transform;
  }

  const point = getEventClientPoint(activatorEvent);
  if (!point) {
    return transform;
  }

  const centerX = draggingNodeRect.left + draggingNodeRect.width / 2;
  const centerY = draggingNodeRect.top + draggingNodeRect.height / 2;

  return {
    ...transform,
    x: transform.x + (point.x - centerX),
    y: transform.y + (point.y - centerY),
  };
};

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
    tags,
    addTag,
    removeTag,
    prompt,
    setPrompt,
    instructions,
    setInstructions,
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
  const [activeKeyword, setActiveKeyword] = useState<Keyword | null>(null);
  const [newKeyword, setNewKeyword] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [polarity, setPolarity] = useState<"positive" | "negative">("positive");
  const [confidence, setConfidence] = useState<number | null>(null);
  const [keywordsDirty, setKeywordsDirty] = useState(false);
  const [dropIndicator, setDropIndicator] = useState<{
    polarity: "positive" | "negative";
    index: number;
  } | null>(null);
  const interpretToastId = useRef<string | number | null>(null);
  const augmentToastId = useRef<string | number | null>(null);
  const generateToastId = useRef<string | number | null>(null);
  const generationStartRef = useRef<number>(0);
  const generationDelayTimeoutRef = useRef<number | null>(null);
  const autosaveTimerRef = useRef<number | null>(null);
  const autosavePendingRef = useRef(false);
  const lastSavedSignatureRef = useRef<string>("");
  const autosaveSignatureRef = useRef<string>("");
  const triggerAutoSaveRef = useRef<() => void>(() => {});
  const hasInitialSignatureRef = useRef(false);
  const previousPromptIdRef = useRef<string | null>(null);
  const clearDropIndicator = useCallback(() => {
    setDropIndicator((previous) => (previous ? null : previous));
  }, []);
  const showLoadingToast = useCallback((ref: { current: string | number | null }, message: string) => {
    if (ref.current) {
      toast.dismiss(ref.current);
    }
    ref.current = toast.loading(message, { duration: Infinity });
  }, []);
  const dismissLoadingToast = useCallback((ref: { current: string | number | null }) => {
    if (ref.current) {
      toast.dismiss(ref.current);
      ref.current = null;
    }
  }, []);
  const ensureMinimumGenerationDuration = useCallback(
    (next: () => void) => {
      const elapsed = Date.now() - generationStartRef.current;
      const remaining = Math.max(
        0,
        PROMPT_AI_GENERATE_MIN_DURATION_MS - elapsed,
      );
      if (generationDelayTimeoutRef.current !== null) {
        window.clearTimeout(generationDelayTimeoutRef.current);
        generationDelayTimeoutRef.current = null;
      }
      if (remaining > 0) {
        generationDelayTimeoutRef.current = window.setTimeout(() => {
          generationDelayTimeoutRef.current = null;
          next();
        }, remaining);
      } else {
        next();
      }
    },
    [PROMPT_AI_GENERATE_MIN_DURATION_MS],
  );

  const keywordLimit = PROMPT_KEYWORD_LIMIT;
  const keywordMaxLength = PROMPT_KEYWORD_MAX_LENGTH;
  const tagLimit = PROMPT_TAG_LIMIT;
  const tagMaxLength = PROMPT_TAG_MAX_LENGTH;

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

  const findKeywordById = useCallback(
    (id: string): Keyword | null => {
      const fromPositive = positiveKeywords.find((item) => item.id === id);
      if (fromPositive) {
        return fromPositive;
      }
      const fromNegative = negativeKeywords.find((item) => item.id === id);
      return fromNegative ?? null;
    },
    [negativeKeywords, positiveKeywords],
  );

  const serializeKeywords = useCallback(
    (keywords: Keyword[]) =>
      keywords.map((item) => ({
        id: item.id,
        weight: clampWeight(item.weight),
        polarity: item.polarity,
        word: item.word,
      })),
    [],
  );

  const buildSignature = useCallback(
    (positive: Keyword[], negative: Keyword[]) =>
      JSON.stringify({
        positive: serializeKeywords(positive),
        negative: serializeKeywords(negative),
      }),
    [serializeKeywords],
  );

  const syncSignature = useMemo(
    () => buildSignature(positiveKeywords, negativeKeywords),
    [buildSignature, negativeKeywords, positiveKeywords],
  );

  const autosaveFingerprint = useMemo(
    () =>
      JSON.stringify({
        topic: topic.trim(),
        prompt: prompt.trim(),
        instructions: instructions.trim(),
        model: model.trim(),
        tags: tags.map((tag) => tag.value.trim()),
        positive: serializeKeywords(positiveKeywords),
        negative: serializeKeywords(negativeKeywords),
      }),
    [
      instructions,
      model,
      negativeKeywords,
      positiveKeywords,
      prompt,
      serializeKeywords,
      tags,
      topic,
    ],
  );

  const lastSyncedSignature = useRef<string>("");
  const latestSignatureRef = useRef<string>(syncSignature);
  const syncTimerRef = useRef<number | null>(null);

  const markKeywordsDirty = useCallback(() => setKeywordsDirty(true), []);

  const updateSignatureBaseline = useCallback(
    (positive: Keyword[], negative: Keyword[]) => {
      const signature = buildSignature(positive, negative);
      lastSyncedSignature.current = signature;
      latestSignatureRef.current = signature;
      setKeywordsDirty(false);
    },
    [buildSignature],
  );

  const handleWeightChange = useCallback(
    (id: string, nextWeight: number) => {
      updateKeyword(id, (keyword) => ({
        ...keyword,
        weight: clampWeight(nextWeight),
      }));
      markKeywordsDirty();
    },
    [markKeywordsDirty, updateKeyword],
  );

  const sortByWeight = useCallback(
    (polarity: "positive" | "negative") => {
      const sortedPositive = [...positiveKeywords];
      const sortedNegative = [...negativeKeywords];
      const compare = (a: Keyword, b: Keyword) =>
        clampWeight(b.weight) - clampWeight(a.weight) ||
        a.word.localeCompare(b.word, undefined, { sensitivity: "base" });
      if (polarity === "positive") {
        sortedPositive.sort(compare);
      } else {
        sortedNegative.sort(compare);
      }
      setCollections(sortedPositive, sortedNegative);
      markKeywordsDirty();
    },
    [markKeywordsDirty, negativeKeywords, positiveKeywords, setCollections],
  );

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const activeId = String(event.active.id);
      const keywordExists =
        positiveKeywords.some((item) => item.id === activeId) ||
        negativeKeywords.some((item) => item.id === activeId);
      if (!keywordExists) {
        return;
      }
      setActiveKeyword(findKeywordById(activeId));
      clearDropIndicator();
    },
    [
      clearDropIndicator,
      findKeywordById,
      negativeKeywords,
      positiveKeywords,
    ],
  );

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event;
      if (!over) {
        clearDropIndicator();
        return;
      }
      const activeId = String(active.id);
      const overId = String(over.id);
      if (overId === activeId) {
        clearDropIndicator();
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
        clearDropIndicator();
        return;
      }
      let destinationPolarity: "positive" | "negative" | null = null;
      if (overId === POSITIVE_CONTAINER_ID || positiveIdSet.has(overId)) {
        destinationPolarity = "positive";
      } else if (overId === NEGATIVE_CONTAINER_ID || negativeIdSet.has(overId)) {
        destinationPolarity = "negative";
      }
      if (!destinationPolarity) {
        clearDropIndicator();
        return;
      }
      const targetKeywords =
        destinationPolarity === "positive"
          ? positiveKeywords
          : negativeKeywords;
      const targetContainerId =
        destinationPolarity === "positive"
          ? POSITIVE_CONTAINER_ID
          : NEGATIVE_CONTAINER_ID;
      let destinationIndex: number;
      if (
        overId === targetContainerId ||
        (!positiveIdSet.has(overId) && !negativeIdSet.has(overId))
      ) {
        destinationIndex = targetKeywords.length;
      } else {
        const idx = targetKeywords.findIndex((item) => item.id === overId);
        destinationIndex = idx === -1 ? targetKeywords.length : idx;
      }
      if (
        sourcePolarity === destinationPolarity &&
        destinationIndex ===
          targetKeywords.findIndex((item) => item.id === activeId)
      ) {
        clearDropIndicator();
        return;
      }
      setDropIndicator((previous) => {
        if (
          previous &&
          previous.polarity === destinationPolarity &&
          previous.index === destinationIndex
        ) {
          return previous;
        }
        return { polarity: destinationPolarity, index: destinationIndex };
      });
    },
    [
      clearDropIndicator,
      negativeIdSet,
      negativeKeywords,
      positiveIdSet,
      positiveKeywords,
    ],
  );

  const handleDragCancel = useCallback(
    (_event: DragCancelEvent) => {
      clearDropIndicator();
      setActiveKeyword(null);
    },
    [clearDropIndicator, setActiveKeyword],
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      clearDropIndicator();
      setActiveKeyword(null);
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
          nextPositive.splice(0, nextPositive.length, ...reordered);
        } else {
          nextNegative.splice(0, nextNegative.length, ...reordered);
        }
      } else {
        const sourceArray =
          sourcePolarity === "positive" ? nextPositive : nextNegative;
        const destinationArray =
          destinationPolarity === "positive" ? nextPositive : nextNegative;
        const sourceIndex = sourceArray.findIndex(
          (item) => item.id === activeId,
        );
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
      }

      setCollections(nextPositive, nextNegative);
      markKeywordsDirty();
    },
    [
      clearDropIndicator,
      markKeywordsDirty,
      negativeIdSet,
      negativeKeywords,
      positiveIdSet,
      positiveKeywords,
      setActiveKeyword,
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
    onSuccess: () => {
      lastSyncedSignature.current = latestSignatureRef.current;
      setKeywordsDirty(false);
    },
  });

  useEffect(() => {
    latestSignatureRef.current = syncSignature;
  }, [syncSignature]);

  useEffect(() => {
    autosaveSignatureRef.current = autosaveFingerprint;
  }, [autosaveFingerprint]);

  useEffect(() => {
    const currentId = promptId ?? null;
    if (previousPromptIdRef.current !== currentId) {
      previousPromptIdRef.current = currentId;
      hasInitialSignatureRef.current = false;
    }
    if (!currentId) {
      return;
    }
    if (
      !hasInitialSignatureRef.current &&
      topic.trim() &&
      prompt.trim()
    ) {
      lastSavedSignatureRef.current = autosaveSignatureRef.current;
      hasInitialSignatureRef.current = true;
    }
  }, [promptId, prompt, topic]);

  useEffect(() => {
    return () => {
      if (generationDelayTimeoutRef.current !== null) {
        window.clearTimeout(generationDelayTimeoutRef.current);
        generationDelayTimeoutRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    if (syncTimerRef.current !== null) {
      window.clearTimeout(syncTimerRef.current);
      syncTimerRef.current = null;
    }

    if (!workspaceToken) {
      lastSyncedSignature.current = "";
      return;
    }

    if (!keywordsDirty) {
      return;
    }

    if (syncMutation.isPending) {
      return;
    }

    if (syncSignature === lastSyncedSignature.current) {
      setKeywordsDirty(false);
      return;
    }

    syncTimerRef.current = window.setTimeout(() => {
      latestSignatureRef.current = syncSignature;
      syncMutation.mutate({
        workspace_token: workspaceToken,
        positive_keywords: positiveKeywords.map(keywordToInput),
        negative_keywords: negativeKeywords.map(keywordToInput),
      });
    }, 400);

    return () => {
      if (syncTimerRef.current !== null) {
        window.clearTimeout(syncTimerRef.current);
        syncTimerRef.current = null;
      }
    };
  }, [
    keywordsDirty,
    negativeKeywords,
    positiveKeywords,
    syncMutation,
    syncSignature,
    workspaceToken,
  ]);

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
    const preferred = profile?.settings?.preferred_model ?? "";
    if (!model && preferred) {
      setModel(preferred);
    }
  }, [model, profile?.settings?.preferred_model, setModel]);

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
    onMutate: () => {
      showLoadingToast(
        interpretToastId,
        t("promptWorkbench.interpretLoading", {
          defaultValue: "正在解析描述...",
        }),
      );
    },
    onSuccess: (data) => {
      dismissLoadingToast(interpretToastId);
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
      const nextPositive = limited.filter(
        (item) => item.polarity === "positive",
      );
      const nextNegative = limited.filter(
        (item) => item.polarity === "negative",
      );
      updateSignatureBaseline(nextPositive, nextNegative);
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
      dismissLoadingToast(interpretToastId);
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
    onMutate: () => {
      showLoadingToast(
        augmentToastId,
        t("promptWorkbench.augmentLoading", {
          defaultValue: "正在补充关键词...",
        }),
      );
    },
    onSuccess: (data: AugmentPromptKeywordsResponse) => {
      dismissLoadingToast(augmentToastId);
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
      const nextPositive = limited.filter(
        (item) => item.polarity === "positive",
      );
      const nextNegative = limited.filter(
        (item) => item.polarity === "negative",
      );
      updateSignatureBaseline(nextPositive, nextNegative);
      toast.success(
        t("promptWorkbench.augmentSuccess", { defaultValue: "已补充关键词" }),
      );
    },
    onError: (error: unknown) => {
      dismissLoadingToast(augmentToastId);
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
      markKeywordsDirty();
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
    onMutate: () => {
      generationStartRef.current = Date.now();
      if (generationDelayTimeoutRef.current !== null) {
        window.clearTimeout(generationDelayTimeoutRef.current);
        generationDelayTimeoutRef.current = null;
      }
      showLoadingToast(
        generateToastId,
        t("promptWorkbench.generateLoading", {
          defaultValue: "正在生成 Prompt… AI 生成可能需要更长时间，请稍候",
        }),
      );
    },
    onSuccess: (response: GeneratePromptResponse) => {
      ensureMinimumGenerationDuration(() => {
        dismissLoadingToast(generateToastId);
        setPrompt(response.prompt);
        setWorkspaceToken(response.workspace_token ?? workspaceToken ?? null);
        if (response.positive_keywords || response.negative_keywords) {
          const nextKeywords = [
            ...positiveKeywords,
            ...negativeKeywords,
            ...(response.positive_keywords ?? []).map(
              mapKeywordResultToKeyword,
            ),
            ...(response.negative_keywords ?? []).map(
              mapKeywordResultToKeyword,
            ),
          ];
          const deduped = dedupeKeywords(nextKeywords);
          const { limited, trimmedPositive, trimmedNegative } =
            limitKeywordCollections(deduped);
          setKeywords(limited);
          notifyKeywordTrim(trimmedPositive, trimmedNegative);
          const nextPositive = limited.filter(
            (item) => item.polarity === "positive",
          );
          const nextNegative = limited.filter(
            (item) => item.polarity === "negative",
          );
          updateSignatureBaseline(nextPositive, nextNegative);
        }
        toast.success(
          t("promptWorkbench.generateSuccess", { defaultValue: "生成完成" }),
        );
        triggerAutoSaveRef.current();
      });
    },
    onError: (error: unknown) => {
      ensureMinimumGenerationDuration(() => {
        dismissLoadingToast(generateToastId);
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
      });
    },
  });

  const validatePublishRequirements = useCallback((): boolean => {
    const missingMessages: string[] = [];
    const trimmedTopic = topic.trim();
    const trimmedPrompt = prompt.trim();
    const trimmedInstructions = instructions.trim();
    if (!trimmedTopic) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.topic", {
          defaultValue: "发布前需要填写主题",
        }),
      );
    }
    if (!trimmedPrompt) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.body", {
          defaultValue: "发布前需要填写提示词正文",
        }),
      );
    }
    if (!trimmedInstructions) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.instructions", {
          defaultValue: "发布前需要补充要求",
        }),
      );
    }
    if (!model) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.model", {
          defaultValue: "发布前需要选择模型",
        }),
      );
    }
    if (positiveKeywords.length === 0) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.positiveKeywords", {
          defaultValue: "发布前至少保留一个正向关键词",
        }),
      );
    }
    if (negativeKeywords.length === 0) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.negativeKeywords", {
          defaultValue: "发布前至少保留一个负向关键词",
        }),
      );
    }
    if (tags.length === 0) {
      missingMessages.push(
        t("promptWorkbench.publishValidation.tags", {
          defaultValue: "发布前至少设置一个标签",
        }),
      );
    }
    if (missingMessages.length > 0) {
      missingMessages.forEach((message) => toast.warning(message));
      return false;
    }
    return true;
  }, [instructions, model, negativeKeywords, positiveKeywords, prompt, t, tags, toast, topic]);

  const savePromptMutation = useMutation({
    mutationFn: async (publish: boolean) => {
      const trimmedTopic = topic.trim();
      const trimmedPrompt = prompt.trim();
      const trimmedInstructions = instructions.trim();
      if (publish) {
        if (!trimmedTopic || !trimmedPrompt) {
          throw new ApiError({
            message: t("promptWorkbench.publishValidation.failed", {
              defaultValue: "发布条件未满足，请补全必填项",
            }),
          });
        }
      }
      setSaving(true);
      const payload: SavePromptRequest = {
        prompt_id:
          promptId && Number(promptId) > 0 ? Number(promptId) : undefined,
        topic: publish ? trimmedTopic : topic,
        body: publish ? trimmedPrompt : prompt,
        model,
        instructions: trimmedInstructions || undefined,
        publish,
        status: publish ? "published" : "draft",
        tags: tags.map((tag) => tag.value),
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
      lastSavedSignatureRef.current = autosaveSignatureRef.current;
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
    onSettled: () => {
      setSaving(false);
      if (autosavePendingRef.current) {
        autosavePendingRef.current = false;
        triggerAutoSaveRef.current();
      }
    },
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

  const triggerAutoSave = useCallback(() => {
    if (!topic.trim() || !prompt.trim()) {
      return;
    }
    if (autosaveSignatureRef.current === lastSavedSignatureRef.current) {
      return;
    }
    if (savePromptMutation.isPending) {
      autosavePendingRef.current = true;
      return;
    }
    autosavePendingRef.current = false;
    savePromptMutation.mutateAsync(false).catch(() => {
      // 错误提示已在 mutation 内统一处理
    });
  }, [prompt, savePromptMutation, topic]);

  useEffect(() => {
    triggerAutoSaveRef.current = triggerAutoSave;
  }, [triggerAutoSave]);

  useEffect(() => {
    if (autosaveTimerRef.current !== null) {
      window.clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }

    if (!topic.trim() || !prompt.trim()) {
      return;
    }

    if (autosaveFingerprint === lastSavedSignatureRef.current) {
      return;
    }

    if (savePromptMutation.isPending) {
      autosavePendingRef.current = true;
      return;
    }

    autosaveTimerRef.current = window.setTimeout(() => {
      autosaveTimerRef.current = null;
      triggerAutoSaveRef.current();
    }, PROMPT_AUTOSAVE_DELAY_MS);

    return () => {
      if (autosaveTimerRef.current !== null) {
        window.clearTimeout(autosaveTimerRef.current);
        autosaveTimerRef.current = null;
      }
    };
  }, [
    autosaveFingerprint,
    prompt,
    savePromptMutation.isPending,
    topic,
    PROMPT_AUTOSAVE_DELAY_MS,
  ]);

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
    const { value: clampedWord, overflow } = clampTextWithOverflow(
      word,
      keywordMaxLength,
    );
    const normalized = clampedWord.toLowerCase();
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
    if (overflow > 0) {
      toast.info(
        t("promptWorkbench.keywordTooLong", {
          limit: keywordMaxLength,
          overflow,
          defaultValue:
            "Keyword exceeded {{overflow}} characters and was truncated",
        }),
      );
    }
    manualKeywordMutation.mutate(clampedWord);
  };

  const handleInterpret = () => interpretMutation.mutate();
  const handleAugment = () => augmentMutation.mutate();
  const handleGenerate = () => generateMutation.mutate();
  const handleSaveDraft = () => savePromptMutation.mutate(false);
  const handlePublish = () => {
    if (!validatePublishRequirements()) {
      return;
    }
    savePromptMutation.mutate(true);
  };

  const handleAddTag = () => {
    const value = tagInput.trim();
    if (!value) return;
    const { value: clamped, overflow } = clampTextWithOverflow(
      value,
      tagMaxLength,
    );
    if (!clamped) return;
    const normalized = clamped.toLowerCase();
    const exists = tags.some(
      (tag) => tag.value.trim().toLowerCase() === normalized,
    );
    if (exists) {
      toast.warning(
        t("promptWorkbench.tagDuplicate", {
          defaultValue: "该标签已存在",
        }),
      );
      return;
    }
    if (tagLimit > 0 && tags.length >= tagLimit) {
      toast.warning(
        t("promptWorkbench.tagLimitReached", {
          limit: tagLimit,
          defaultValue: "标签最多 {{limit}} 个",
        }),
      );
      return;
    }
    if (overflow > 0) {
      toast.info(
        t("promptWorkbench.tagTooLong", {
          limit: tagMaxLength,
          overflow,
          defaultValue:
            "Tag exceeded {{overflow}} characters and was truncated",
        }),
      );
    }
    addTag(clamped);
    setTagInput("");
  };

  const handleRemoveTag = (tag: string) => {
    removeTag(tag);
  };

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
      markKeywordsDirty();
      return;
    }
    removeKeywordMutation.mutate(keyword);
  };

  const handleCancel = () => {
    reset();
    setDescription("");
    setInstructions("");
    setTagInput("");
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
    <div className="flex flex-col gap-6 text-slate-700 transition-colors dark:text-slate-200">
      <PageHeader
        eyebrow={t("promptWorkbench.workbenchEyebrow", {
          defaultValue: "Prompt 工作台",
        })}
        title={t("promptWorkbench.workbenchTitle", {
          defaultValue: "打造你的 Prompt 工作台",
        })}
        description={t("promptWorkbench.workbenchSubtitle", {
          defaultValue: "从解析需求到发布 Prompt 的一站式工作区。",
        })}
      />
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[360px_minmax(320px,360px)_minmax(0,1fr)] xl:items-start">
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
                    "workbench-pill group relative cursor-pointer overflow-hidden px-3 py-1 transition-colors",
                    option.disabled && "cursor-not-allowed opacity-60",
                    selected && "border-transparent bg-primary text-white",
                  )}
                  variant={selected ? "default" : "outline"}
                  onClick={() =>
                    !option.disabled && handleModelSelect(option.key)
                  }
                  aria-disabled={option.disabled}
                >
                  <span className="relative z-10">
                    {option.label}
                  </span>
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

        <header>
          <p className="text-xs uppercase tracking-[0.28em] text-slate-400 dark:text-slate-500">
            {t("promptWorkbench.keywordsTitle", { defaultValue: "关键词" })}
          </p>
          <h2 className="mt-1 text-xl font-semibold text-slate-800 dark:text-slate-100">
            {t("promptWorkbench.keywordsSubtitle", {
              defaultValue: "关键词治理",
            })}
          </h2>
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
              maxLength={keywordMaxLength * 2}
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
          <p className="text-xs text-slate-400 dark:text-slate-500">
            {t("promptWorkbench.keywordLengthHint", {
              limit: keywordMaxLength,
              defaultValue:
                "Each keyword can contain at most {{limit}} characters",
            })}
          </p>
          <div className="mt-3 flex gap-2 text-xs">
            <Badge
              className={cn(
                "workbench-pill group relative cursor-pointer overflow-hidden px-3 py-1 transition-colors",
                polarity === "positive" &&
                  "border-transparent bg-primary text-white",
              )}
              variant={polarity === "positive" ? "default" : "outline"}
              onClick={() => setPolarity("positive")}
            >
              <span className="relative z-10">
                {t("promptWorkbench.positive", { defaultValue: "正向" })}
              </span>
            </Badge>
            <Badge
              className={cn(
                "workbench-pill group relative cursor-pointer overflow-hidden px-3 py-1 transition-colors",
                polarity === "negative" &&
                  "border-transparent bg-secondary text-white",
              )}
              variant={polarity === "negative" ? "default" : "outline"}
              onClick={() => setPolarity("negative")}
            >
              <span className="relative z-10">
                {t("promptWorkbench.negative", { defaultValue: "负向" })}
              </span>
            </Badge>
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-3 text-sm text-slate-500 dark:text-slate-300">
            <span>{t("promptWorkbench.augmentSuggestion")}</span>
            <Button
              variant="secondary"
              size="sm"
              className="shadow-sm"
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
          </div>
        </div>

      </GlassCard>

      <GlassCard className="flex flex-col gap-5">
        <DndContext
          sensors={sensors}
          modifiers={[snapOverlayToCursor]}
          onDragStart={handleDragStart}
          onDragOver={handleDragOver}
          onDragEnd={handleDragEnd}
          onDragCancel={handleDragCancel}
        >
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
            onSort={() => sortByWeight("positive")}
            dropIndicatorIndex={
              dropIndicator?.polarity === "positive"
                ? dropIndicator.index
                : null
            }
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
            onSort={() => sortByWeight("negative")}
            dropIndicatorIndex={
              dropIndicator?.polarity === "negative"
                ? dropIndicator.index
                : null
            }
            onWeightChange={handleWeightChange}
            onRemove={handleRemoveKeyword}
          />
          <DragOverlay dropAnimation={null}>
            {activeKeyword ? (
              <KeywordDragPreview keyword={activeKeyword} />
            ) : null}
          </DragOverlay>
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
          <Button
            variant="secondary"
            size="sm"
            className="shadow-sm"
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
          <div className="flex flex-col gap-5">
            <div className="flex flex-col">
              <label className="text-sm font-medium text-slate-600 dark:text-slate-300">
                {t("promptWorkbench.instructionsLabel", {
                  defaultValue: "补充要求",
                })}
              </label>
              <Textarea
                className="mt-3 min-h-[120px]"
                value={instructions}
                onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
                  setInstructions(event.target.value)
                }
                placeholder={t("promptWorkbench.instructionsPlaceholder", {
                  defaultValue: "可选：语气、结构、输出格式等",
                })}
              />
            </div>
            <div className="flex flex-col">
              <div className="flex items-center justify-between gap-2">
                <label className="text-sm font-medium text-slate-600 dark:text-slate-300">
                  {t("promptWorkbench.tagsLabel", { defaultValue: "标签" })}
                </label>
                <span className="text-xs text-slate-400 dark:text-slate-500">
                  {t("promptWorkbench.tagLimitHint", {
                    limit: tagLimit,
                    defaultValue: "最多 {{limit}} 个标签",
                  })}
                </span>
              </div>
              <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                <Input
                  value={tagInput}
                  onChange={(event: ChangeEvent<HTMLInputElement>) =>
                    setTagInput(event.target.value)
                  }
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      handleAddTag();
                    }
                  }}
                  placeholder={t("promptWorkbench.tagsPlaceholder", {
                    defaultValue: "输入标签后按 Enter",
                  })}
                  maxLength={tagMaxLength * 2}
                />
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="sm:w-auto sm:min-w-[120px] sm:justify-center sm:px-5"
                  onClick={handleAddTag}
                  disabled={tagLimit > 0 && tags.length >= tagLimit}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  {t("promptWorkbench.addTag", { defaultValue: "添加标签" })}
                </Button>
              </div>
              <p className="mt-2 text-xs text-slate-400 dark:text-slate-500">
                {t("promptWorkbench.tagsHelper", {
                  maxLength: tagMaxLength,
                  defaultValue:
                    "Use tags to organise prompts. Press Enter to add, each tag can contain at most {{maxLength}} characters",
                })}
              </p>
              <div className="mt-2 flex flex-wrap gap-2">
                {tags.length === 0 ? (
                  <span className="text-xs text-slate-400 dark:text-slate-500">
                    {t("promptWorkbench.tagsEmptyHint", {
                      defaultValue: "暂无标签，可在上方输入添加",
                    })}
                  </span>
                ) : (
                  tags.map((tag) => (
                    <Badge
                      key={tag.value}
                      variant="outline"
                      className="flex items-center gap-1 rounded-xl border border-white/70 bg-white/80 px-3 py-1 text-xs font-medium text-slate-600 shadow-sm transition-colors dark:border-slate-700 dark:bg-slate-800/70 dark:text-slate-200"
                    >
                      <span title={tag.value}>
                        {formatOverflowLabel(tag.value, tag.overflow)}
                      </span>
                      <button
                        type="button"
                        className="rounded-full bg-white/80 p-0.5 text-slate-400 transition hover:bg-white hover:text-slate-600 focus:outline-none dark:bg-slate-900/60 dark:hover:bg-slate-800/70"
                        onClick={() => handleRemoveTag(tag.value)}
                        aria-label={t("promptWorkbench.tagRemoveAria", {
                          tag: tag.value,
                          defaultValue: "移除标签",
                        })}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))
                )}
              </div>
            </div>
          </div>
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
  </div>
);
}

interface KeywordSectionProps {
  title: string;
  hint?: string;
  keywords: Keyword[];
  polarity: "positive" | "negative";
  droppableId: string;
  onSort?: () => void;
  dropIndicatorIndex?: number | null;
  onWeightChange: (id: string, weight: number) => void;
  onRemove: (keyword: Keyword) => void;
}

function KeywordSection({
  title,
  hint,
  keywords,
  polarity,
  droppableId,
  onSort,
  dropIndicatorIndex,
  onWeightChange,
  onRemove,
}: KeywordSectionProps) {
  const { t } = useTranslation();
  const { setNodeRef, isOver } = useDroppable({ id: droppableId });
  const indicatorIndex = typeof dropIndicatorIndex === "number" ? dropIndicatorIndex : -1;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-slate-500 dark:text-slate-400">
        <span className="flex-1 min-w-[140px]">{title}</span>
        {onSort ? (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="ml-auto h-7 rounded-lg px-2 text-xs font-medium text-primary shadow-sm transition-colors hover:bg-primary/15 hover:text-primary dark:bg-primary/20 dark:text-primary-100 dark:hover:bg-primary/25"
            onClick={onSort}
          >
            {t("promptWorkbench.sortByWeight", {
              defaultValue: "按权重排序",
            })}
          </Button>
        ) : null}
      </div>
      {hint ? (
        <p className="text-xs text-slate-400 dark:text-slate-500">{hint}</p>
      ) : null}
      <div
        ref={setNodeRef}
        className={cn(
          "rounded-2xl border border-white/60 bg-white/80 p-3 transition-all duration-200 dark:border-slate-800 dark:bg-slate-900/70",
          isOver &&
            "border-primary/70 bg-primary/10 shadow-[0_0_0_3px_rgba(59,130,246,0.12)] dark:border-primary/70 dark:bg-primary/10",
          indicatorIndex >= 0 &&
            "border-primary/60 bg-primary/5 dark:border-primary/60 dark:bg-primary/5",
        )}
      >
        <SortableContext
          items={keywords.map((keyword) => keyword.id)}
          strategy={verticalListSortingStrategy}
        >
          {keywords.length === 0 ? (
            <div className="flex h-16 flex-col items-center justify-center gap-2 text-xs text-slate-400 dark:text-slate-500">
              <span>
                {t("promptWorkbench.keywordDropPlaceholder", {
                  defaultValue: "将关键词拖拽到此处",
                })}
              </span>
              {indicatorIndex === 0 ? <KeywordDropIndicator /> : null}
            </div>
          ) : (
            <div className="space-y-2">
              {keywords.map((keyword, index) => (
                <Fragment key={keyword.id}>
                  {indicatorIndex === index ? <KeywordDropIndicator /> : null}
                  <SortableKeywordChip
                    keyword={keyword}
                    polarity={polarity}
                    onWeightChange={onWeightChange}
                    onRemove={onRemove}
                  />
                </Fragment>
              ))}
              {indicatorIndex === keywords.length ? (
                <KeywordDropIndicator />
              ) : null}
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
    position: "relative" as const,
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 30 : undefined,
  };
  const isPositive = polarity === "positive";
  const displayWord = formatOverflowLabel(keyword.word, keyword.overflow ?? 0);
  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        "group relative flex items-center gap-3 rounded-2xl border px-3 py-2 text-sm shadow-sm transition-colors transition-transform duration-150 ease-out",
        isPositive
          ? "border-primary/30 bg-primary/10 text-primary"
          : "border-secondary/30 bg-secondary/10 text-secondary",
        isDragging && "scale-[1.02] border-dashed opacity-80 shadow-md",
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
        <span
          className="text-sm font-medium text-slate-700 dark:text-slate-100"
          title={keyword.word}
        >
          {displayWord}
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
            <span className="min-w-[64px] text-center text-[11px] font-semibold text-slate-600 dark:text-slate-200">
              {t("promptWorkbench.weightDisplay", {
                value: keyword.weight,
                defaultValue: `${keyword.weight}/5`,
              })}
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
          <span className="rounded-full bg-white/80 px-2 py-0.5 text-[10px] text-slate-500 dark:bg-slate-800/60 dark:text-slate-200">
            {t(`promptWorkbench.sourceLabels.${keyword.source ?? "model"}`, {
              defaultValue: keyword.source ?? "model",
            })}
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

function KeywordDropIndicator() {
  return (
    <div
      className="h-1 w-full rounded-full bg-primary/60 transition-all duration-150 dark:bg-primary/50"
      role="presentation"
    />
  );
}

function KeywordDragPreview({ keyword }: { keyword: Keyword }) {
  const { t } = useTranslation();
  const isPositive = keyword.polarity === "positive";
  const displayWord = formatOverflowLabel(keyword.word, keyword.overflow ?? 0);
  return (
    <div
      className={cn(
        "pointer-events-none flex w-[240px] items-center gap-3 rounded-2xl border px-3 py-2 text-sm shadow-lg",
        isPositive
          ? "border-primary/30 bg-primary/90 text-white"
          : "border-secondary/30 bg-secondary/90 text-white",
      )}
    >
      <GripVertical className="h-3.5 w-3.5 opacity-80" />
      <div className="flex flex-1 flex-col gap-1 text-left">
        <span className="text-sm font-medium" title={keyword.word}>
          {displayWord}
        </span>
        <span className="text-xs opacity-80">
          {t("promptWorkbench.weightDisplay", {
            value: clampWeight(keyword.weight),
            defaultValue: `权重 ${clampWeight(keyword.weight)}`,
          })}
        </span>
      </div>
      <span className="rounded-full bg-white/20 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide">
        {t(`promptWorkbench.sourceLabels.${keyword.source ?? "model"}`, {
          defaultValue: keyword.source ?? "model",
        })}
      </span>
    </div>
  );
}
