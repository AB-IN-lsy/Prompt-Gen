import { jsx as _jsx } from "react/jsx-runtime";
import { cn } from "../../lib/utils";
export function GlassCard({ children, className }) {
    return (_jsx("div", { className: cn("glass rounded-3xl border border-white/50 bg-white/70 p-6 shadow-elevation transition-colors dark:border-slate-800/70 dark:bg-slate-900/60 dark:shadow-none", className), children: children }));
}
