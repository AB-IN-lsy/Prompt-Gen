/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 22:44:53
 * @FilePath: \electron-go-app\frontend\src\pages\EmailVerificationCallback.tsx
 * @LastEditTime: 2025-10-10 22:44:59
 */
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { confirmEmailVerification } from "../lib/api";
import { Button } from "../components/ui/button";
import { GlassCard } from "../components/ui/glass-card";
import { useAuth } from "../hooks/useAuth";
import { ApiError } from "../lib/errors";
import { EMAIL_VERIFIED_EVENT_KEY } from "../lib/verification";

export default function EmailVerificationCallbackPage() {
    const { t } = useTranslation();
    const [searchParams] = useSearchParams();
    const initialize = useAuth((state) => state.initialize);

    const token = useMemo(() => searchParams.get("token")?.trim() ?? "", [searchParams]);
    const redirect = useMemo(() => searchParams.get("redirect")?.trim() ?? "", [searchParams]);
    const [manualCloseHint, setManualCloseHint] = useState(false);

    const [status, setStatus] = useState<"pending" | "success" | "error">("pending");
    const [message, setMessage] = useState<string>("");

    useEffect(() => {
        if (!token) {
            setStatus("error");
            setMessage(t("auth.verification.callbackMissingToken", "验证链接缺少 token，无法完成验证"));
            return;
        }
        let cancelled = false;
        const run = async () => {
            try {
                await confirmEmailVerification(token);
                if (cancelled) {
                    return;
                }
                setStatus("success");
                setMessage(t("auth.verification.callbackSuccess", "邮箱验证成功，您可以关闭此页面"));
                if (typeof window !== "undefined" && window.localStorage) {
                    try {
                        window.localStorage.setItem(EMAIL_VERIFIED_EVENT_KEY, Date.now().toString());
                    } catch (storageError) {
                        console.warn("failed to broadcast verification event", storageError);
                    }
                }
                await initialize();
            } catch (error) {
                if (cancelled) {
                    return;
                }
                const apiError = error instanceof ApiError ? error : undefined;
                const translated = apiError?.code ? t(`errors.${apiError.code}`, { defaultValue: "" }) : "";
                const fallback = apiError?.message ?? t("errors.generic");
                setStatus("error");
                setMessage(translated && translated !== `errors.${apiError?.code}` ? translated : fallback);
            }
        };
        void run();
        return () => {
            cancelled = true;
        };
    }, [token, initialize, t]);

    const handleClose = () => {
        if (typeof window !== "undefined") {
            window.close();
            setTimeout(() => {
                setManualCloseHint(true);
            }, 200);
        }
    };

    return (
        <div className="min-h-screen bg-gradient-to-br from-[#F8F9FA] via-[#EEF2FF] to-[#E9EDFF] p-6 transition-colors dark:from-[#0f172a] dark:via-[#111827] dark:to-[#020617]">
            <div className="mx-auto flex w-full max-w-lg flex-col items-center justify-center gap-8">
                <GlassCard className="w-full space-y-6 text-center">
                    <div>
                        <h1 className="text-xl font-semibold text-slate-800 dark:text-slate-100">{t("auth.verification.callbackTitle", "邮箱验证")}</h1>
                        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                            {status === "pending"
                                ? t("auth.verification.callbackPending", "正在确认您的邮箱验证状态，请稍候……")
                                : message}
                        </p>
                    </div>
                    {status === "pending" ? (
                        <div className="flex flex-col items-center gap-3 text-sm text-slate-500 dark:text-slate-300">
                            <span className="inline-flex h-10 w-10 animate-spin items-center justify-center rounded-full border-2 border-primary/30 border-t-primary" />
                            <span>{t("common.loading")}</span>
                        </div>
                    ) : (
                        <div className="flex flex-col gap-3">
                            <Button onClick={handleClose} variant="secondary" className="w-full">
                                {t("auth.verification.callbackClose", "关闭此页面")}
                            </Button>
                            <Button
                                onClick={() => {
                                    if (typeof window !== "undefined") {
                                        window.location.href = redirect || "/login";
                                    }
                                }}
                                variant="outline"
                                className="w-full"
                            >
                                {status === "success"
                                    ? t("auth.verification.callbackReturn", "返回登录")
                                    : t("auth.verification.callbackRetry", "重新尝试")}
                            </Button>
                            {manualCloseHint ? (
                                <p className="text-xs text-slate-500 dark:text-slate-400">
                                    {t("auth.verification.callbackManualClose", "无法自动关闭窗口，请手动关闭当前标签页。")}
                                </p>
                            ) : null}
                        </div>
                    )}
                </GlassCard>
            </div>
        </div>
    );
}
