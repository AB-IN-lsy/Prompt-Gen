/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:55:12
 * @FilePath: \electron-go-app\frontend\src\components\layout\AuthLayout.tsx
 * @LastEditTime: 2025-10-09 23:55:16
 */
import { ReactNode } from "react";
import { Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

interface AuthLayoutProps {
    title: string;
    subtitle?: string;
    children: ReactNode;
}

export function AuthLayout({ title, subtitle, children }: AuthLayoutProps) {
    const { t } = useTranslation();

    return (
        <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-br from-[#F8F9FA] via-[#EEF2FF] to-[#E9EDFF] px-4 py-12">
            <div className="mb-12 flex items-center gap-3 text-primary">
                <Sparkles className="h-8 w-8" />
                <span className="text-2xl font-semibold">{t("app.name")}</span>
            </div>
            <div className="w-full max-w-md space-y-6 rounded-3xl border border-white/60 bg-white/80 p-8 shadow-xl backdrop-blur-xl">
                <header className="space-y-2 text-center">
                    <h1 className="text-2xl font-semibold text-slate-800">{title}</h1>
                    {subtitle ? <p className="text-sm text-slate-500">{subtitle}</p> : null}
                </header>
                <div>{children}</div>
            </div>
        </div>
    );
}
