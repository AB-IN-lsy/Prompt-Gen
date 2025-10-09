/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:55:37
 * @FilePath: \electron-go-app\frontend\src\pages\Login.tsx
 * @LastEditTime: 2025-10-10 00:59:05
 */
import { FormEvent, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { AuthLayout } from "../components/layout/AuthLayout";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { login, type LoginRequest } from "../lib/api";
import { ApiError } from "../lib/errors";
import { useAuth } from "../hooks/useAuth";

export default function LoginPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const [credentials, setCredentials] = useState<LoginRequest>({ email: "", password: "" });
    // 行内校验的错误信息容器，按字段分别存储提示文案。
    const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({});
    const authenticate = useAuth((state) => state.authenticate);

    // 单字段校验逻辑：在输入过程与提交前复用，确保提示一致。
    const validateField = (field: keyof LoginRequest, value: string): string | undefined => {
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
    const validateAll = (): { email?: string; password?: string } => {
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
            navigate("/prompt-workbench", { replace: true });
        },
        onError: (error: unknown) => {
            // 后端错误优先显示字典翻译，兜底使用通用提示。
            let message = t("errors.generic");
            if (error instanceof ApiError) {
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
        !credentials.email.trim() ||
        !credentials.password.trim();

    return (
        <AuthLayout title={t("auth.login.title") ?? ""} subtitle={t("auth.login.subtitle") ?? undefined}>
            <form className="space-y-5" onSubmit={handleSubmit}>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700" htmlFor="email">
                        {t("auth.form.email")}
                    </label>
                    <Input
                        id="email"
                        type="email"
                        autoComplete="email"
                        required
                        value={credentials.email}
                        onChange={(event) => {
                            const value = event.target.value;
                            setCredentials((prev) => ({ ...prev, email: value }));
                            setFieldErrors((prev) => ({ ...prev, email: validateField("email", value) }));
                        }}
                    />
                    {fieldErrors.email ? <p className="text-xs text-red-500">{fieldErrors.email}</p> : null}
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-slate-700" htmlFor="password">
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
                {backendError ? <p className="text-sm text-red-500">{backendError}</p> : null}
                <Button type="submit" className="w-full" disabled={isSubmitDisabled}>
                    {mutation.isPending ? t("common.loading") : t("auth.login.cta")}
                </Button>
            </form>
            <div className="text-center text-sm text-slate-500">
                {t("auth.login.switch")}{" "}
                <Link to="/register" className="font-medium text-primary hover:underline">
                    {t("auth.login.switchCta")}
                </Link>
            </div>
        </AuthLayout>
    );
}
