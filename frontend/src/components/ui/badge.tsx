/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:46:34
 * @FilePath: \electron-go-app\frontend\src\components\ui\badge.tsx
 * @LastEditTime: 2025-10-10 21:52:06
 */
import { HTMLAttributes } from "react";
import { cn } from "../../lib/utils";

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
    variant?: "default" | "outline";
}

export function Badge({ children, variant = "default", className, ...props }: BadgeProps) {
    return (
        <span
            className={cn(
                "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium transition-colors",
                variant === "default"
                    ? "border-transparent bg-primary/10 text-primary dark:bg-primary/20"
                    : "border-primary/30 text-primary dark:border-primary/50",
                className
            )}
            {...props}
        >
            {children}
        </span>
    );
}
