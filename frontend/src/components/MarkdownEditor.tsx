import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import {
  Bold,
  Code,
  Code2,
  Link as LinkIcon,
  List,
  ListOrdered,
  Quote,
  Italic,
} from "lucide-react";

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
  hint,
}: MarkdownEditorProps) {
  const { t } = useTranslation();
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [activeTab, setActiveTab] = useState<"edit" | "preview" | "split">(
    "edit",
  );

  const previewContent = useMemo(() => {
    if (value.trim().length === 0 && placeholder) {
      return `> ${placeholder}`;
    }
    return value;
  }, [placeholder, value]);

  const focusTextarea = useCallback(
    (start: number, end: number) => {
      const textarea = textareaRef.current;
      if (!textarea) {
        return;
      }
      requestAnimationFrame(() => {
        textarea.focus();
        textarea.setSelectionRange(start, end);
      });
    },
    [],
  );

  const wrapSelection = useCallback(
    (prefix: string, suffix: string, placeholderText: string) => {
      const textarea = textareaRef.current;
      if (!textarea) {
        return;
      }
      const start = textarea.selectionStart ?? value.length;
      const end = textarea.selectionEnd ?? value.length;
      const selected = value.slice(start, end);
      const needsPlaceholder = selected.length === 0;
      const inner = needsPlaceholder ? placeholderText : selected;
      const formatted = `${prefix}${inner}${suffix}`;
      const nextValue = `${value.slice(0, start)}${formatted}${value.slice(end)}`;
      onChange(nextValue);
      const selectionStart = start + prefix.length;
      const selectionEnd = selectionStart + inner.length;
      focusTextarea(selectionStart, needsPlaceholder ? selectionEnd : selectionEnd);
    },
    [focusTextarea, onChange, value],
  );

  const applyList = useCallback(
    (ordered: boolean, placeholderText: string) => {
      const textarea = textareaRef.current;
      if (!textarea) {
        return;
      }
      const start = textarea.selectionStart ?? value.length;
      const end = textarea.selectionEnd ?? value.length;
      const selected = value.slice(start, end);
      const lines = selected ? selected.split("\n") : [""];
      const formattedLines = lines.map((line, index) => {
        const marker = ordered ? `${index + 1}. ` : "- ";
        const content = line.trim().length === 0 ? placeholderText : line;
        return `${marker}${content}`;
      });
      const formatted = formattedLines.join("\n");
      const nextValue = `${value.slice(0, start)}${formatted}${value.slice(end)}`;
      onChange(nextValue);
      const markerLength = ordered ? 3 : 2;
      focusTextarea(start + markerLength, start + markerLength + placeholderText.length);
    },
    [focusTextarea, onChange, value],
  );

  const handleToolbarAction = useCallback(
    (action: string) => {
      switch (action) {
        case "bold":
          wrapSelection(
            "**",
            "**",
            t("promptWorkbench.markdownPlaceholders.bold", {
              defaultValue: "加粗文本",
            }),
          );
          break;
        case "italic":
          wrapSelection(
            "_",
            "_",
            t("promptWorkbench.markdownPlaceholders.italic", {
              defaultValue: "斜体文本",
            }),
          );
          break;
        case "inline_code":
          wrapSelection(
            "`",
            "`",
            t("promptWorkbench.markdownPlaceholders.inlineCode", {
              defaultValue: "code",
            }),
          );
          break;
        case "code_block":
          wrapSelection(
            "```\n",
            "\n```",
            t("promptWorkbench.markdownPlaceholders.codeBlock", {
              defaultValue: "在此输入代码",
            }),
          );
          break;
        case "quote":
          wrapSelection(
            "> ",
            "",
            t("promptWorkbench.markdownPlaceholders.quote", {
              defaultValue: "引用内容",
            }),
          );
          break;
        case "unordered_list":
          applyList(
            false,
            t("promptWorkbench.markdownPlaceholders.listItem", {
              defaultValue: "列表项",
            }),
          );
          break;
        case "ordered_list":
          applyList(
            true,
            t("promptWorkbench.markdownPlaceholders.listItem", {
              defaultValue: "列表项",
            }),
          );
          break;
        case "link":
          {
            const linkText = t("promptWorkbench.markdownPlaceholders.linkText", {
              defaultValue: "链接文本",
            });
            const linkUrl = t("promptWorkbench.markdownPlaceholders.linkUrl", {
              defaultValue: "https://example.com",
            });
            wrapSelection(`[`, `](${linkUrl})`, linkText);
          }
          break;
        default:
          break;
      }
    },
    [applyList, t, wrapSelection],
  );

  const renderTextarea = useCallback(
    (textAreaClassName?: string) => (
      <textarea
        ref={textareaRef}
        className={cn(
          "h-full w-full resize-none bg-transparent px-4 py-3 text-sm text-slate-700 outline-none transition placeholder:text-slate-300 dark:text-slate-200 dark:placeholder:text-slate-500",
          textAreaClassName,
        )}
        style={{ minHeight }}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    ),
    [minHeight, onChange, placeholder, value],
  );

  const renderPreview = useCallback(
    (previewClassName?: string) => (
      <div
        className={cn(
          "markdown-preview h-full overflow-auto px-4 py-3 text-sm text-slate-700 transition dark:text-slate-200",
          previewClassName,
        )}
        style={{ minHeight }}
      >
        <ReactMarkdown remarkPlugins={[remarkGfm]}>
          {previewContent || ""}
        </ReactMarkdown>
      </div>
    ),
    [minHeight, previewContent],
  );

  const toolbarButtons = useMemo(
    () => [
      {
        action: "bold",
        icon: Bold,
        label: t("promptWorkbench.markdownToolbar.bold", { defaultValue: "加粗" }),
      },
      {
        action: "italic",
        icon: Italic,
        label: t("promptWorkbench.markdownToolbar.italic", { defaultValue: "斜体" }),
      },
      {
        action: "inline_code",
        icon: Code2,
        label: t("promptWorkbench.markdownToolbar.inlineCode", {
          defaultValue: "行内代码",
        }),
      },
      {
        action: "code_block",
        icon: Code,
        label: t("promptWorkbench.markdownToolbar.codeBlock", {
          defaultValue: "代码块",
        }),
      },
      {
        action: "quote",
        icon: Quote,
        label: t("promptWorkbench.markdownToolbar.quote", {
          defaultValue: "引用",
        }),
      },
      {
        action: "unordered_list",
        icon: List,
        label: t("promptWorkbench.markdownToolbar.unorderedList", {
          defaultValue: "无序列表",
        }),
      },
      {
        action: "ordered_list",
        icon: ListOrdered,
        label: t("promptWorkbench.markdownToolbar.orderedList", {
          defaultValue: "有序列表",
        }),
      },
      {
        action: "link",
        icon: LinkIcon,
        label: t("promptWorkbench.markdownToolbar.link", { defaultValue: "插入链接" }),
      },
    ],
    [t],
  );

  const shouldShowToolbar = activeTab !== "preview";

  return (
    <div
      className={cn(
        "flex h-full flex-col rounded-2xl border border-white/80 bg-white/60 shadow-inner transition-colors dark:border-slate-800 dark:bg-slate-900/60",
        className,
      )}
    >
      <div className="flex items-center gap-2 border-b border-white/60 px-4 py-2 text-xs font-medium text-slate-500 transition-colors dark:border-slate-800 dark:text-slate-400">
        <button
          type="button"
          onClick={() => setActiveTab("edit")}
          className={cn(
            "rounded-full px-3 py-1 transition",
            activeTab === "edit"
              ? "bg-primary text-white"
              : "bg-transparent text-slate-500 hover:bg-primary/10",
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
              : "bg-transparent text-slate-500 hover:bg-primary/10",
          )}
        >
          {t("promptWorkbench.markdownPreview", { defaultValue: "预览" })}
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("split")}
          className={cn(
            "rounded-full px-3 py-1 transition",
            activeTab === "split"
              ? "bg-primary text-white"
              : "bg-transparent text-slate-500 hover:bg-primary/10",
          )}
        >
          {t("promptWorkbench.markdownSplit", { defaultValue: "并排" })}
        </button>
        <div className="ml-auto text-[11px] text-slate-400 dark:text-slate-500">
          {t("promptWorkbench.markdownLabel", { defaultValue: "Markdown" })}
        </div>
      </div>
      {shouldShowToolbar ? (
        <div className="flex flex-wrap items-center gap-2 border-b border-white/60 px-4 py-2 text-xs transition-colors dark:border-slate-800">
          <span className="mr-2 text-slate-400 dark:text-slate-500">
            {t("promptWorkbench.markdownToolbar.title", {
              defaultValue: "常用格式",
            })}
            ：
          </span>
          {toolbarButtons.map(({ action, icon: Icon, label }) => (
            <button
              key={action}
              type="button"
              onClick={() => handleToolbarAction(action)}
              className="flex items-center gap-1 rounded-md border border-white/70 bg-white/80 px-2 py-1 text-slate-600 shadow-sm transition hover:border-primary/40 hover:text-primary focus:outline-none dark:border-slate-700 dark:bg-slate-800/70 dark:text-slate-200 dark:hover:border-primary/40"
              title={label}
              aria-label={label}
            >
              <Icon className="h-3.5 w-3.5" />
              <span>{label}</span>
            </button>
          ))}
        </div>
      ) : null}
      <div className="flex-1 overflow-hidden">
        {activeTab === "edit" ? (
          renderTextarea()
        ) : activeTab === "preview" ? (
          renderPreview()
        ) : (
          <div className="flex h-full flex-col gap-4 p-4 md:flex-row md:gap-6">
            <div className="md:w-1/2">
              <div className="mb-2 text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("promptWorkbench.markdownSplitEdit", {
                  defaultValue: "编辑",
                })}
              </div>
              <div className="h-full rounded-xl border border-white/70 bg-white/80 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
                {renderTextarea("rounded-xl border-none px-4 py-3")}
              </div>
            </div>
            <div className="md:w-1/2">
              <div className="mb-2 text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("promptWorkbench.markdownSplitPreview", {
                  defaultValue: "实时预览",
                })}
              </div>
              <div className="h-full rounded-xl border border-white/70 bg-white/80 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
                {renderPreview("rounded-xl border-none px-4 py-3")}
              </div>
            </div>
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
