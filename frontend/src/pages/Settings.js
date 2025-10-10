import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:33:13
 * @FilePath: \electron-go-app\frontend\src\pages\Settings.tsx
 * @LastEditTime: 2025-10-09 23:33:18
 */
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { GlassCard } from "../components/ui/glass-card";
import { useAppSettings } from "../hooks/useAppSettings";
import { LANGUAGE_OPTIONS } from "../i18n";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { AvatarUploader } from "../components/account/AvatarUploader";
import { useAuth } from "../hooks/useAuth";
import { updateCurrentUser, requestEmailVerification } from "../lib/api";
import { ApiError, isApiError } from "../lib/errors";
import { EMAIL_VERIFIED_EVENT_KEY } from "../lib/verification";
export default function SettingsPage() {
    const { t } = useTranslation();
    const { language, setLanguage, theme, resolvedTheme, setTheme } = useAppSettings();
    const profile = useAuth((state) => state.profile);
    const setProfile = useAuth((state) => state.setProfile);
    const initializeAuth = useAuth((state) => state.initialize);
    const isEmailVerified = Boolean(profile?.user.email_verified_at);
    const [profileForm, setProfileForm] = useState({
        username: profile?.user.username ?? "",
        email: profile?.user.email ?? "",
        avatar_url: profile?.user.avatar_url ?? "",
        preferred_model: profile?.settings.preferred_model ?? "",
        sync_enabled: profile?.settings.sync_enabled ?? false,
    });
    const [profileErrors, setProfileErrors] = useState(() => ({
        username: undefined,
        email: undefined,
    }));
    const [verificationTargetEmail, setVerificationTargetEmail] = useState(profile?.user.email ?? "");
    const [verificationFeedback, setVerificationFeedback] = useState(null);
    useEffect(() => {
        setProfileForm({
            username: profile?.user.username ?? "",
            email: profile?.user.email ?? "",
            avatar_url: profile?.user.avatar_url ?? "",
            preferred_model: profile?.settings.preferred_model ?? "",
            sync_enabled: profile?.settings.sync_enabled ?? false,
        });
        setProfileErrors({ username: undefined, email: undefined });
        setVerificationTargetEmail(profile?.user.email ?? "");
        setVerificationFeedback(null);
    }, [profile]);
    useEffect(() => {
        if (typeof window === "undefined") {
            return undefined;
        }
        const syncProfile = () => {
            toast.success(t("settings.emailStatus.verifiedToast", "邮箱验证已完成"));
            void initializeAuth();
        };
        const checkLocalFlag = () => {
            const flag = window.localStorage.getItem(EMAIL_VERIFIED_EVENT_KEY);
            if (flag) {
                window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
                syncProfile();
                return true;
            }
            return false;
        };
        checkLocalFlag();
        const handleStorage = (event) => {
            if (event.key === EMAIL_VERIFIED_EVENT_KEY && event.newValue) {
                window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
                syncProfile();
            }
        };
        window.addEventListener("storage", handleStorage);
        return () => {
            window.removeEventListener("storage", handleStorage);
        };
    }, [initializeAuth, t]);
    const mutation = useMutation({
        mutationFn: async (payload) => updateCurrentUser(payload),
        onSuccess: (data) => {
            setProfile(data);
            toast.success(t("settings.profileSaveSuccess"));
        },
        onError: (error) => {
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
                setProfileErrors((prev) => ({
                    ...prev,
                    email: conflictFields.has("email") ? t("settings.errors.emailTaken") : prev.email,
                    username: conflictFields.has("username") ? t("settings.errors.usernameTaken") : prev.username,
                }));
            }
            const message = error instanceof ApiError ? error.message ?? t("errors.generic") : t("errors.generic");
            toast.error(message);
        },
    });
    const handleLanguageChange = (event) => {
        const value = event.target.value;
        if (LANGUAGE_OPTIONS.some((option) => option.code === value)) {
            setLanguage(value);
        }
    };
    const validateProfile = () => {
        const username = profileForm.username.trim();
        const email = profileForm.email.trim();
        const nextErrors = {
            username: username ? undefined : t("settings.errors.usernameRequired"),
            email: /^[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}$/.test(email) ? undefined : t("settings.errors.emailInvalid"),
        };
        setProfileErrors(nextErrors);
        return nextErrors;
    };
    const handleProfileSubmit = (event) => {
        event.preventDefault();
        if (mutation.isPending) {
            return;
        }
        const errors = validateProfile();
        if (Object.values(errors).some(Boolean)) {
            return;
        }
        const payload = {
            username: profileForm.username.trim(),
            email: profileForm.email.trim(),
            preferred_model: profileForm.preferred_model.trim() || undefined,
            sync_enabled: profileForm.sync_enabled,
        };
        const initialAvatar = profile?.user.avatar_url ?? "";
        if (profileForm.avatar_url !== initialAvatar) {
            // 置空头像时需要显式发送空字符串，后端会将其写入数据库实现“移除头像”效果。
            const avatarValue = profileForm.avatar_url?.trim?.() ?? "";
            payload.avatar_url = avatarValue;
        }
        mutation.mutate(payload);
    };
    const syncLabel = useMemo(() => (profileForm.sync_enabled ? t("settings.syncEnabledOn") : t("settings.syncEnabledOff")), [profileForm.sync_enabled, t]);
    const verificationMutation = useMutation({
        mutationFn: async () => {
            const email = profileForm.email.trim();
            if (!email) {
                throw new ApiError({ code: "BAD_REQUEST", message: t("settings.verificationPending.emailMissing") });
            }
            return requestEmailVerification(email);
        },
        onSuccess: (result) => {
            const email = profileForm.email.trim();
            setVerificationTargetEmail(email);
            const remaining = typeof result.remainingAttempts === "number" ? result.remainingAttempts : undefined;
            if (result.issued) {
                setVerificationFeedback({
                    tone: "success",
                    message: result.token
                        ? t("settings.verificationPending.sentDev", { token: result.token })
                        : t("settings.verificationPending.sent"),
                    remaining,
                });
            }
            else {
                setVerificationFeedback({
                    tone: "info",
                    message: t("settings.verificationPending.sentNeutral"),
                    remaining,
                });
            }
        },
        onError: (error) => {
            let message = t("errors.generic");
            let retryAfter;
            let remaining;
            if (isApiError(error)) {
                message = error.message ?? t("errors.generic");
                const details = (error.details ?? {});
                if (typeof details.remaining_attempts === "number") {
                    remaining = details.remaining_attempts;
                }
                if (typeof details.retry_after_seconds === "number") {
                    retryAfter = details.retry_after_seconds;
                }
            }
            else if (error instanceof Error) {
                message = error.message;
            }
            setVerificationFeedback({
                tone: "error",
                message,
                remaining,
                retryAfter,
            });
        },
    });
    const themeOptions = useMemo(() => [
        {
            value: "system",
            label: t("settings.themeCard.options.system"),
            description: t("settings.themeCard.optionDescriptions.system", "跟随操作系统的外观设定"),
        },
        {
            value: "light",
            label: t("settings.themeCard.options.light"),
            description: t("settings.themeCard.optionDescriptions.light", "使用浅色背景"),
        },
        {
            value: "dark",
            label: t("settings.themeCard.options.dark"),
            description: t("settings.themeCard.optionDescriptions.dark", "使用深色背景"),
        },
    ], [t]);
    return (_jsxs("div", { className: "mx-auto flex w-full max-w-4xl flex-col gap-6 text-slate-700 transition-colors dark:text-slate-200", children: [_jsxs("header", { className: "flex flex-col gap-1", children: [_jsx("h1", { className: "text-2xl font-semibold text-slate-800 dark:text-slate-100", children: t("settings.title") }), _jsx("p", { className: "text-sm text-slate-500 dark:text-slate-400", children: t("settings.subtitle") })] }), isEmailVerified ? null : (_jsxs("div", { className: "rounded-xl border border-amber-200 bg-amber-50/80 p-4 text-sm leading-relaxed text-amber-800 transition-colors dark:border-amber-400/50 dark:bg-amber-500/10 dark:text-amber-100", children: [_jsx("p", { children: t("settings.verificationPending.notice", "邮箱尚未完成验证，请尽快前往验证以解锁全部功能。") }), verificationTargetEmail ? (_jsx("p", { className: "mt-2 text-xs text-amber-700 dark:text-amber-200", children: t("settings.verificationPending.emailLabel", { email: verificationTargetEmail }) })) : null, _jsx("div", { className: "mt-3 flex flex-wrap gap-2", children: _jsx(Button, { type: "button", disabled: verificationMutation.isPending, onClick: () => verificationMutation.mutate(), children: verificationMutation.isPending
                                ? t("common.loading")
                                : t("settings.verificationPending.send", "发送验证邮件") }) }), verificationFeedback ? (_jsxs("div", { className: `mt-3 rounded-lg border px-3 py-2 text-xs transition-colors ${verificationFeedback.tone === "success"
                            ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-200"
                            : verificationFeedback.tone === "error"
                                ? "border-red-200 bg-red-50 text-red-700 dark:border-red-400/50 dark:bg-red-500/10 dark:text-red-200"
                                : "border-slate-200 bg-white/70 text-slate-600 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-200"}`, children: [_jsx("p", { children: verificationFeedback.message }), typeof verificationFeedback.remaining === "number" ? (_jsx("p", { className: "mt-1", children: t("settings.verificationPending.remaining", {
                                    count: verificationFeedback.remaining,
                                }) })) : null, typeof verificationFeedback.retryAfter === "number" ? (_jsx("p", { className: "mt-1", children: t("settings.verificationPending.rateLimit", {
                                    seconds: verificationFeedback.retryAfter,
                                }) })) : null] })) : null] })), _jsx(GlassCard, { className: "space-y-6", children: _jsxs("form", { onSubmit: handleProfileSubmit, className: "space-y-6", children: [_jsxs("div", { children: [_jsx("h2", { className: "text-lg font-medium text-slate-800 dark:text-slate-100", children: t("settings.profileCard.title") }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("settings.profileCard.description") })] }), _jsx(AvatarUploader, { value: profileForm.avatar_url, onChange: (value) => {
                                setProfileForm((prev) => ({ ...prev, avatar_url: value }));
                            } }), _jsxs("div", { className: "grid gap-4 sm:grid-cols-2", children: [_jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "profile-username", children: t("settings.profileCard.username") }), _jsx(Input, { id: "profile-username", value: profileForm.username, autoComplete: "username", onChange: (event) => {
                                                const value = event.target.value;
                                                setProfileForm((prev) => ({ ...prev, username: value }));
                                                setProfileErrors((prev) => ({ ...prev, username: undefined }));
                                            }, required: true }), profileErrors.username ? _jsx("p", { className: "text-xs text-red-500", children: profileErrors.username }) : null] }), _jsxs("div", { className: "space-y-2", children: [_jsxs("label", { className: "flex items-center gap-2 text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "profile-email", children: [t("settings.profileCard.email"), isEmailVerified ? (_jsx(Badge, { className: "bg-emerald-500/15 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200", children: t("settings.emailStatus.verified", "已验证") })) : (_jsx(Badge, { variant: "outline", className: "border-amber-400/60 text-amber-700 dark:border-amber-300/50 dark:text-amber-200", children: t("settings.emailStatus.pending", "未验证") }))] }), _jsx(Input, { id: "profile-email", type: "email", value: profileForm.email, autoComplete: "email", onChange: (event) => {
                                                const value = event.target.value;
                                                setProfileForm((prev) => ({ ...prev, email: value }));
                                                setProfileErrors((prev) => ({ ...prev, email: undefined }));
                                            }, required: true }), profileErrors.email ? _jsx("p", { className: "text-xs text-red-500", children: profileErrors.email }) : null, isEmailVerified ? null : (_jsx(_Fragment, { children: _jsx("p", { className: "mt-2 text-xs text-amber-700 dark:text-amber-200", children: verificationTargetEmail
                                                    ? t("settings.verificationPending.emailLabel", { email: verificationTargetEmail })
                                                    : t("settings.verificationPending.emailLabelFallback") }) }))] })] }), _jsxs("div", { className: "grid gap-4 sm:grid-cols-2", children: [_jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", htmlFor: "profile-model", children: t("settings.profileCard.preferredModel") }), _jsx(Input, { id: "profile-model", value: profileForm.preferred_model, onChange: (event) => setProfileForm((prev) => ({ ...prev, preferred_model: event.target.value })), placeholder: t("settings.profileCard.preferredModelPlaceholder") ?? "" })] }), _jsxs("div", { className: "space-y-2", children: [_jsx("span", { className: "text-sm font-medium text-slate-700 dark:text-slate-200", children: t("settings.profileCard.syncEnabled") }), _jsxs("button", { type: "button", className: "flex w-full items-center justify-between rounded-xl border border-white/60 bg-white/70 px-4 py-2 text-left text-sm text-slate-700 shadow-sm transition hover:border-primary dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200", onClick: () => setProfileForm((prev) => ({ ...prev, sync_enabled: !prev.sync_enabled })), children: [_jsx("span", { children: t("settings.profileCard.syncEnabledLabel") }), _jsx("span", { className: "text-xs text-primary", children: syncLabel })] })] })] }), _jsx("div", { className: "flex justify-end", children: _jsx(Button, { type: "submit", disabled: mutation.isPending, children: mutation.isPending ? t("common.loading") : t("settings.profileCard.save") }) })] }) }), _jsxs(GlassCard, { className: "space-y-4", children: [_jsxs("div", { children: [_jsx("h2", { className: "text-lg font-medium text-slate-800 dark:text-slate-100", children: t("settings.themeCard.title") }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("settings.themeCard.description") })] }), _jsx("div", { className: "grid gap-3 sm:grid-cols-3", children: themeOptions.map((option) => {
                            const isActive = option.value === theme;
                            return (_jsxs("button", { type: "button", onClick: () => setTheme(option.value), className: `rounded-xl border px-4 py-3 text-left transition ${isActive
                                    ? "border-primary bg-primary/10 text-primary"
                                    : "border-white/60 bg-white/80 text-slate-700 hover:border-primary dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"}`, children: [_jsx("span", { className: "block text-sm font-medium dark:text-slate-100", children: option.label }), _jsx("span", { className: "mt-1 block text-xs text-slate-500 dark:text-slate-400", children: option.value === "system"
                                            ? t("settings.themeCard.systemHint", {
                                                mode: resolvedTheme === "dark"
                                                    ? t("settings.themeCard.systemHintDark", "深色")
                                                    : t("settings.themeCard.systemHintLight", "浅色"),
                                            })
                                            : option.description })] }, option.value));
                        }) })] }), _jsxs(GlassCard, { className: "space-y-4", children: [_jsxs("div", { children: [_jsx("h2", { className: "text-lg font-medium text-slate-800 dark:text-slate-100", children: t("settings.languageCardTitle") }), _jsx("p", { className: "mt-1 text-sm text-slate-500 dark:text-slate-400", children: t("settings.languageCardDescription") })] }), _jsxs("div", { className: "flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between", children: [_jsx("label", { className: "text-sm font-medium text-slate-600 dark:text-slate-300", htmlFor: "language-select", children: t("settings.languageSelectLabel") }), _jsx("select", { id: "language-select", className: "w-full rounded-xl border border-white/60 bg-white/80 px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/40 sm:w-64 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200", value: language, onChange: handleLanguageChange, children: LANGUAGE_OPTIONS.map((option) => (_jsx("option", { value: option.code, children: option.label }, option.code))) })] })] })] }));
}
