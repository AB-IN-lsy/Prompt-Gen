/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:45:13
 * @FilePath: \electron-go-app\frontend\src\hooks\usePromptWorkbench.ts
 * @LastEditTime: 2025-10-09 22:45:18
 */
import { create } from "zustand";
import { nanoid } from "nanoid";
export const usePromptWorkbench = create((set, get) => ({
    topic: "前端工程师面试",
    model: "deepseek",
    prompt: "",
    positiveKeywords: [],
    negativeKeywords: [],
    isSaving: false,
    setTopic: (topic) => set({ topic }),
    setModel: (model) => set({ model }),
    setPrompt: (prompt) => set({ prompt }),
    setKeywords: (keywords) => set({
        positiveKeywords: keywords.filter((item) => item.polarity === "positive"),
        negativeKeywords: keywords.filter((item) => item.polarity === "negative")
    }),
    addKeyword: (keyword) => {
        const id = nanoid();
        const entry = { ...keyword, id };
        if (entry.polarity === "positive") {
            set({ positiveKeywords: [...get().positiveKeywords, entry] });
        }
        else {
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
    reset: () => set({
        topic: "",
        model: "deepseek",
        prompt: "",
        positiveKeywords: [],
        negativeKeywords: []
    })
}));
