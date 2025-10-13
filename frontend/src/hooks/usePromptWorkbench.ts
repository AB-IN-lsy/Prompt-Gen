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
  addKeyword: (keyword: Keyword) => void;
  upsertKeyword: (keyword: Keyword) => void;
  removeKeyword: (id: string) => void;
  setSaving: (saving: boolean) => void;
  reset: () => void;
}

const KEYWORD_LIMIT = PROMPT_KEYWORD_LIMIT;

const normalizeWord = (word: string) => word.trim().toLowerCase();

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
    return { ...keyword, id: nanoid() };
  }
  return keyword;
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
