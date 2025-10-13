import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { cn } from "../lib/utils";

interface MarkdownEditorProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    minHeight?: number;
    className?: string;
    hint?: string;
}

// MarkdownEditor 提供基础的 Markdown 编辑与预览能力，满足“草稿支持 Markdown 解释器”的需求。
export function MarkdownEditor({
    value,
    onChange,
    placeholder,
    minHeight = 360,
    className,
    hint
}: MarkdownEditorProps) {
    const { t } = useTranslation();
    const [activeTab, setActiveTab] = useState<"edit" | "preview">("edit");

    const previewContent = useMemo(() => {
        if (value.trim().length === 0 && placeholder) {
            return `> ${placeholder}`;
        }
        return value;
    }, [placeholder, value]);

    return (
        <div className={cn("flex h-full flex-col rounded-2xl border border-white/80 bg-white/60 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/60", className)}>
            <div className="flex items-center gap-2 border-b border-white/60 px-4 py-2 text-xs font-medium text-slate-500 transition-colors dark:border-slate-800 dark:text-slate-400">
                <button
                    type="button"
                    onClick={() => setActiveTab("edit")}
                    className={cn(
                        "rounded-full px-3 py-1 transition",
                        activeTab === "edit"
                            ? "bg-primary text-white"
                            : "bg-transparent text-slate-500 hover:bg-primary/10"
                    )}
                >
                    {t("promptWorkbench.markdownEdit", { defaultValue: "编辑" })}
                </button>
                <button
                    type="button"
                    onClick={() => setActiveTab("preview")}
                    className={cn(
                        "rounded-full px-3 py-1 transition",
                        activeTab === "preview"
                            ? "bg-primary text-white"
                            : "bg-transparent text-slate-500 hover:bg-primary/10"
                    )}
                >
                    {t("promptWorkbench.markdownPreview", { defaultValue: "预览" })}
                </button>
                <div className="ml-auto text-[11px] text-slate-400 dark:text-slate-500">
                    {t("promptWorkbench.markdownLabel", { defaultValue: "Markdown" })}
                </div>
            </div>
            <div className="flex-1 overflow-hidden">
                {activeTab === "edit" ? (
                    <textarea
                        className="h-full w-full resize-none bg-transparent px-4 py-3 text-sm text-slate-700 outline-none transition placeholder:text-slate-300 dark:text-slate-200 dark:placeholder:text-slate-500"
                        style={{ minHeight }}
                        value={value}
                        onChange={(event) => onChange(event.target.value)}
                        placeholder={placeholder}
                    />
                ) : (
                    <div
                        className="markdown-preview h-full overflow-auto px-4 py-3 text-sm text-slate-700 transition dark:text-slate-200"
                        style={{ minHeight }}
                    >
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {previewContent || ""}
                        </ReactMarkdown>
                    </div>
                )}
            </div>
            {hint ? (
                <div className="border-t border-white/60 px-4 py-2 text-xs text-slate-400 transition-colors dark:border-slate-800 dark:text-slate-500">
                    {hint}
                </div>
            ) : null}
        </div>
    );
}
