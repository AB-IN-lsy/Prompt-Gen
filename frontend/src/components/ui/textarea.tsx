/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:46:25
 * @FilePath: \electron-go-app\frontend\src\components\ui\textarea.tsx
 * @LastEditTime: 2025-10-09 22:46:29
 */
import { forwardRef, TextareaHTMLAttributes } from "react";
import { cn } from "../../lib/utils";

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> { }

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(({ className, ...props }, ref) => (
    <textarea
        ref={ref}
        className={cn(
            "min-h-[160px] w-full rounded-2xl border border-white/70 bg-white/80 px-4 py-3 text-sm text-slate-700 shadow-sm transition placeholder:text-slate-400 focus:border-primary focus:shadow-glow focus:outline-none dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200 dark:placeholder:text-slate-500",
            className
        )}
        {...props}
    />
));

Textarea.displayName = "Textarea";
