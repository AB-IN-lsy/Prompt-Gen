/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:26:18
 * @FilePath: \electron-go-app\frontend\src\i18n\index.ts
 * @LastEditTime: 2025-10-09 23:26:23
 */
/*
 * @fileoverview Initialises the i18next instance with PromptGen translations.
 */
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./locales/en.json";
import zhCN from "./locales/zh-CN.json";
// UI 支持的语言清单；label 用于设置页展示。
export const LANGUAGE_OPTIONS = [
    { code: "zh-CN", label: "简体中文" },
    { code: "en", label: "English" },
];
// 通过 localStorage 持久化语言首选项。
const STORAGE_KEY = "promptgen.settings.language";
const FALLBACK_LANGUAGE = "zh-CN";
function detectInitialLanguage() {
    if (typeof window === "undefined") {
        return FALLBACK_LANGUAGE;
    }
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored && LANGUAGE_OPTIONS.some((option) => option.code === stored)) {
        return stored;
    }
    // 如果用户没有显式选择，尝试根据浏览器语言推断。
    const navigatorLang = window.navigator.language;
    if (navigatorLang.toLowerCase().startsWith("zh")) {
        return "zh-CN";
    }
    return FALLBACK_LANGUAGE;
}
const initialLanguage = detectInitialLanguage();
if (!i18n.isInitialized) {
    // 初始化 i18next，将翻译资源注入并关闭 escapeValue 以适配 React。
    i18n.use(initReactI18next).init({
        resources: {
            en: { translation: en },
            "zh-CN": { translation: zhCN },
        },
        lng: initialLanguage,
        fallbackLng: FALLBACK_LANGUAGE,
        interpolation: {
            escapeValue: false,
        },
        returnNull: false,
        returnEmptyString: false,
    });
}
export function persistLanguage(language) {
    if (typeof window === "undefined") {
        return;
    }
    // 切换语言时同步更新浏览器存储，保证刷新后保持一致。
    window.localStorage.setItem(STORAGE_KEY, language);
}
export default i18n;
