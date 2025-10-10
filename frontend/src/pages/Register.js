import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:56:00
 * @FilePath: \electron-go-app\frontend\src\pages\Register.tsx
 * @LastEditTime: 2025-10-10 00:46:53
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { AvatarUploader } from "../components/account/AvatarUploader";
import { fetchCaptcha, register } from "../lib/api";
import { ApiError } from "../lib/errors";
import { useAuth } from "../hooks/useAuth";
import { toast } from "sonner";
// 验证码获取失败时会自动重试 3 次，每次间隔约 1.5 秒。
const MAX_AUTO_RETRIES = 3;
const RETRY_DELAY_MS = 1500;
export default function RegisterPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const authenticate = useAuth((state) => state.authenticate);
    const [form, setForm] = useState({
        username: "",
        email: "",
        password: "",
        captcha_id: "",
        captcha_code: "",
        avatar_url: undefined,
    });
    const [captcha, setCaptcha] = useState(null);
    const [captchaLoading, setCaptchaLoading] = useState(false);
    const [captchaStatus, setCaptchaStatus] = useState(null);
    const [retryInfo, setRetryInfo] = useState(null);
    // 行内校验的错误提示集合，对应每个表单字段。
    const [fieldErrors, setFieldErrors] = useState({});
    const retryCountRef = useRef(0);
    const retryTimerRef = useRef(null);
    const clearScheduledRetry = useCallback(() => {
        if (retryTimerRef.current) {
            clearTimeout(retryTimerRef.current);
            retryTimerRef.current = null;
        }
    }, []);
    const loadCaptcha = useCallback(async function loadCaptchaInner(options) {
        // 手动刷新会重置重试计数与状态提示，避免历史信息残留。
        if (!options?.auto) {
            retryCountRef.current = 0;
            clearScheduledRetry();
            setCaptchaStatus(null);
            setRetryInfo(null);
        }
        setCaptchaLoading(true);
        try {
            const data = await fetchCaptcha();
            setCaptcha({ id: data.captcha_id, image: data.image });
            setForm((prev) => ({ ...prev, captcha_id: data.captcha_id, captcha_code: "" }));
            // 新验证码生成后清空验证码输入框的错误提示。
            setFieldErrors((prev) => ({ ...prev, captcha_code: undefined }));
            setCaptchaStatus(null);
            setRetryInfo(null);
            clearScheduledRetry();
        }
        catch (error) {
            console.error("Failed to load captcha", error);
            setCaptcha(null);
            if (retryCountRef.current < MAX_AUTO_RETRIES) {
                const nextAttempt = retryCountRef.current + 1;
                retryCountRef.current = nextAttempt;
                setCaptchaStatus("retrying");
                setRetryInfo({ attempt: nextAttempt, total: MAX_AUTO_RETRIES });
                retryTimerRef.current = setTimeout(() => {
                    void loadCaptchaInner({ auto: true });
                }, RETRY_DELAY_MS);
            }
            else {
                setCaptchaStatus("failed");
                setRetryInfo(null);
            }
        }
        finally {
            setCaptchaLoading(false);
        }
    }, [clearScheduledRetry]);
    useEffect(() => {
        // 组件挂载时自动拉取一次验证码，并在卸载时清理定时器。
        void loadCaptcha();
        return () => {
            clearScheduledRetry();
        };
    }, [clearScheduledRetry, loadCaptcha]);
    // 负责提交注册请求，成功后调用 authenticate 写入令牌并导航。
    const mutation = useMutation({
        mutationFn: async () => {
            if (!captcha?.id || !form.captcha_code?.trim()) {
                throw new ApiError({ message: t("auth.validation.captchaRequired") ?? t("errors.generic") });
            }
            const auth = await register({
                username: form.username,
                email: form.email,
                password: form.password,
                captcha_id: captcha.id,
                captcha_code: form.captcha_code,
                avatar_url: form.avatar_url,
            });
            await authenticate(auth.tokens);
        },
        onSuccess: () => {
            toast.success(t("auth.register.success"));
            toast.info(t("auth.verification.sent", "验证邮件已发送，请前往邮箱完成确认。"));
            navigate("/prompt-workbench", { replace: true });
        },
        onError: (error) => {
            if (error instanceof ApiError && ["CAPTCHA_INVALID", "CAPTCHA_EXPIRED"].includes(error.code ?? "")) {
                void loadCaptcha();
            }
            if (error instanceof ApiError && error.code === "CONFLICT") {
                const details = error.details ?? {};
                const conflictFields = new Set();
                if (typeof details.field === "string") {
                    conflictFields.add(details.field);
                }
                if (Array.isArray(details.fields)) {
                    for (const field of details.fields) {
                        if (typeof field === "string") {
                            conflictFields.add(field);
                        }
                    }
                }
                setFieldErrors((prev) => ({
                    ...prev,
                    email: conflictFields.has("email") ? t("auth.validation.emailTaken") : prev.email,
                    username: conflictFields.has("username") ? t("auth.validation.usernameTaken") : prev.username,
                }));
            }
            // 统一的失败提示：优先使用错误码对应的翻译。
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
            toast.error(message);
        },
    });
    // 按需校验单个字段，返回对应的错误提示文案。
    const validateField = (field, value) => {
        if (field === "username") {
            if (!value.trim()) {
                return t("auth.validation.usernameRequired");
            }
            if (value.trim().length < 2) {
                return t("auth.validation.usernameTooShort");
            }
        }
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
        if (field === "captcha_code") {
            if (!value.trim()) {
                return t("auth.validation.captchaRequired");
            }
        }
        return undefined;
    };
    // 聚合所有字段的校验结果，提交时使用。
    const validateAllFields = () => {
        const nextErrors = {
            username: validateField("username", form.username),
            email: validateField("email", form.email),
            password: validateField("password", form.password),
            captcha_code: validateField("captcha_code", form.captcha_code ?? ""),
        };
        setFieldErrors(nextErrors);
        return nextErrors;
    };
    const handleSubmit = (event) => {
        event.preventDefault();
        if (mutation.isPending || captchaLoading) {
            return;
        }
        const nextErrors = validateAllFields();
        if (Object.values(nextErrors).some(Boolean)) {
            return;
        }
        mutation.mutate();
    };
    const error = mutation.error instanceof ApiError ? mutation.error : undefined;
    const errorMessage = useMemo(() => {
        if (!error) {
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
    const captchaSrc = useMemo(() => {
        if (!captcha?.image) {
            return null;
        }
        return captcha.image.startsWith("data:") ? captcha.image : `data:image/png;base64,${captcha.image}`;
    }, [captcha]);
    // 满足以下任一条件时禁用提交按钮，避免重复提交或数据不完整。
    const isSubmitDisabled = mutation.isPending ||
        captchaLoading ||
        !form.username.trim() ||
        !form.email.trim() ||
        !form.password.trim() ||
        !form.captcha_code?.trim() ||
        !captcha ||
        Object.values(fieldErrors).some(Boolean);
    return (_jsxs(AuthLayout, { title: t("auth.register.title") ?? "", subtitle: t("auth.register.subtitle") ?? undefined, children: [_jsxs("form", { className: "space-y-5", onSubmit: handleSubmit, children: [_jsx(AvatarUploader, { value: form.avatar_url ?? "", onChange: (value) => {
                            setForm((prev) => ({ ...prev, avatar_url: value || undefined }));
                            setFieldErrors((prev) => ({ ...prev, avatar_url: undefined }));
                        } }), fieldErrors.avatar_url ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.avatar_url }) : null, _jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "username", children: t("auth.form.username") }), _jsx(Input, { id: "username", autoComplete: "username", required: true, value: form.username, onChange: (event) => {
                                    const value = event.target.value;
                                    setForm((prev) => ({ ...prev, username: value }));
                                    setFieldErrors((prev) => ({ ...prev, username: validateField("username", value) }));
                                } }), fieldErrors.username ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.username }) : null] }), _jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "email", children: t("auth.form.email") }), _jsx(Input, { id: "email", type: "email", autoComplete: "email", required: true, value: form.email, onChange: (event) => {
                                    const value = event.target.value;
                                    setForm((prev) => ({ ...prev, email: value }));
                                    setFieldErrors((prev) => ({ ...prev, email: validateField("email", value) }));
                                } }), fieldErrors.email ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.email }) : null] }), _jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "password", children: t("auth.form.password") }), _jsx(Input, { id: "password", type: "password", autoComplete: "new-password", required: true, value: form.password, onChange: (event) => {
                                    const value = event.target.value;
                                    setForm((prev) => ({ ...prev, password: value }));
                                    setFieldErrors((prev) => ({ ...prev, password: validateField("password", value) }));
                                } }), fieldErrors.password ? _jsx("p", { className: "text-xs text-red-500", children: fieldErrors.password }) : null] }), _jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "captcha", children: t("auth.form.captcha") }), _jsxs("div", { className: "flex flex-wrap items-center gap-3", children: [captchaSrc ? (_jsx("img", { src: captchaSrc, alt: t("auth.captcha.alt") ?? "Captcha", className: "h-14 w-32 rounded-xl border border-white/60 bg-white/80 object-cover shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70" })) : (_jsx("div", { className: "flex h-14 w-32 items-center justify-center rounded-xl border border-dashed border-slate-200 bg-white/50 text-xs text-slate-400 transition-colors dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-500", children: t("auth.captcha.unavailable") })), _jsx(Button, { type: "button", size: "sm", variant: "outline", onClick: () => {
                                            void loadCaptcha();
                                        }, disabled: captchaLoading, children: captchaLoading ? t("common.loading") : t("auth.captcha.refresh") })] }), captchaStatus ? (_jsxs("div", { className: "space-y-1", "aria-live": "polite", role: "status", children: [_jsx("p", { className: "text-xs text-amber-600", children: captchaStatus === "retrying" && retryInfo
                                            ? t("auth.captcha.retrying", {
                                                attempt: retryInfo.attempt,
                                                total: retryInfo.total,
                                            })
                                            : t("auth.captcha.failed") }), captchaStatus === "failed" ? (_jsx("p", { className: "text-xs text-muted-foreground", children: t("auth.captcha.manualHint") })) : null] })) : null, _jsx(Input, { id: "captcha", autoComplete: "off", required: true, value: form.captcha_code ?? "", onChange: (event) => {
                                    const value = event.target.value;
                                    setForm((prev) => ({ ...prev, captcha_code: value }));
                                    setFieldErrors((prev) => ({ ...prev, captcha_code: validateField("captcha_code", value) }));
                                }, placeholder: t("auth.captcha.placeholder") ?? "" }), fieldErrors.captcha_code ? (_jsx("p", { className: "text-xs text-red-500", children: fieldErrors.captcha_code })) : null] }), errorMessage ? _jsx("p", { className: "text-sm text-red-500", children: errorMessage }) : null, _jsx(Button, { type: "submit", className: "w-full", disabled: isSubmitDisabled, children: mutation.isPending ? t("common.loading") : t("auth.register.cta") })] }), _jsxs("div", { className: "text-center text-sm text-slate-500 dark:text-slate-400", children: [t("auth.register.switch"), " ", " ", _jsx(Link, { to: "/login", className: "font-medium text-primary hover:underline", children: t("auth.register.switchCta") })] })] }));
}
