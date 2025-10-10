import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 19:40:00
 * @FilePath: \electron-go-app\frontend\src\pages\VerifyEmail.tsx
 * @LastEditTime: 2025-10-10 20:01:31
 */
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { requestEmailVerification } from "../lib/api";
import { ApiError } from "../lib/errors";
/**
 * 邮件验证页：仅提供重新发送验证邮件的入口，并引导用户前往邮箱点击链接完成验证。
 */
export default function VerifyEmailPage() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const location = useLocation();
    const [email, setEmail] = useState("");
    // 解析 query string 中的 token 与 email（若前端附带）。
    useEffect(() => {
        const params = new URLSearchParams(location.search);
        const queryEmail = params.get("email") ?? "";
        if (queryEmail) {
            setEmail(queryEmail.trim());
        }
    }, [location.search]);
    const resendMutation = useMutation({
        mutationFn: async () => {
            if (!email.trim()) {
                throw new ApiError({ message: t("auth.verification.emailRequired") ?? "" });
            }
            return requestEmailVerification(email.trim());
        },
        onSuccess: (result) => {
            if (result.issued && result.token) {
                toast.success(t("auth.verification.sent"));
            }
            else {
                toast.success(t("auth.verification.sent"));
            }
        },
        onError: (error) => {
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
    return (_jsx(AuthLayout, { title: t("auth.verification.pageTitle") ?? "", subtitle: t("auth.verification.pageSubtitle") ?? undefined, children: _jsxs("div", { className: "space-y-6", children: [_jsxs("div", { className: "space-y-2 text-sm text-slate-600 dark:text-slate-300", children: [_jsx("p", { children: t("auth.verification.pageIntro") }), _jsx("p", { children: t("auth.verification.pageSteps") })] }), _jsxs("div", { className: "space-y-4 rounded-xl border border-slate-200 bg-white/70 p-4 shadow-sm transition-colors dark:border-slate-700 dark:bg-slate-900/60", children: [_jsxs("div", { className: "space-y-2", children: [_jsx("label", { className: "text-xs font-medium text-slate-600 dark:text-slate-300", htmlFor: "verify-email-input", children: t("auth.form.email") }), _jsx(Input, { id: "verify-email-input", value: email, type: "email", placeholder: t("auth.verification.emailPlaceholder") ?? "", onChange: (event) => setEmail(event.target.value), disabled: resendMutation.isPending })] }), _jsx(Button, { type: "button", variant: "outline", className: "w-full", disabled: resendMutation.isPending, onClick: () => resendMutation.mutate(), children: resendMutation.isPending ? t("common.loading") : t("auth.verification.send") }), _jsx("p", { className: "text-xs text-slate-500 dark:text-slate-400", children: t("auth.verification.emailHint", "我们会把验证链接发送到上面的邮箱，请留意收件箱与垃圾邮件夹。") })] }), _jsx(Button, { type: "button", variant: "ghost", className: "w-full text-primary underline-offset-4 hover:underline", onClick: () => navigate("/login"), children: t("auth.verification.backToLogin") })] }) }));
}
