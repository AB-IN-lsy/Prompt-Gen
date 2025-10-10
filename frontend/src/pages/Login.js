import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:55:37
 * @FilePath: \electron-go-app\frontend\src\pages\Login.tsx
 * @LastEditTime: 2025-10-10 23:17:17
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { login, requestEmailVerification, confirmEmailVerification, } from "../lib/api";
import { ApiError } from "../lib/errors";
import { useAuth } from "../hooks/useAuth";
import { useVerificationToken } from "../hooks/useVerificationToken";
import { EMAIL_VERIFIED_EVENT_KEY } from "../lib/verification";
export default function LoginPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const [credentials, setCredentials] = useState({ email: "", password: "" });
    // 行内校验的错误信息容器，按字段分别存储提示文案。
    const [fieldErrors, setFieldErrors] = useState({});
    const [needsVerification, setNeedsVerification] = useState(false);
    const [shouldRetryLogin, setShouldRetryLogin] = useState(false);
    const [verificationFeedback, setVerificationFeedback] = useState(null);
    const authenticate = useAuth((state) => state.authenticate);
    const verificationToken = useVerificationToken();
    // 单字段校验逻辑：在输入过程与提交前复用，确保提示一致。
    const validateField = (field, value) => {
        if (field === "email") {
            if (!value.trim()) {
                return t("auth.validation.emailInvalid");
            }
            const emailPattern = /^[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}$/;
            if (!emailPattern.test(value)) {
                return t("auth.validation.emailInvalid");
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
    const validateAll = () => {
        return {
            email: validateField("email", credentials.email),
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
            navigate("/prompt-workbench", { replace: true });
        },
        onError: (error) => {
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
                }
                else {
                    message = error.message ?? message;
                }
            }
            toast.error(message);
        },
    });
    const loginMutate = mutation.mutate;
    const isLoginPending = mutation.isPending;
    const resendMutation = useMutation({
        mutationFn: async () => {
            if (!credentials.email.trim()) {
                throw new ApiError({ message: t("auth.validation.emailInvalid") ?? "" });
            }
            return requestEmailVerification(credentials.email.trim());
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
            }
            else {
                setVerificationFeedback({
                    tone: "info",
                    message: t("auth.verification.sentNeutral"),
                    remaining,
                });
                toast.success(t("auth.verification.sentNeutral"));
            }
        },
        onError: (error) => {
            let message = t("errors.generic");
            let retryAfter;
            let remaining;
            if (error instanceof ApiError) {
                if (error.code) {
                    const key = `errors.${error.code}`;
                    const translated = t(key, { defaultValue: "" });
                    message = translated && translated !== key ? translated : error.message ?? message;
                }
                else {
                    message = error.message ?? message;
                }
                const details = (error.details ?? {});
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
        }
        else {
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
                }
                catch (error) {
                    if (cancelled) {
                        return;
                    }
                    let message = t("errors.generic");
                    if (error instanceof ApiError) {
                        if (error.code) {
                            const key = `errors.${error.code}`;
                            const translated = t(key, { defaultValue: "" });
                            message = translated && translated !== key ? translated : error.message ?? message;
                        }
                        else {
                            message = error.message ?? message;
                        }
                    }
                    else if (error instanceof Error) {
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
        const handleStorage = (event) => {
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
    const handleSubmit = (event) => {
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
    const isSubmitDisabled = mutation.isPending ||
        Object.values(fieldErrors).some(Boolean) ||
        !credentials.email.trim() ||
        !credentials.password.trim();
    return (_jsxs(AuthLayout, { title: t("auth.login.title") ?? "", subtitle: t("auth.login.subtitle") ?? undefined, children: [_jsxs("form", { className: "space-y-5", onSubmit: handleSubmit, children: [_jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "email", children: t("auth.form.email") }), _jsx(Input, { id: "email", type: "email", autoComplete: "email", required: true, value: credentials.email, onChange: (event) => {
                                    const value = event.target.value;
                                    setCredentials((prev) => ({ ...prev, email: value }));
                                    setFieldErrors((prev) => ({ ...prev, email: validateField("email", value) }));
                                } }), fieldErrors.email ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.email }) : null] }), _jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "password", children: t("auth.form.password") }), _jsx(Input, { id: "password", type: "password", autoComplete: "current-password", required: true, value: credentials.password, onChange: (event) => {
                                    const value = event.target.value;
                                    setCredentials((prev) => ({ ...prev, password: value }));
                                    setFieldErrors((prev) => ({ ...prev, password: validateField("password", value) }));
                                } }), fieldErrors.password ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.password }) : null] }), backendError ? _jsx("p", { className: "text-sm text-red-500", children: backendError }) : null, needsVerification ? (_jsxs("div", { className: "space-y-3 rounded-xl border border-amber-200 bg-amber-50/80 p-4 transition-colors dark:border-amber-400/40 dark:bg-amber-500/10", children: [_jsxs("div", { className: "space-y-1", children: [_jsx("p", { className: "text-sm font-medium text-amber-900 dark:text-amber-200", children: t("auth.verification.blockTitle") }), _jsx("p", { className: "text-sm text-amber-800 dark:text-amber-100/90", children: t("auth.verification.blockMessage") })] }), _jsx("div", { className: "flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap", children: _jsx(Button, { type: "button", variant: "outline", className: "w-full sm:w-auto sm:flex-shrink-0", disabled: resendMutation.isPending, onClick: () => resendMutation.mutate(), children: resendMutation.isPending ? t("common.loading") : t("auth.verification.send") }) }), verificationFeedback ? (_jsxs("div", { className: `mt-3 rounded-lg border px-3 py-2 text-xs transition-colors ${verificationFeedback.tone === "success"
                                    ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-200"
                                    : verificationFeedback.tone === "error"
                                        ? "border-red-200 bg-red-50 text-red-700 dark:border-red-400/50 dark:bg-red-500/10 dark:text-red-200"
                                        : "border-slate-200 bg-white/70 text-slate-600 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-200"}`, children: [_jsx("p", { children: verificationFeedback.message }), typeof verificationFeedback.remaining === "number" ? (_jsx("p", { className: "mt-1", children: t("settings.verificationPending.remaining", {
                                            count: verificationFeedback.remaining,
                                        }) })) : null, typeof verificationFeedback.retryAfter === "number" ? (_jsx("p", { className: "mt-1", children: t("settings.verificationPending.rateLimit", {
                                            seconds: verificationFeedback.retryAfter,
                                        }) })) : null] })) : null, _jsx("p", { className: "text-xs text-amber-700 dark:text-amber-200", children: t("auth.verification.emailReminder") }), _jsx(Button, { type: "button", variant: "ghost", className: "w-full justify-start px-0 text-primary underline-offset-4 hover:underline", onClick: () => {
                                    const searchParams = new URLSearchParams();
                                    if (credentials.email.trim()) {
                                        searchParams.set("email", credentials.email.trim());
                                    }
                                    navigate(`/verify-email${searchParams.toString() ? `?${searchParams.toString()}` : ""}`);
                                }, children: t("auth.verification.goToPage") })] })) : null, _jsx(Button, { type: "submit", className: "w-full", disabled: isSubmitDisabled, children: mutation.isPending ? t("common.loading") : t("auth.login.cta") })] }), _jsxs("div", { className: "text-center text-sm text-slate-500 dark:text-slate-400", children: [t("auth.login.switch"), " ", _jsx(Link, { to: "/register", className: "font-medium text-primary hover:underline", children: t("auth.login.switchCta") })] })] }));
}
