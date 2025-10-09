/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:46:43
 * @FilePath: \electron-go-app\frontend\src\components\ui\glass-card.tsx
 * @LastEditTime: 2025-10-09 22:46:48
 */
import { ReactNode } from "react";
import { cn } from "../../lib/utils";

interface GlassCardProps {
    children: ReactNode;
    className?: string;
}

export function GlassCard({ children, className }: GlassCardProps) {
    return (
        <div className={cn("glass rounded-3xl border border-white/50 bg-white/70 p-6 shadow-elevation", className)}>
            {children}
        </div>
    );
}
