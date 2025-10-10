import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
export function AuthLayout({ title, subtitle, children }) {
    const { t } = useTranslation();
    return (_jsxs("div", { className: "flex min-h-screen flex-col items-center justify-center bg-[var(--bg)] px-4 py-12 text-[var(--fg)] transition-colors", children: [_jsxs("div", { className: "mb-12 flex items-center gap-3 text-primary", children: [_jsx(Sparkles, { className: "h-8 w-8" }), _jsx("span", { className: "text-2xl font-semibold", children: t("app.name") })] }), _jsxs("div", { className: "w-full max-w-md space-y-6 rounded-3xl border border-white/60 bg-white/80 p-8 shadow-xl backdrop-blur-xl transition-colors dark:border-slate-800/70 dark:bg-slate-900/70", children: [_jsxs("header", { className: "space-y-2 text-center", children: [_jsx("h1", { className: "text-2xl font-semibold text-slate-800 dark:text-slate-100", children: title }), subtitle ? _jsx("p", { className: "text-sm text-slate-500 dark:text-slate-400", children: subtitle }) : null] }), _jsx("div", { className: "text-slate-700 transition-colors dark:text-slate-200", children: children })] })] }));
}
