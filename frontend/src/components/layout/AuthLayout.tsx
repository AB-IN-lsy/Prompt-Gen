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
        <div className="flex min-h-screen flex-col bg-[var(--bg)] text-[var(--fg)] transition-colors">
            <div className="flex flex-1 flex-col items-center justify-center px-4 py-12">
                <div className="mb-12 flex items-center gap-3 text-primary">
                    <Sparkles className="h-8 w-8" />
                    <span className="text-2xl font-semibold">{t("app.name")}</span>
                </div>
                <div className="w-full max-w-md space-y-6 rounded-3xl border border-white/60 bg-white/80 p-8 shadow-xl backdrop-blur-xl transition-colors dark:border-slate-800/70 dark:bg-slate-900/70">
                    <header className="space-y-2 text-center">
                        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">{title}</h1>
                        {subtitle ? <p className="text-sm text-slate-500 dark:text-slate-400">{subtitle}</p> : null}
                    </header>
                    <div className="text-slate-700 transition-colors dark:text-slate-200">{children}</div>
                </div>
            </div>
            <footer className="border-t border-white/60 px-4 py-4 text-xs text-slate-400 transition-colors dark:border-slate-800/70 dark:text-slate-500">
                <div className="mx-auto flex w-full max-w-4xl flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <span>Powered by AB-IN · 鲁ICP备2021035431号-1</span>
                    <a
                        href="https://ab-in.blog.csdn.net/"
                        target="_blank"
                        rel="noreferrer"
                        className="text-slate-500 underline-offset-4 hover:text-primary hover:underline dark:text-slate-400"
                    >
                        AB-IN CSDN 博客 · ab-in.blog.csdn.net
                    </a>
                </div>
            </footer>
        </div>
    );
}
