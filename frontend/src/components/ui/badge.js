import { jsx as _jsx } from "react/jsx-runtime";
import { cn } from "../../lib/utils";
export function Badge({ children, variant = "default", className, ...props }) {
    return (_jsx("span", { className: cn("inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium", variant === "default"
            ? "border-transparent bg-primary/10 text-primary"
            : "border-primary/30 text-primary", className), ...props, children: children }));
}
