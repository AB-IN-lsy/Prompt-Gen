/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:45:13
 * @FilePath: \electron-go-app\frontend\src\hooks\usePromptWorkbench.ts
 * @LastEditTime: 2025-10-09 22:45:18
 */
import { create } from "zustand";
import { nanoid } from "nanoid";
import { Keyword } from "../lib/api";
import { PROMPT_KEYWORD_LIMIT } from "../config/prompt";

interface WorkbenchState {
  topic: string;
  model: string;
  prompt: string;
  promptId: string | null;
  workspaceToken: string | null;
  positiveKeywords: Keyword[];
  negativeKeywords: Keyword[];
  isSaving: boolean;
  setTopic: (topic: string) => void;
  setModel: (model: string) => void;
  setPrompt: (prompt: string) => void;
  setPromptId: (id: string | null) => void;
  setWorkspaceToken: (token: string | null) => void;
  setKeywords: (keywords: Keyword[]) => void;
  setCollections: (positive: Keyword[], negative: Keyword[]) => void;
  addKeyword: (keyword: Keyword) => void;
  upsertKeyword: (keyword: Keyword) => void;
  updateKeyword: (id: string, updater: (keyword: Keyword) => Keyword) => void;
  removeKeyword: (id: string) => void;
  setSaving: (saving: boolean) => void;
  reset: () => void;
}

const KEYWORD_LIMIT = PROMPT_KEYWORD_LIMIT;
const DEFAULT_KEYWORD_WEIGHT = 5;

const normalizeWord = (word: string) => word.trim().toLowerCase();

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

const dedupeByPolarity = (keywords: Keyword[]) => {
  const seen = new Map<string, Keyword>();
  keywords.forEach((item) => {
    const key = `${item.polarity}:${normalizeWord(item.word)}`;
    if (!seen.has(key)) {
      seen.set(key, item);
    }
  });
  return Array.from(seen.values());
};

const normaliseKeyword = (keyword: Keyword): Keyword => {
  if (!keyword.id || keyword.id.length === 0) {
    return {
      ...keyword,
      id: nanoid(),
      weight: clampWeight(keyword.weight),
    };
  }
  return { ...keyword, weight: clampWeight(keyword.weight) };
};

export const usePromptWorkbench = create<WorkbenchState>((set, get) => ({
  topic: "",
  model: "",
  prompt: "",
  promptId: null,
  workspaceToken: null,
  positiveKeywords: [],
  negativeKeywords: [],
  isSaving: false,
  setTopic: (topic) => set({ topic }),
  setModel: (model) => set({ model }),
  setPrompt: (prompt) => set({ prompt }),
  setPromptId: (promptId) => set({ promptId }),
  setWorkspaceToken: (workspaceToken) => set({ workspaceToken }),
  setKeywords: (keywords) =>
    set({
      positiveKeywords: dedupeByPolarity(
        keywords
          .filter((item) => item.polarity === "positive")
          .map(normaliseKeyword),
      ).slice(0, KEYWORD_LIMIT),
      negativeKeywords: dedupeByPolarity(
        keywords
          .filter((item) => item.polarity === "negative")
          .map(normaliseKeyword),
      ).slice(0, KEYWORD_LIMIT),
    }),
  setCollections: (positive, negative) =>
    set({
      positiveKeywords: positive.map(normaliseKeyword).slice(0, KEYWORD_LIMIT),
      negativeKeywords: negative.map(normaliseKeyword).slice(0, KEYWORD_LIMIT),
    }),
  addKeyword: (keyword) => {
    const entry = normaliseKeyword(keyword);
    if (entry.polarity === "positive") {
      set(({ positiveKeywords }) => {
        const duplicated = positiveKeywords.some(
          (item) => normalizeWord(item.word) === normalizeWord(entry.word),
        );
        if (duplicated) {
          return { positiveKeywords };
        }
        if (positiveKeywords.length >= KEYWORD_LIMIT) {
          return { positiveKeywords };
        }
        return { positiveKeywords: [...positiveKeywords, entry] };
      });
    } else {
      set(({ negativeKeywords }) => {
        const duplicated = negativeKeywords.some(
          (item) => normalizeWord(item.word) === normalizeWord(entry.word),
        );
        if (duplicated) {
          return { negativeKeywords };
        }
        if (negativeKeywords.length >= KEYWORD_LIMIT) {
          return { negativeKeywords };
        }
        return { negativeKeywords: [...negativeKeywords, entry] };
      });
    }
  },
  upsertKeyword: (keyword) => {
    const entry = normaliseKeyword(keyword);
    set(({ positiveKeywords, negativeKeywords }) => {
      const updateCollection = (collection: Keyword[]) => {
        const idx = collection.findIndex(
          (item) =>
            item.id === entry.id ||
            (item.polarity === entry.polarity &&
              normalizeWord(item.word) === normalizeWord(entry.word)),
        );
        if (idx >= 0) {
          const next = [...collection];
          next[idx] = { ...collection[idx], ...entry };
          return next;
        }
        return [...collection, entry];
      };
      if (entry.polarity === "positive") {
        const updated = updateCollection(positiveKeywords);
        return {
          positiveKeywords: updated.slice(0, KEYWORD_LIMIT),
          negativeKeywords,
        };
      }
      return {
        positiveKeywords,
        negativeKeywords: updateCollection(negativeKeywords).slice(
          0,
          KEYWORD_LIMIT,
        ),
      };
    });
  },
  updateKeyword: (id, updater) => {
    set(({ positiveKeywords, negativeKeywords }) => {
      const updateList = (collection: Keyword[]) => {
        const idx = collection.findIndex((item) => item.id === id);
        if (idx === -1) {
          return collection;
        }
        const next = [...collection];
        next[idx] = normaliseKeyword(updater(collection[idx]));
        return next;
      };
      const updatedPositive = updateList(positiveKeywords);
      if (updatedPositive !== positiveKeywords) {
        return { positiveKeywords: updatedPositive, negativeKeywords };
      }
      return {
        positiveKeywords,
        negativeKeywords: updateList(negativeKeywords),
      };
    });
  },
  removeKeyword: (id) => {
    set(({ positiveKeywords, negativeKeywords }) => ({
      positiveKeywords: positiveKeywords.filter((item) => item.id !== id),
      negativeKeywords: negativeKeywords.filter((item) => item.id !== id),
    }));
  },
  setSaving: (saving) => set({ isSaving: saving }),
  reset: () =>
    set({
      topic: "",
      model: "",
      prompt: "",
      promptId: null,
      workspaceToken: null,
      positiveKeywords: [],
      negativeKeywords: [],
    }),
}));
