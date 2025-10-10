import { jsx as _jsx } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:45:37
 * @FilePath: \electron-go-app\frontend\src\components\ui\input.tsx
 * @LastEditTime: 2025-10-09 22:45:42
 */
import { forwardRef } from "react";
import { cn } from "../../lib/utils";
export const Input = forwardRef(({ className, type, ...props }, ref) => {
    return (_jsx("input", { type: type, className: cn("h-10 w-full rounded-xl border border-white/70 bg-white/80 px-3 text-sm text-slate-700 shadow-sm transition placeholder:text-slate-400 focus:border-primary focus:shadow-glow focus:outline-none dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200 dark:placeholder:text-slate-500 dark:focus:border-primary/60", className), ref: ref, ...props }));
});
Input.displayName = "Input";
