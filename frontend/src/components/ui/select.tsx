import { forwardRef, type SelectHTMLAttributes } from "react";
import { cn } from "../../lib/utils";

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
    containerClassName?: string;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
    { className, containerClassName, disabled, multiple, children, ...rest },
    ref
) {
    const showChevron = multiple !== true && !rest.size;

    return (
        <div className={cn("relative inline-flex w-full", containerClassName)}>
            <select
                ref={ref}
                className={cn(
                    "w-full appearance-none rounded-2xl border border-slate-200/80 bg-white/95 px-4 py-2.5 pr-11 text-sm font-medium text-slate-700 shadow-[0_15px_35px_rgba(15,23,42,0.08)] transition-all duration-200 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200/80",
                    "hover:border-indigo-300 dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-100 dark:focus:border-indigo-300 dark:focus:ring-indigo-500/30",
                    disabled && "cursor-not-allowed opacity-70 shadow-none",
                    multiple && "pr-4",
                    className
                )}
                disabled={disabled}
                multiple={multiple}
                {...rest}
            >
                {children}
            </select>
            {showChevron ? (
                <div className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/70 p-1.5 text-slate-500 shadow-sm ring-1 ring-white/60 backdrop-blur dark:bg-slate-800/70 dark:text-slate-300 dark:ring-white/10">
                    <svg
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <polyline points="6 9 12 15 18 9" />
                    </svg>
                </div>
            ) : null}
        </div>
    );
});
