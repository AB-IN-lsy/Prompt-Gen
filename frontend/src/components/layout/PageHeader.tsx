import { ReactNode } from "react";
import { cn } from "../../lib/utils";

interface PageHeaderProps {
    eyebrow?: string;
    title: string;
    description?: string;
    actions?: ReactNode;
    className?: string;
    headingClassName?: string;
}

export function PageHeader({
    eyebrow,
    title,
    description,
    actions,
    className,
    headingClassName
}: PageHeaderProps) {
    return (
        <div
            className={cn(
                "flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between",
                className
            )}
        >
            <div className={cn("space-y-1.5 sm:max-w-xl", headingClassName)}>
                {eyebrow ? (
                    <span className="text-[11px] font-semibold uppercase tracking-[0.3em] text-slate-400 dark:text-slate-500">
                        {eyebrow}
                    </span>
                ) : null}
                <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
                    {title}
                </h1>
                {description ? (
                    <p className="text-sm text-slate-500 dark:text-slate-400">{description}</p>
                ) : null}
            </div>
            {actions ? (
                <div className="flex flex-wrap items-center gap-2">{actions}</div>
            ) : null}
        </div>
    );
}
