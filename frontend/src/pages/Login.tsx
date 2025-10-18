/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:55:37
 * @FilePath: \electron-go-app\frontend\src\pages\Login.tsx
 * @LastEditTime: 2025-10-10 23:17:17
 */
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { AuthLayout } from "../components/layout/AuthLayout";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import {
    login,
    requestEmailVerification,
    confirmEmailVerification,
    type LoginRequest,
    type EmailVerificationRequestResult,
} from "../lib/api";
import { ApiError } from "../lib/errors";
import { useAuth } from "../hooks/useAuth";
import { useVerificationToken } from "../hooks/useVerificationToken";
import { EMAIL_VERIFIED_EVENT_KEY } from "../lib/verification";
import { normaliseError } from "../lib/http";

const IDENTIFIER_STORAGE_KEY = "promptgen:last-identifier";

type VerificationFeedback = {
    tone: "info" | "success" | "error";
    message: string;
    remaining?: number;
    retryAfter?: number;
};

export default function LoginPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const [credentials, setCredentials] = useState<LoginRequest>({ identifier: "", password: "" });
    // 行内校验的错误信息容器，按字段分别存储提示文案。
    const [fieldErrors, setFieldErrors] = useState<{ identifier?: string; password?: string }>({});
    const [needsVerification, setNeedsVerification] = useState(false);
    const [shouldRetryLogin, setShouldRetryLogin] = useState(false);
    const [verificationFeedback, setVerificationFeedback] = useState<VerificationFeedback | null>(null);
    const authenticate = useAuth((state) => state.authenticate);
    const enterOfflineMode = useAuth((state) => state.enterOffline);
    const verificationToken = useVerificationToken();
    const [rememberIdentifier, setRememberIdentifier] = useState(false);
    const [offlineLoading, setOfflineLoading] = useState(false);

    // 单字段校验逻辑：在输入过程与提交前复用，确保提示一致。
    const validateField = (field: keyof LoginRequest, value: string): string | undefined => {
        if (field === "identifier") {
            const trimmed = value.trim();
            if (!trimmed) {
                return t("auth.validation.identifierRequired");
            }
            const emailPattern = /^[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}$/;
            if (trimmed.includes("@") && !emailPattern.test(trimmed)) {
                return t("auth.validation.identifierInvalid");
            }
            if (!trimmed.includes("@") && trimmed.length < 2) {
                return t("auth.validation.identifierInvalid");
            }
        }
        if (field === "password") {
            if (!value.trim()) {
                return t("auth.validation.passwordRequired");
            }
            if (value.length < 8) {
                return t("auth.validation.passwordTooShort");
            }
        }
        return undefined;
    };

    // 提交前的整体校验，汇总所有字段错误。
    const validateAll = (): { identifier?: string; password?: string } => {
        return {
            identifier: validateField("identifier", credentials.identifier),
            password: validateField("password", credentials.password),
        };
    };

    const mutation = useMutation({
        mutationFn: async () => {
            const auth = await login(credentials);
            await authenticate(auth.tokens);
        },
        onSuccess: () => {
            // 登录成功后的友好提示与跳转。
            toast.success(t("auth.login.success"));
            setNeedsVerification(false);
            setShouldRetryLogin(false);
            setVerificationFeedback(null);
            if (typeof window !== "undefined") {
                const trimmed = credentials.identifier.trim();
                if (rememberIdentifier && trimmed) {
                    window.localStorage.setItem(IDENTIFIER_STORAGE_KEY, trimmed);
                } else {
                    window.localStorage.removeItem(IDENTIFIER_STORAGE_KEY);
                }
            }
            navigate("/prompt-workbench", { replace: true });
        },
        onError: (error: unknown) => {
            // 后端错误优先显示字典翻译，兜底使用通用提示。
            let message = t("errors.generic");
            if (error instanceof ApiError) {
                if (error.code === "EMAIL_NOT_VERIFIED" || error.status === 403) {
                    setNeedsVerification(true);
                    setShouldRetryLogin(true);
                    toast.error(t("auth.verification.required"));
                    return;
                }
                if (error.code) {
                    const key = `errors.${error.code}`;
                    const translated = t(key, { defaultValue: "" });
                    message = translated && translated !== key ? translated : error.message ?? message;
                } else {
                    message = error.message ?? message;
                }
            }
            toast.error(message);
        },
    });

    const loginMutate = mutation.mutate;
    const isLoginPending = mutation.isPending;

    const resendMutation = useMutation({
        mutationFn: async (): Promise<EmailVerificationRequestResult> => {
            const trimmed = credentials.identifier.trim();
            if (!trimmed) {
                throw new ApiError({ message: t("auth.validation.identifierRequired") ?? "" });
            }
            if (!trimmed.includes("@")) {
                throw new ApiError({ message: t("auth.verification.emailOnly") ?? "" });
            }
            return requestEmailVerification(trimmed);
        },
        onSuccess: (result) => {
            const remaining = typeof result.remainingAttempts === "number" ? result.remainingAttempts : undefined;
            if (result.issued) {
                setVerificationFeedback({
                    tone: "success",
                    message: result.token
                        ? t("auth.verification.sentWithToken", { token: result.token })
                        : t("auth.verification.sent"),
                    remaining,
                });
                toast.success(t("auth.verification.sent"));
            } else {
                setVerificationFeedback({
                    tone: "info",
                    message: t("auth.verification.sentNeutral"),
                    remaining,
                });
                toast.success(t("auth.verification.sentNeutral"));
            }
        },
        onError: (error: unknown) => {
            let message = t("errors.generic");
            let retryAfter: number | undefined;
            let remaining: number | undefined;
            if (error instanceof ApiError) {
                if (error.code) {
                    const key = `errors.${error.code}`;
                    const translated = t(key, { defaultValue: "" });
                    message = translated && translated !== key ? translated : error.message ?? message;
                } else {
                    message = error.message ?? message;
                }
                const details = (error.details ?? {}) as { remaining_attempts?: number; retry_after_seconds?: number };
                if (typeof details.remaining_attempts === "number") {
                    remaining = details.remaining_attempts;
                }
                if (typeof details.retry_after_seconds === "number") {
                    retryAfter = details.retry_after_seconds;
                }
            }
            setVerificationFeedback({
                tone: "error",
                message,
                remaining,
                retryAfter,
            });
            toast.error(message);
        },
    });

    const handleVerifiedBroadcast = useCallback(() => {
        toast.success(t("auth.verification.autoSuccess"));
        setNeedsVerification(false);
        setVerificationFeedback(null);
        if (shouldRetryLogin && !isLoginPending) {
            loginMutate(undefined, {
                onSettled: () => {
                    setShouldRetryLogin(false);
                },
            });
        } else {
            setShouldRetryLogin(false);
        }
    }, [t, shouldRetryLogin, isLoginPending, loginMutate]);

    useEffect(() => {
        if (verificationToken) {
            let cancelled = false;
            const confirm = async () => {
                try {
                    await confirmEmailVerification(verificationToken);
                    if (cancelled) {
                        return;
                    }
                    handleVerifiedBroadcast();
                } catch (error) {
                    if (cancelled) {
                        return;
                    }
                    let message = t("errors.generic");
                    if (error instanceof ApiError) {
                        if (error.code) {
                            const key = `errors.${error.code}`;
                            const translated = t(key, { defaultValue: "" });
                            message = translated && translated !== key ? translated : error.message ?? message;
                        } else {
                            message = error.message ?? message;
                        }
                    } else if (error instanceof Error) {
                        message = error.message;
                    }
                    toast.error(message);
                }
            };
            void confirm();
            return () => {
                cancelled = true;
            };
        }
        return undefined;
    }, [verificationToken, handleVerifiedBroadcast, t]);

    useEffect(() => {
        const broadcast = () => {
            const payload = window.localStorage.getItem(EMAIL_VERIFIED_EVENT_KEY);
            if (payload) {
                window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
                handleVerifiedBroadcast();
            }
        };

        const handleStorage = (event: StorageEvent) => {
            if (event.key === EMAIL_VERIFIED_EVENT_KEY && event.newValue) {
                handleVerifiedBroadcast();
                if (typeof window !== "undefined") {
                    window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
                }
            }
        };

        if (typeof window === "undefined") {
            return undefined;
        }

        broadcast();
        window.addEventListener("storage", handleStorage);
        return () => {
            window.removeEventListener("storage", handleStorage);
        };
    }, [handleVerifiedBroadcast]);

    const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (mutation.isPending) {
            return;
        }
        const nextErrors = validateAll();
        setFieldErrors(nextErrors);
        if (Object.values(nextErrors).some(Boolean)) {
            return;
        }
        mutation.mutate();
    };

    const error = mutation.error instanceof ApiError ? mutation.error : undefined;
    const backendError = useMemo(() => {
        if (!error) {
            return undefined;
        }
        if (error.code === "EMAIL_NOT_VERIFIED") {
            return undefined;
        }
        if (error.code) {
            const key = `errors.${error.code}`;
            const translated = t(key, { defaultValue: "" });
            if (translated && translated !== key) {
                return translated;
            }
        }
        return error.message ?? t("errors.generic");
    }, [error, t]);
    // 禁用提交按钮的条件：请求中、存在校验错误或必填项为空。
    const isSubmitDisabled =
        mutation.isPending ||
        Object.values(fieldErrors).some(Boolean) ||
        !credentials.identifier.trim() ||
        !credentials.password.trim();

    const handleOfflineMode = useCallback(() => {
        if (offlineLoading) {
            return;
        }
        setOfflineLoading(true);
        void enterOfflineMode()
            .then(() => {
                toast.success(t("auth.login.success"));
                navigate("/prompt-workbench", { replace: true });
            })
            .catch((error: unknown) => {
                const normalised = normaliseError(error);
                const message = normalised.message ?? t("auth.offline.failed");
                toast.error(message);
            })
            .finally(() => setOfflineLoading(false));
    }, [enterOfflineMode, navigate, offlineLoading, t]);

    return (
        <AuthLayout title={t("auth.login.title") ?? ""} subtitle={t("auth.login.subtitle") ?? undefined}>
            <form className="space-y-5" onSubmit={handleSubmit}>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700 dark:text-slate-200" htmlFor="identifier">
                        {t("auth.form.identifier")}
                    </label>
                    <Input
                        id="identifier"
                        type="text"
                        autoComplete="username"
                        required
                        value={credentials.identifier}
                        onChange={(event) => {
                            const value = event.target.value;
                            setCredentials((prev) => ({ ...prev, identifier: value }));
                            setFieldErrors((prev) => ({ ...prev, identifier: validateField("identifier", value) }));
                        }}
                    />
                    {fieldErrors.identifier ? <p className="text-xs text-red-500">{fieldErrors.identifier}</p> : null}
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700 dark:text-slate-200" htmlFor="password">
                        {t("auth.form.password")}
                    </label>
                    <Input
                        id="password"
                        type="password"
                        autoComplete="current-password"
                        required
                        value={credentials.password}
                        onChange={(event) => {
                            const value = event.target.value;
                            setCredentials((prev) => ({ ...prev, password: value }));
                            setFieldErrors((prev) => ({ ...prev, password: validateField("password", value) }));
                        }}
                    />
                    {fieldErrors.password ? <p className="text-xs text-red-500">{fieldErrors.password}</p> : null}
                </div>
                <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
                    <label className="flex items-center gap-2">
                        <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary"
                            checked={rememberIdentifier}
                            onChange={(event) => setRememberIdentifier(event.target.checked)}
                        />
                        <span>{t("auth.login.remember")}</span>
                    </label>
                </div>
                {backendError ? <p className="text-sm text-red-500">{backendError}</p> : null}
                {needsVerification ? (
                    <div className="space-y-3 rounded-xl border border-amber-200 bg-amber-50/80 p-4 transition-colors dark:border-amber-400/40 dark:bg-amber-500/10">
                        <div className="space-y-1">
                            <p className="text-sm font-medium text-amber-900 dark:text-amber-200">{t("auth.verification.blockTitle")}</p>
                            <p className="text-sm text-amber-800 dark:text-amber-100/90">{t("auth.verification.blockMessage")}</p>
                        </div>
                        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
                            <Button
                                type="button"
                                variant="outline"
                                className="w-full sm:w-auto sm:flex-shrink-0"
                                disabled={resendMutation.isPending}
                                onClick={() => resendMutation.mutate()}
                            >
                                {resendMutation.isPending ? t("common.loading") : t("auth.verification.send")}
                            </Button>
                        </div>
                        {verificationFeedback ? (
                            <div
                                className={`mt-3 rounded-lg border px-3 py-2 text-xs transition-colors ${verificationFeedback.tone === "success"
                                        ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-200"
                                        : verificationFeedback.tone === "error"
                                            ? "border-red-200 bg-red-50 text-red-700 dark:border-red-400/50 dark:bg-red-500/10 dark:text-red-200"
                                            : "border-slate-200 bg-white/70 text-slate-600 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-200"
                                    }`}
                            >
                                <p>{verificationFeedback.message}</p>
                                {typeof verificationFeedback.remaining === "number" ? (
                                    <p className="mt-1">
                                        {t("settings.verificationPending.remaining", {
                                            count: verificationFeedback.remaining,
                                        })}
                                    </p>
                                ) : null}
                                {typeof verificationFeedback.retryAfter === "number" ? (
                                    <p className="mt-1">
                                        {t("settings.verificationPending.rateLimit", {
                                            seconds: verificationFeedback.retryAfter,
                                        })}
                                    </p>
                                ) : null}
                            </div>
                        ) : null}
                        <p className="text-xs text-amber-700 dark:text-amber-200">{t("auth.verification.emailReminder")}</p>
                        <Button
                            type="button"
                            variant="ghost"
                            className="w-full justify-start px-0 text-primary underline-offset-4 hover:underline"
                            onClick={() => {
                                const searchParams = new URLSearchParams();
                                const trimmed = credentials.identifier.trim();
                                if (trimmed && trimmed.includes("@")) {
                                    searchParams.set("email", trimmed);
                                }
                                navigate(`/verify-email${searchParams.toString() ? `?${searchParams.toString()}` : ""}`);
                            }}
                        >
                            {t("auth.verification.goToPage")}
                        </Button>
                    </div>
                ) : null}
                <Button type="submit" className="w-full" disabled={isSubmitDisabled}>
                    {mutation.isPending ? t("common.loading") : t("auth.login.cta")}
                </Button>
            </form>
            <div className="mt-6 rounded-xl border border-slate-200 bg-white/70 p-4 text-left shadow-sm transition-colors dark:border-slate-700 dark:bg-slate-900/70">
                <div className="space-y-1">
                    <p className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                        {t("auth.offline.title")}
                    </p>
                    <p className="text-xs text-slate-500 dark:text-slate-400">
                        {t("auth.offline.description")}
                    </p>
                </div>
                <Button
                    type="button"
                    variant="outline"
                    className="mt-4 w-full"
                    disabled={offlineLoading}
                    onClick={handleOfflineMode}
                >
                    {offlineLoading ? t("common.loading") : t("auth.offline.enter")}
                </Button>
            </div>
            <div className="text-center text-sm text-slate-500 dark:text-slate-400">
                {t("auth.login.switch")}{" "}
                <Link to="/register" className="font-medium text-primary hover:underline">
                    {t("auth.login.switchCta")}
                </Link>
            </div>
        </AuthLayout>
    );
}
