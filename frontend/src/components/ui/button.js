import { jsx as _jsx } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:44:03
 * @FilePath: \electron-go-app\frontend\src\components\ui\button.tsx
 * @LastEditTime: 2025-10-09 22:44:07
 */
import { cva } from "class-variance-authority";
import { forwardRef } from "react";
import { cn } from "../../lib/utils";
const buttonVariants = cva("inline-flex items-center justify-center rounded-xl text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50", {
    variants: {
        variant: {
            default: "bg-primary text-primary-foreground shadow-glow hover:shadow-md",
            secondary: "bg-gradient-to-r from-primary to-secondary text-white shadow-glow hover:shadow-lg",
            ghost: "bg-transparent text-slate-600 hover:bg-white/60",
            outline: "border border-primary/30 text-primary hover:bg-primary/5"
        },
        size: {
            default: "h-10 px-4",
            sm: "h-9 px-3",
            lg: "h-11 px-6"
        }
    },
    defaultVariants: {
        variant: "default",
        size: "default"
    }
});
export const Button = forwardRef(({ className, variant, size, ...props }, ref) => (_jsx("button", { ref: ref, className: cn(buttonVariants({ variant, size }), className), ...props })));
Button.displayName = "Button";
