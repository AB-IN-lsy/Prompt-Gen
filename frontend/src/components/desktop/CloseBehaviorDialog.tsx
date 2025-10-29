import { ChangeEvent } from "react";
import { Button } from "../ui/button";
import { GlassCard } from "../ui/glass-card";
import { cn } from "../../lib/utils";

interface CloseBehaviorDialogProps {
    open: boolean;
    title: string;
    description: string;
    rememberLabel: string;
    trayLabel: string;
    quitLabel: string;
    cancelLabel: string;
    trayHint: string;
    quitHint: string;
    remember: boolean;
    defaultBehavior: "tray" | "quit";
    onSelectTray: () => void;
    onSelectQuit: () => void;
    onRememberChange: (remember: boolean) => void;
    onCancel: () => void;
}

export function CloseBehaviorDialog({
    open,
    title,
    description,
    rememberLabel,
    trayLabel,
    quitLabel,
    cancelLabel,
    trayHint,
    quitHint,
    remember,
    defaultBehavior,
    onSelectTray,
    onSelectQuit,
    onRememberChange,
    onCancel
}: CloseBehaviorDialogProps): JSX.Element | null {
    if (!open) {
        return null;
    }

    const handleRememberChange = (event: ChangeEvent<HTMLInputElement>) => {
        onRememberChange(event.target.checked);
    };

    const trayActive = defaultBehavior === "tray";
    const quitActive = defaultBehavior === "quit";

    return (
        <div className="fixed inset-0 z-[130] flex items-center justify-center px-6 py-10">
            <div
                className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
                onClick={onCancel}
            />
            <GlassCard className="relative z-[131] w-full max-w-lg space-y-6 bg-white/92 text-slate-700 dark:bg-slate-900/85 dark:text-slate-200">
                <div className="space-y-2">
                    <h2 className="text-xl font-semibold text-slate-900 dark:text-white">
                        {title}
                    </h2>
                    <p className="text-sm text-slate-500 dark:text-slate-400">
                        {description}
                    </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                    <Button
                        type="button"
                        className={cn(
                            "h-auto flex-col items-start rounded-2xl bg-primary/15 px-4 py-4 text-left text-primary shadow-sm transition hover:bg-primary/20 hover:text-primary-foreground dark:bg-primary/20 dark:hover:bg-primary/30",
                            trayActive
                                ? "border border-primary/40 ring-2 ring-primary/30"
                                : "border border-transparent"
                        )}
                        onClick={onSelectTray}
                    >
                        <span className="text-base font-semibold">{trayLabel}</span>
                        <span className="mt-1 text-xs text-primary/90">
                            {trayHint}
                        </span>
                    </Button>
                    <Button
                        type="button"
                        className={cn(
                            "h-auto flex-col items-start rounded-2xl bg-rose-100/60 px-4 py-4 text-left text-rose-600 shadow-sm transition hover:bg-rose-100 dark:bg-rose-500/20 dark:text-rose-200 dark:hover:bg-rose-500/30",
                            quitActive
                                ? "border border-rose-400/70 ring-2 ring-rose-300/60"
                                : "border border-transparent"
                        )}
                        onClick={onSelectQuit}
                    >
                        <span className="text-base font-semibold">{quitLabel}</span>
                        <span className="mt-1 text-xs text-rose-500/80 dark:text-rose-200/80">
                            {quitHint}
                        </span>
                    </Button>
                </div>

                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <label className="inline-flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                        <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary"
                            checked={remember}
                            onChange={handleRememberChange}
                        />
                        <span>{rememberLabel}</span>
                    </label>
                    <Button
                        type="button"
                        variant="ghost"
                        className="self-end text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
                        onClick={onCancel}
                    >
                        {cancelLabel}
                    </Button>
                </div>
            </GlassCard>
        </div>
    );
}
