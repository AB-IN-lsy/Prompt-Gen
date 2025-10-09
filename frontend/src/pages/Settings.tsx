/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:33:13
 * @FilePath: \electron-go-app\frontend\src\pages\Settings.tsx
 * @LastEditTime: 2025-10-09 23:33:18
 */
import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";

import { GlassCard } from "../components/ui/glass-card";
import { useAppSettings } from "../hooks/useAppSettings";
import { LANGUAGE_OPTIONS } from "../i18n";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { AvatarUploader } from "../components/account/AvatarUploader";
import { useAuth } from "../hooks/useAuth";
import { updateCurrentUser, type UpdateCurrentUserRequest } from "../lib/api";
import { ApiError } from "../lib/errors";

export default function SettingsPage() {
    const { t } = useTranslation();
    const { language, setLanguage } = useAppSettings();
    const profile = useAuth((state) => state.profile);
    const setProfile = useAuth((state) => state.setProfile);

    const [profileForm, setProfileForm] = useState({
        username: profile?.user.username ?? "",
        email: profile?.user.email ?? "",
        avatar_url: profile?.user.avatar_url ?? "",
        preferred_model: profile?.settings.preferred_model ?? "",
        sync_enabled: profile?.settings.sync_enabled ?? false,
    });

    const [profileErrors, setProfileErrors] = useState<{ username?: string; email?: string }>(() => ({
        username: undefined,
        email: undefined,
    }));

    useEffect(() => {
        setProfileForm({
            username: profile?.user.username ?? "",
            email: profile?.user.email ?? "",
            avatar_url: profile?.user.avatar_url ?? "",
            preferred_model: profile?.settings.preferred_model ?? "",
            sync_enabled: profile?.settings.sync_enabled ?? false,
        });
        setProfileErrors({ username: undefined, email: undefined });
    }, [profile]);

    const mutation = useMutation({
        mutationFn: async (payload: UpdateCurrentUserRequest) => updateCurrentUser(payload),
        onSuccess: (data) => {
            setProfile(data);
            toast.success(t("settings.profileSaveSuccess"));
        },
        onError: (error) => {
            if (error instanceof ApiError && error.code === "CONFLICT") {
                const details = (error.details as { field?: string; fields?: string[] } | undefined) ?? {};
                const conflictFields = new Set<string>();
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

    const handleLanguageChange = (event: ChangeEvent<HTMLSelectElement>) => {
        const value = event.target.value;
        if (LANGUAGE_OPTIONS.some((option) => option.code === value)) {
            setLanguage(value as (typeof LANGUAGE_OPTIONS)[number]["code"]);
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

    const handleProfileSubmit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (mutation.isPending) {
            return;
        }
        const errors = validateProfile();
        if (Object.values(errors).some(Boolean)) {
            return;
        }

        const payload: UpdateCurrentUserRequest = {
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

    const syncLabel = useMemo(
        () => (profileForm.sync_enabled ? t("settings.syncEnabledOn") : t("settings.syncEnabledOff")),
        [profileForm.sync_enabled, t]
    );

    return (
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-6">
            <header className="flex flex-col gap-1">
                <h1 className="text-2xl font-semibold text-slate-800">{t("settings.title")}</h1>
                <p className="text-sm text-slate-500">{t("settings.subtitle")}</p>
            </header>

            <GlassCard className="space-y-6">
                <form onSubmit={handleProfileSubmit} className="space-y-6">
                    <div>
                        <h2 className="text-lg font-medium text-slate-800">{t("settings.profileCard.title")}</h2>
                        <p className="mt-1 text-sm text-slate-500">{t("settings.profileCard.description")}</p>
                    </div>

                    <AvatarUploader
                        value={profileForm.avatar_url}
                        onChange={(value) => {
                            setProfileForm((prev) => ({ ...prev, avatar_url: value }));
                        }}
                    />

                    <div className="grid gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                            <label className="text-sm font-medium text-slate-700" htmlFor="profile-username">
                                {t("settings.profileCard.username")}
                            </label>
                            <Input
                                id="profile-username"
                                value={profileForm.username}
                                autoComplete="username"
                                onChange={(event) => {
                                    const value = event.target.value;
                                    setProfileForm((prev) => ({ ...prev, username: value }));
                                    setProfileErrors((prev) => ({ ...prev, username: undefined }));
                                }}
                                required
                            />
                            {profileErrors.username ? <p className="text-xs text-red-500">{profileErrors.username}</p> : null}
                        </div>

                        <div className="space-y-2">
                            <label className="text-sm font-medium text-slate-700" htmlFor="profile-email">
                                {t("settings.profileCard.email")}
                            </label>
                            <Input
                                id="profile-email"
                                type="email"
                                value={profileForm.email}
                                autoComplete="email"
                                onChange={(event) => {
                                    const value = event.target.value;
                                    setProfileForm((prev) => ({ ...prev, email: value }));
                                    setProfileErrors((prev) => ({ ...prev, email: undefined }));
                                }}
                                required
                            />
                            {profileErrors.email ? <p className="text-xs text-red-500">{profileErrors.email}</p> : null}
                        </div>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                            <label className="text-sm font-medium text-slate-700" htmlFor="profile-model">
                                {t("settings.profileCard.preferredModel")}
                            </label>
                            <Input
                                id="profile-model"
                                value={profileForm.preferred_model}
                                onChange={(event) => setProfileForm((prev) => ({ ...prev, preferred_model: event.target.value }))}
                                placeholder={t("settings.profileCard.preferredModelPlaceholder") ?? ""}
                            />
                        </div>

                        <div className="space-y-2">
                            <span className="text-sm font-medium text-slate-700">{t("settings.profileCard.syncEnabled")}</span>
                            <button
                                type="button"
                                className="flex w-full items-center justify-between rounded-xl border border-white/60 bg-white/70 px-4 py-2 text-left text-sm text-slate-700 shadow-sm transition hover:border-primary"
                                onClick={() => setProfileForm((prev) => ({ ...prev, sync_enabled: !prev.sync_enabled }))}
                            >
                                <span>{t("settings.profileCard.syncEnabledLabel")}</span>
                                <span className="text-xs text-primary">{syncLabel}</span>
                            </button>
                        </div>
                    </div>

                    <div className="flex justify-end">
                        <Button type="submit" disabled={mutation.isPending}>
                            {mutation.isPending ? t("common.loading") : t("settings.profileCard.save")}
                        </Button>
                    </div>
                </form>
            </GlassCard>

            <GlassCard className="space-y-4">
                <div>
                    <h2 className="text-lg font-medium text-slate-800">{t("settings.languageCardTitle")}</h2>
                    <p className="mt-1 text-sm text-slate-500">{t("settings.languageCardDescription")}</p>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <label className="text-sm font-medium text-slate-600" htmlFor="language-select">
                        {t("settings.languageSelectLabel")}
                    </label>
                    <select
                        id="language-select"
                        className="w-full rounded-xl border border-white/60 bg-white/80 px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/40 sm:w-64"
                        value={language}
                        onChange={handleLanguageChange}
                    >
                        {LANGUAGE_OPTIONS.map((option) => (
                            <option key={option.code} value={option.code}>
                                {option.label}
                            </option>
                        ))}
                    </select>
                </div>
            </GlassCard>
        </div>
    );
}
