import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useTranslation } from "react-i18next";
import { GlassCard } from "../components/ui/glass-card";
import { useAppSettings } from "../hooks/useAppSettings";
import { LANGUAGE_OPTIONS } from "../i18n";
export default function SettingsPage() {
    const { t } = useTranslation();
    const { language, setLanguage } = useAppSettings();
    // 语言切换下拉框，限制为 LANGUAGE_OPTIONS 白名单。
    const handleLanguageChange = (event) => {
        const value = event.target.value;
        if (LANGUAGE_OPTIONS.some((option) => option.code === value)) {
            setLanguage(value);
        }
    };
    return (_jsxs("div", { className: "mx-auto flex w-full max-w-4xl flex-col gap-6", children: [_jsxs("header", { className: "flex flex-col gap-1", children: [_jsx("h1", { className: "text-2xl font-semibold text-slate-800", children: t("settings.title") }), _jsx("p", { className: "text-sm text-slate-500", children: t("settings.languageCardDescription") })] }), _jsxs(GlassCard, { className: "space-y-4", children: [_jsxs("div", { children: [_jsx("h2", { className: "text-lg font-medium text-slate-800", children: t("settings.languageCardTitle") }), _jsx("p", { className: "mt-1 text-sm text-slate-500", children: t("settings.languageCardDescription") })] }), _jsxs("div", { className: "flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between", children: [_jsx("label", { className: "text-sm font-medium text-slate-600", htmlFor: "language-select", children: t("settings.languageSelectLabel") }), _jsx("select", { id: "language-select", className: "w-full rounded-xl border border-white/60 bg-white/80 px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/40 sm:w-64", value: language, onChange: handleLanguageChange, children: LANGUAGE_OPTIONS.map((option) => (_jsx("option", { value: option.code, children: option.label }, option.code))) })] })] })] }));
}
