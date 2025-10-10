/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:27:25
 * @FilePath: \electron-go-app\frontend\src\hooks\useAppSettings.ts
 * @LastEditTime: 2025-10-10 20:49:33
 */
import { create } from "zustand";
import i18n, { persistLanguage, type SupportedLanguage } from "../i18n";

type ThemePreference = "light" | "dark" | "system";
type ResolvedTheme = "light" | "dark";

interface AppSettingsState {
  language: SupportedLanguage;
  theme: ThemePreference;
  resolvedTheme: ResolvedTheme;
  setLanguage: (language: SupportedLanguage) => void;
  setTheme: (theme: ThemePreference) => void;
}

const initialLanguage = i18n.language as SupportedLanguage;

const THEME_STORAGE_KEY = "promptgen.settings.theme";
const DEFAULT_THEME: ThemePreference = "system";

function detectSystemTheme(): ResolvedTheme {
  if (typeof window === "undefined") {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyThemeClass(theme: ResolvedTheme): void {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  if (theme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}

function loadInitialTheme(): { preference: ThemePreference; resolved: ResolvedTheme } {
  if (typeof window === "undefined") {
    return { preference: DEFAULT_THEME, resolved: "light" };
  }

  const stored = window.localStorage.getItem(THEME_STORAGE_KEY) as ThemePreference | null;
  const preference: ThemePreference = stored ?? DEFAULT_THEME;
  const resolved = preference === "system" ? detectSystemTheme() : preference;
  applyThemeClass(resolved);
  return { preference, resolved };
}

export const useAppSettings = create<AppSettingsState>((set, get) => {
  const { preference, resolved } = loadInitialTheme();

  if (typeof window !== "undefined") {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    // 当系统主题变更时，若用户选择跟随系统则即时切换。
    const handler = (event: MediaQueryListEvent): void => {
      if (get().theme !== "system") {
        return;
      }
      const nextTheme: ResolvedTheme = event.matches ? "dark" : "light";
      applyThemeClass(nextTheme);
      set({ resolvedTheme: nextTheme });
    };

    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", handler);
    } else {
      // 针对旧版 Safari 的兼容处理。
      media.addListener(handler);
    }
  }

  return {
    language: initialLanguage,
    theme: preference,
    resolvedTheme: resolved,
    setLanguage: (language) => {
      // 触发 i18next 切换并写入 localStorage，保证 UI 与刷新后的状态一致。
      i18n.changeLanguage(language);
      persistLanguage(language);
      set({ language });
    },
    setTheme: (theme) => {
      const resolvedTheme: ResolvedTheme = theme === "system" ? detectSystemTheme() : theme;
      if (typeof window !== "undefined") {
        window.localStorage.setItem(THEME_STORAGE_KEY, theme);
      }
      applyThemeClass(resolvedTheme);
      set({ theme, resolvedTheme });
    },
  };
});
