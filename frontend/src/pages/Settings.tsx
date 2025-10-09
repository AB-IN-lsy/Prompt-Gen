/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:33:13
 * @FilePath: \electron-go-app\frontend\src\pages\Settings.tsx
 * @LastEditTime: 2025-10-09 23:33:18
 */
import { ChangeEvent } from "react";
import { useTranslation } from "react-i18next";

import { GlassCard } from "../components/ui/glass-card";
import { useAppSettings } from "../hooks/useAppSettings";
import { LANGUAGE_OPTIONS } from "../i18n";

export default function SettingsPage() {
    const { t } = useTranslation();
    const { language, setLanguage } = useAppSettings();

    // 语言切换下拉框，限制为 LANGUAGE_OPTIONS 白名单。
    const handleLanguageChange = (event: ChangeEvent<HTMLSelectElement>) => {
        const value = event.target.value;
        if (LANGUAGE_OPTIONS.some((option) => option.code === value)) {
            setLanguage(value as typeof LANGUAGE_OPTIONS[number]["code"]);
        }
    };

    return (
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-6">
            <header className="flex flex-col gap-1">
                <h1 className="text-2xl font-semibold text-slate-800">{t("settings.title")}</h1>
                <p className="text-sm text-slate-500">{t("settings.languageCardDescription")}</p>
            </header>

            <GlassCard className="space-y-4">
                <div>
                    <h2 className="text-lg font-medium text-slate-800">{t("settings.languageCardTitle")}</h2>
                    <p className="mt-1 text-sm text-slate-500">{t("settings.languageCardDescription")}</p>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <label className="text-sm font-medium text-slate-600" htmlFor="language-select">
                        {t("settings.languageSelectLabel")}
                    </label>
                    <select
                        id="language-select"
                        className="w-full rounded-xl border border-white/60 bg-white/80 px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/40 sm:w-64"
                        value={language}
                        onChange={handleLanguageChange}
                    >
                        {LANGUAGE_OPTIONS.map((option) => (
                            <option key={option.code} value={option.code}>
                                {option.label}
                            </option>
                        ))}
                    </select>
                </div>
            </GlassCard>
        </div>
    );
}
