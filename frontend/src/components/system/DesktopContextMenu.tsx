import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { cn } from "../../lib/utils";

type MenuState = {
    visible: boolean;
    x: number;
    y: number;
    anchorX: number;
    anchorY: number;
    hasSelection: boolean;
    isEditable: boolean;
};

const initialState: MenuState = {
    visible: false,
    x: 0,
    y: 0,
    anchorX: 0,
    anchorY: 0,
    hasSelection: false,
    isEditable: false,
};

const EDIT_COMMANDS = ["undo", "redo", "cut", "copy", "paste", "delete", "selectAll", "reload"] as const;

type EditCommand = typeof EDIT_COMMANDS[number];

interface MenuItem {
    key: EditCommand;
    requireEditability?: boolean;
    requireSelection?: boolean;
}

const MENU_ITEMS: Array<MenuItem | "separator"> = [
    { key: "undo", requireEditability: true },
    { key: "redo", requireEditability: true },
    "separator",
    { key: "cut", requireEditability: true, requireSelection: true },
    { key: "copy", requireSelection: true },
    { key: "paste", requireEditability: true },
    { key: "delete", requireEditability: true, requireSelection: true },
    "separator",
    { key: "selectAll" },
    "separator",
    { key: "reload" },
];

function getShortcut(command: EditCommand, isMac: boolean): string {
    if (isMac) {
        switch (command) {
            case "undo":
                return "⌘Z";
            case "redo":
                return "⇧⌘Z";
            case "cut":
                return "⌘X";
            case "copy":
                return "⌘C";
            case "paste":
                return "⌘V";
            case "delete":
                return "⌫";
            case "selectAll":
                return "⌘A";
            case "reload":
                return "⌘R";
            default:
                return "";
        }
    }
    switch (command) {
        case "undo":
            return "Ctrl+Z";
        case "redo":
            return "Ctrl+Shift+Z";
        case "cut":
            return "Ctrl+X";
        case "copy":
            return "Ctrl+C";
        case "paste":
            return "Ctrl+V";
        case "delete":
            return "Del";
        case "selectAll":
            return "Ctrl+A";
        case "reload":
            return "Ctrl+R";
        default:
            return "";
    }
}

function isEditableElement(target: EventTarget | null): boolean {
    if (!target || !(target instanceof HTMLElement)) {
        return false;
    }
    const nodeName = target.nodeName.toLowerCase();
    return (
        target.isContentEditable ||
        nodeName === "input" ||
        nodeName === "textarea" ||
        target.getAttribute("role") === "textbox"
    );
}

export function DesktopContextMenu(): JSX.Element | null {
    const { t } = useTranslation();
    const [state, setState] = useState<MenuState>(initialState);
    const menuRef = useRef<HTMLDivElement | null>(null);

    useEffect(() => {
        if (typeof window === "undefined" || !window.desktop) {
            return;
        }

        const handleContextMenu = (event: MouseEvent) => {
            if (!window.desktop) {
                return;
            }
            event.preventDefault();
            const selection = window.getSelection();
            const hasSelection = Boolean(selection && selection.toString().trim().length > 0);
            const targetEditable = isEditableElement(event.target);
            const anchorX = event.clientX;
            const anchorY = event.clientY;
            setState({
                visible: true,
                x: anchorX,
                y: anchorY,
                anchorX,
                anchorY,
                hasSelection,
                isEditable: targetEditable,
            });
        };

        const hideMenu = () => {
            setState((prev) => (prev.visible ? initialState : prev));
        };

        document.addEventListener("contextmenu", handleContextMenu);
        document.addEventListener("click", hideMenu);
        document.addEventListener("keydown", hideMenu);

        return () => {
            document.removeEventListener("contextmenu", handleContextMenu);
            document.removeEventListener("click", hideMenu);
            document.removeEventListener("keydown", hideMenu);
        };
    }, []);

    useLayoutEffect(() => {
        if (!state.visible || typeof window === "undefined") {
            return;
        }
        const menuElement = menuRef.current;
        if (!menuElement) {
            return;
        }
        const rect = menuElement.getBoundingClientRect();
        const { innerWidth, innerHeight } = window;
        const viewportPadding = 8;
        const pointerOffset = 4;

        let nextX = state.anchorX + pointerOffset;
        let nextY = state.anchorY + pointerOffset;

        if (nextX + rect.width + viewportPadding > innerWidth) {
            const flippedX = state.anchorX - rect.width - pointerOffset;
            nextX = flippedX >= viewportPadding ? flippedX : Math.max(innerWidth - rect.width - viewportPadding, viewportPadding);
        } else if (nextX < viewportPadding) {
            nextX = viewportPadding;
        }

        if (nextY + rect.height + viewportPadding > innerHeight) {
            const flippedY = state.anchorY - rect.height - pointerOffset;
            nextY = flippedY >= viewportPadding ? flippedY : Math.max(innerHeight - rect.height - viewportPadding, viewportPadding);
        } else if (nextY < viewportPadding) {
            nextY = viewportPadding;
        }

        if (Math.abs(nextX - state.x) > 0.5 || Math.abs(nextY - state.y) > 0.5) {
            setState((prev) => ({
                ...prev,
                x: nextX,
                y: nextY,
            }));
        }
    }, [state.visible, state.anchorX, state.anchorY, state.x, state.y]);

    const isVisible = state.visible && Boolean(window.desktop);
    const isMac = useMemo(() => navigator.platform.toLowerCase().includes("mac"), []);

    const handleCommand = async (command: EditCommand) => {
        if (!window.desktop) {
            return;
        }
        await window.desktop.executeEditCommand(command);
        setState((prev) => ({ ...prev, visible: false }));
    };

    if (!isVisible) {
        return null;
    }

    return createPortal(
        <div className="pointer-events-none fixed inset-0 z-[9999]">
            <div
                className={cn(
                    "pointer-events-auto absolute min-w-[208px] rounded-2xl border p-2 text-xs shadow-2xl backdrop-blur",
                    "border-black/5 bg-white/95 text-slate-800 dark:border-white/10 dark:bg-slate-900/95 dark:text-slate-100",
                    "context-menu-surface"
                )}
                ref={menuRef}
                style={{ left: state.x, top: state.y }}
            >
                {MENU_ITEMS.map((item, index) => {
                    if (item === "separator") {
                        return (
                            <div
                                key={`separator-${index}`}
                                className="my-1 h-px bg-slate-200 dark:bg-white/10"
                            />
                        );
                    }
                    const disabled =
                        (item.requireEditability && !state.isEditable) ||
                        (item.requireSelection && !state.hasSelection);
                    return (
                        <button
                            key={item.key}
                            type="button"
                            disabled={disabled}
                            onClick={() => handleCommand(item.key)}
                            className={cn(
                                "flex w-full items-center justify-between rounded-xl px-3 py-2 text-left transition-colors",
                                disabled
                                    ? "cursor-not-allowed text-slate-400 dark:text-slate-500"
                                    : "hover:bg-slate-100 dark:hover:bg-slate-800/70"
                            )}
                        >
                            <span className="text-sm font-medium">
                                {t(`contextMenu.${item.key}` as const, {
                                    defaultValue: item.key,
                                })}
                            </span>
                            <span className="text-[11px] font-semibold tracking-wide text-slate-500 dark:text-slate-400">
                                {getShortcut(item.key, isMac)}
                            </span>
                        </button>
                    );
                })}
            </div>
        </div>,
        document.body,
    );
}
