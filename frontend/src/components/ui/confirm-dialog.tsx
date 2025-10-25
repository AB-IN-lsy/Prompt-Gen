/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-26 18:12:00
 * @FilePath: \electron-go-app\frontend\src\components\ui\confirm-dialog.tsx
 * @LastEditTime: 2025-10-26 18:12:00
 */
import { ReactNode, useEffect, useRef } from "react";
import { Button } from "./button";
import { GlassCard } from "./glass-card";
import { cn } from "../../lib/utils";

interface ConfirmDialogProps {
    open: boolean;
    title: ReactNode;
    description?: ReactNode;
    confirmLabel: ReactNode;
    cancelLabel: ReactNode;
    loading?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
}

export function ConfirmDialog({
    open,
    title,
    description,
    confirmLabel,
    cancelLabel,
    loading = false,
    onConfirm,
    onCancel,
}: ConfirmDialogProps) {
    const confirmRef = useRef<HTMLButtonElement | null>(null);

    useEffect(() => {
        if (!open) {
            return;
        }
        const handleKeydown = (event: KeyboardEvent) => {
            if (event.key === "Escape" && !loading) {
                onCancel();
            }
        };
        window.addEventListener("keydown", handleKeydown);
        return () => window.removeEventListener("keydown", handleKeydown);
    }, [open, loading, onCancel]);

    useEffect(() => {
        if (open) {
            const timer = window.setTimeout(() => {
                confirmRef.current?.focus();
            }, 0);
            return () => window.clearTimeout(timer);
        }
        return undefined;
    }, [open]);

    if (!open) {
        return null;
    }

    return (
        <div className="fixed inset-0 z-[120] flex items-center justify-center px-6 py-10">
            <div
                className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
                onClick={() => {
                    if (!loading) {
                        onCancel();
                    }
                }}
            />
            <GlassCard className="relative z-[121] w-full max-w-md space-y-5 bg-white/90 text-slate-700 dark:bg-slate-900/80 dark:text-slate-200">
                <div className="space-y-2">
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
                        {title}
                    </h2>
                    {description ? (
                        <p className="text-sm text-slate-500 dark:text-slate-400">
                            {description}
                        </p>
                    ) : null}
                </div>
                <div className="flex items-center justify-end gap-3">
                    <Button
                        type="button"
                        variant="ghost"
                        className={cn("text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-white")}
                        onClick={onCancel}
                        disabled={loading}
                    >
                        {cancelLabel}
                    </Button>
                    <Button
                        type="button"
                        ref={confirmRef}
                        className="bg-rose-500 hover:bg-rose-600 focus-visible:ring-rose-400 dark:bg-rose-600 dark:hover:bg-rose-500"
                        onClick={onConfirm}
                        disabled={loading}
                    >
                        {loading ? (
                            <span className="inline-flex items-center gap-2">
                                <span className="h-3 w-3 animate-spin rounded-full border-2 border-white/60 border-t-white" />
                                {confirmLabel}
                            </span>
                        ) : (
                            confirmLabel
                        )}
                    </Button>
                </div>
            </GlassCard>
        </div>
    );
}
