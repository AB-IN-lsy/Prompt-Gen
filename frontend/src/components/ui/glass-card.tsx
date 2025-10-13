/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:46:43
 * @FilePath: \electron-go-app\frontend\src\components\ui\glass-card.tsx
 * @LastEditTime: 2025-10-09 22:46:48
 */
import { CSSProperties, ForwardedRef, ReactNode, forwardRef } from "react";
import { cn } from "../../lib/utils";

interface GlassCardProps {
    children: ReactNode;
    className?: string;
    style?: CSSProperties;
}

export const GlassCard = forwardRef(function GlassCard(
    { children, className, style }: GlassCardProps,
    ref: ForwardedRef<HTMLDivElement>
) {
    return (
        <div
            ref={ref}
            className={cn(
                "glass rounded-3xl border border-white/50 bg-white/70 p-6 shadow-elevation transition-colors dark:border-slate-800/70 dark:bg-slate-900/60 dark:shadow-none",
                className
            )}
            style={style}
        >
            {children}
        </div>
    );
});
