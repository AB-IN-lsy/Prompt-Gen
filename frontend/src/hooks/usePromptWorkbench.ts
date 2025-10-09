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
  model: "deepseek" | "gpt-5";
  prompt: string;
  positiveKeywords: Keyword[];
  negativeKeywords: Keyword[];
  isSaving: boolean;
  setTopic: (topic: string) => void;
  setModel: (model: WorkbenchState["model"]) => void;
  setPrompt: (prompt: string) => void;
  setKeywords: (keywords: Keyword[]) => void;
  addKeyword: (keyword: Omit<Keyword, "id">) => void;
  updateKeyword: (id: string, partial: Partial<Keyword>) => void;
  removeKeyword: (id: string) => void;
  setSaving: (saving: boolean) => void;
  reset: () => void;
}

export const usePromptWorkbench = create<WorkbenchState>((set, get) => ({
  topic: "前端工程师面试",
  model: "deepseek",
  prompt: "",
  positiveKeywords: [],
  negativeKeywords: [],
  isSaving: false,
  setTopic: (topic) => set({ topic }),
  setModel: (model) => set({ model }),
  setPrompt: (prompt) => set({ prompt }),
  setKeywords: (keywords) =>
    set({
      positiveKeywords: keywords.filter((item) => item.polarity === "positive"),
      negativeKeywords: keywords.filter((item) => item.polarity === "negative")
    }),
  addKeyword: (keyword) => {
    const id = nanoid();
    const entry: Keyword = { ...keyword, id };
    if (entry.polarity === "positive") {
      set({ positiveKeywords: [...get().positiveKeywords, entry] });
    } else {
      set({ negativeKeywords: [...get().negativeKeywords, entry] });
    }
  },
  updateKeyword: (id, partial) => {
    set(({ positiveKeywords, negativeKeywords }) => ({
      positiveKeywords: positiveKeywords.map((item) => (item.id === id ? { ...item, ...partial } : item)),
      negativeKeywords: negativeKeywords.map((item) => (item.id === id ? { ...item, ...partial } : item))
    }));
  },
  removeKeyword: (id) => {
    set(({ positiveKeywords, negativeKeywords }) => ({
      positiveKeywords: positiveKeywords.filter((item) => item.id !== id),
      negativeKeywords: negativeKeywords.filter((item) => item.id !== id)
    }));
  },
  setSaving: (saving) => set({ isSaving: saving }),
  reset: () =>
    set({
      topic: "",
      model: "deepseek",
      prompt: "",
      positiveKeywords: [],
      negativeKeywords: []
    })
}));
