import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 02:34:34
 * @FilePath: \electron-go-app\frontend\src\components\account\AvatarUploader.tsx
 * @LastEditTime: 2025-10-10 02:34:38
 */
import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { UploadCloud, X } from "lucide-react";
import { uploadAvatar } from "../../lib/api";
import { ApiError } from "../../lib/errors";
import { Button } from "../ui/button";
import { cn } from "../../lib/utils";
import { toast } from "sonner";
// AvatarUploader 负责处理头像的选择、上传与预览：
// - 通过隐藏的 <input type="file"> 选择图片，并调用后端上传接口；
// - 上传成功后回调最新的头像 URL；
// - 支持移除头像（回调空字符串），方便用户恢复默认头像。
export function AvatarUploader({ value, onChange, disabled = false, title, description, size = 96, }) {
    const { t } = useTranslation();
    const [isUploading, setIsUploading] = useState(false);
    const inputRef = useRef(null);
    const handleFileChange = useCallback(async (event) => {
        const file = event.target.files?.[0];
        if (!file) {
            return;
        }
        setIsUploading(true);
        try {
            const url = await uploadAvatar(file);
            onChange(url);
            toast.success(t("avatarUploader.uploadSuccess"));
        }
        catch (error) {
            const message = error instanceof ApiError ? error.message : t("avatarUploader.uploadError");
            toast.error(message || t("avatarUploader.uploadError"));
        }
        finally {
            setIsUploading(false);
            // 允许重复选择同一文件。
            event.target.value = "";
        }
    }, [onChange, t]);
    const handleRemove = useCallback(() => {
        onChange("");
    }, [onChange]);
    const displayTitle = title ?? t("avatarUploader.title");
    const displayDescription = description ?? t("avatarUploader.description");
    return (_jsxs("div", { className: "space-y-4", children: [_jsxs("div", { children: [_jsx("h3", { className: "text-sm font-medium text-slate-700", children: displayTitle }), _jsx("p", { className: "text-xs text-slate-500", children: displayDescription })] }), _jsxs("div", { className: "flex items-center gap-6", children: [_jsx("div", { className: cn("relative flex items-center justify-center overflow-hidden rounded-full border border-white/60 bg-white/70 shadow-sm", isUploading && "animate-pulse"), style: { width: size, height: size }, children: value ? (_jsx("img", { src: value, alt: displayTitle, className: "h-full w-full object-cover" })) : (_jsx(UploadCloud, { className: "h-6 w-6 text-slate-400" })) }), _jsxs("div", { className: "flex flex-col gap-2 sm:flex-row", children: [_jsx("input", { ref: inputRef, type: "file", accept: "image/*", className: "hidden", onChange: handleFileChange, disabled: disabled || isUploading }), _jsx(Button, { type: "button", disabled: disabled || isUploading, variant: "outline", onClick: () => inputRef.current?.click(), children: isUploading
                                    ? t("avatarUploader.uploading")
                                    : value
                                        ? t("avatarUploader.change")
                                        : t("avatarUploader.upload") }), value ? (_jsxs(Button, { type: "button", variant: "ghost", className: "flex items-center gap-2 text-xs text-slate-500", onClick: handleRemove, disabled: disabled || isUploading, children: [_jsx(X, { className: "h-3 w-3" }), t("avatarUploader.remove")] })) : null] })] })] }));
}
