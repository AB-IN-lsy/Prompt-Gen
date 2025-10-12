/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:45:13
 * @FilePath: \electron-go-app\frontend\src\hooks\usePromptWorkbench.ts
 * @LastEditTime: 2025-10-09 22:45:18
 */
import { create } from "zustand";
import { nanoid } from "nanoid";
import { Keyword } from "../lib/api";

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
      positiveKeywords: keywords
        .filter((item) => item.polarity === "positive")
        .map(normaliseKeyword),
      negativeKeywords: keywords
        .filter((item) => item.polarity === "negative")
        .map(normaliseKeyword),
    }),
  addKeyword: (keyword) => {
    const entry = normaliseKeyword(keyword);
    if (entry.polarity === "positive") {
      set({ positiveKeywords: [...get().positiveKeywords, entry] });
    } else {
      set({ negativeKeywords: [...get().negativeKeywords, entry] });
    }
  },
  upsertKeyword: (keyword) => {
    const entry = normaliseKeyword(keyword);
    set(({ positiveKeywords, negativeKeywords }) => {
      const updateCollection = (collection: Keyword[]) => {
        const idx = collection.findIndex((item) => item.id === entry.id);
        if (idx >= 0) {
          const next = [...collection];
          next[idx] = { ...collection[idx], ...entry };
          return next;
        }
        return [...collection, entry];
      };
      if (entry.polarity === "positive") {
        return {
          positiveKeywords: updateCollection(positiveKeywords),
          negativeKeywords,
        };
      }
      return {
        positiveKeywords,
        negativeKeywords: updateCollection(negativeKeywords),
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
