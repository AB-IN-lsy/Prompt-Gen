import { ReactNode, useEffect, useRef, useState } from "react";
import { MoreHorizontal } from "lucide-react";

import { Button, type ButtonProps } from "./button";
import { cn } from "../../lib/utils";

export interface MoreActionItem {
  key: string;
  label: string;
  onSelect: () => void;
  icon?: ReactNode;
  disabled?: boolean;
  danger?: boolean;
}

type Align = "left" | "right";

interface MoreActionsMenuProps {
  items: MoreActionItem[];
  triggerLabel: string;
  disabled?: boolean;
  align?: Align;
  triggerIcon?: ReactNode;
  triggerVariant?: ButtonProps["variant"];
  triggerSize?: ButtonProps["size"];
  triggerClassName?: string;
  triggerWrapperClassName?: string;
  menuClassName?: string;
}

export function MoreActionsMenu({
  items,
  triggerLabel,
  disabled = false,
  align = "right",
  triggerIcon,
  triggerVariant = "outline",
  triggerSize = "sm",
  triggerClassName,
  triggerWrapperClassName,
  menuClassName,
}: MoreActionsMenuProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const hasItems = items.length > 0;
  const triggerDisabled = disabled || !hasItems;

  useEffect(() => {
    if (!open) {
      return;
    }
    const handleClick = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("keydown", handleKeydown);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKeydown);
    };
  }, [open]);

  useEffect(() => {
    if (triggerDisabled && open) {
      setOpen(false);
    }
  }, [triggerDisabled, open]);

  const alignmentClass = align === "left" ? "left-0" : "right-0";

  return (
    <div className={cn("relative", triggerWrapperClassName)} ref={containerRef}>
      <Button
        type="button"
        variant={triggerVariant}
        size={triggerSize}
        className={cn("shadow-sm dark:shadow-none", triggerClassName)}
        disabled={triggerDisabled}
        onClick={() => {
          if (!triggerDisabled) {
            setOpen((prev) => !prev);
          }
        }}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        {triggerIcon ?? <MoreHorizontal className="mr-2 h-4 w-4" />}
        {triggerLabel}
      </Button>
      {open ? (
        <div
          className={cn(
            "absolute z-40 mt-2 w-60 rounded-2xl border border-slate-200 bg-white/95 p-2 shadow-xl backdrop-blur-lg dark:border-slate-700 dark:bg-slate-900/95",
            alignmentClass,
            menuClassName,
          )}
          role="menu"
        >
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              className={cn(
                "flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition",
                item.disabled
                  ? "cursor-not-allowed opacity-50"
                  : item.danger
                    ? "text-rose-600 hover:bg-rose-50 hover:text-rose-600 dark:text-rose-200 dark:hover:bg-rose-500/10"
                    : "text-slate-600 hover:bg-primary/10 hover:text-primary dark:text-slate-200 dark:hover:bg-primary/20",
              )}
              onClick={() => {
                if (item.disabled) {
                  return;
                }
                setOpen(false);
                item.onSelect();
              }}
              disabled={item.disabled}
              role="menuitem"
            >
              {item.icon ? (
                <span className="flex h-4 w-4 items-center justify-center text-current">{item.icon}</span>
              ) : null}
              <span className="flex-1 text-left">{item.label}</span>
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
