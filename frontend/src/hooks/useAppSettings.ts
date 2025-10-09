/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:27:25
 * @FilePath: \electron-go-app\frontend\src\hooks\useAppSettings.ts
 * @LastEditTime: 2025-10-09 23:27:29
 */
import { create } from "zustand";
import i18n, { persistLanguage, type SupportedLanguage } from "../i18n";

interface AppSettingsState {
  language: SupportedLanguage;
  setLanguage: (language: SupportedLanguage) => void;
}

const initialLanguage = i18n.language as SupportedLanguage;

export const useAppSettings = create<AppSettingsState>((set) => ({
  language: initialLanguage,
  setLanguage: (language) => {
    // 触发 i18next 切换并写入 localStorage，保证 UI 与刷新后的状态一致。
    i18n.changeLanguage(language);
    persistLanguage(language);
    set({ language });
  },
}));
