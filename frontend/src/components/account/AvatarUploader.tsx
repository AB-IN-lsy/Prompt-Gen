/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 02:34:34
 * @FilePath: \electron-go-app\frontend\src\components\account\AvatarUploader.tsx
 * @LastEditTime: 2025-10-10 02:34:38
 */
import { ChangeEvent, useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { UploadCloud, X } from "lucide-react";
import { uploadAvatar } from "../../lib/api";
import { ApiError } from "../../lib/errors";
import { Button } from "../ui/button";
import { cn, resolveAssetUrl } from "../../lib/utils";
import { toast } from "sonner";

// AvatarUploaderProps 定义头像上传组件的外部接口。
interface AvatarUploaderProps {
    value?: string;
    onChange: (value: string) => void;
    disabled?: boolean;
    title?: string;
    description?: string;
    size?: number;
}

// AvatarUploader 负责处理头像的选择、上传与预览：
// - 通过隐藏的 <input type="file"> 选择图片，并调用后端上传接口；
// - 上传成功后回调最新的头像 URL；
// - 支持移除头像（回调空字符串），方便用户恢复默认头像。
export function AvatarUploader({
    value,
    onChange,
    disabled = false,
    title,
    description,
    size = 96,
}: AvatarUploaderProps) {
    const { t } = useTranslation();
    const [isUploading, setIsUploading] = useState(false);
    const inputRef = useRef<HTMLInputElement | null>(null);

    const handleFileChange = useCallback(
        async (event: ChangeEvent<HTMLInputElement>) => {
            const file = event.target.files?.[0];
            if (!file) {
                return;
            }

            setIsUploading(true);
            try {
                const url = await uploadAvatar(file);
                onChange(url);
                toast.success(t("avatarUploader.uploadSuccess"));
            } catch (error) {
                const message =
                    error instanceof ApiError ? error.message : t("avatarUploader.uploadError");
                toast.error(message || t("avatarUploader.uploadError"));
            } finally {
                setIsUploading(false);
                // 允许重复选择同一文件。
                event.target.value = "";
            }
        },
        [onChange, t]
    );

    const handleRemove = useCallback(() => {
        onChange("");
    }, [onChange]);

    const displayTitle = title ?? t("avatarUploader.title");
    const displayDescription = description ?? t("avatarUploader.description");
    const resolvedSrc = resolveAssetUrl(value);

    return (
        <div className="space-y-4">
            <div>
                <h3 className="text-sm font-medium text-slate-700">{displayTitle}</h3>
                <p className="text-xs text-slate-500">{displayDescription}</p>
            </div>
            <div className="flex items-center gap-6">
                <div
                    className={cn(
                        "relative flex items-center justify-center overflow-hidden rounded-full border border-white/60 bg-white/70 shadow-sm",
                        isUploading && "animate-pulse"
                    )}
                    style={{ width: size, height: size }}
                >
                    {resolvedSrc ? (
                        <img src={resolvedSrc} alt={displayTitle} className="h-full w-full object-cover" />
                    ) : (
                        <UploadCloud className="h-6 w-6 text-slate-400" />
                    )}
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
                    <input
                        ref={inputRef}
                        type="file"
                        accept="image/*"
                        className="hidden"
                        onChange={handleFileChange}
                        disabled={disabled || isUploading}
                    />
                    <Button
                        type="button"
                        disabled={disabled || isUploading}
                        variant="outline"
                        onClick={() => inputRef.current?.click()}
                    >
                        {isUploading
                            ? t("avatarUploader.uploading")
                            : value
                                ? t("avatarUploader.change")
                                : t("avatarUploader.upload")}
                    </Button>
                    {value ? (
                        <Button
                            type="button"
                            variant="ghost"
                            className="flex items-center gap-2 text-xs text-slate-500"
                            onClick={handleRemove}
                            disabled={disabled || isUploading}
                        >
                            <X className="h-3 w-3" />
                            {t("avatarUploader.remove")}
                        </Button>
                    ) : null}
                </div>
            </div>
        </div>
    );
}
