import { useEffect, useState } from "react";
import { Minus, Maximize2, Minimize2, X } from "lucide-react";
import { cn } from "../../lib/utils";

type WindowState = {
    isMaximized: boolean;
    isAlwaysOnTop: boolean;
};

const defaultState: WindowState = {
    isMaximized: false,
    isAlwaysOnTop: false
};

export function TitleBar() {
    const [state, setState] = useState<WindowState>(defaultState);
    const [electronAvailable, setElectronAvailable] = useState(false);
    const [isMac, setIsMac] = useState(false);

    useEffect(() => {
        if (typeof window === "undefined") {
            return;
        }
        if (window.desktop) {
            setElectronAvailable(true);
        }
    }, []);

    useEffect(() => {
        if (typeof navigator !== "undefined") {
            const ua = navigator.userAgent ?? "";
            const platform = (navigator.platform ?? "").toLowerCase();
            if (/mac/i.test(ua) || platform.includes("mac")) {
                setIsMac(true);
            }
        }
    }, []);

    useEffect(() => {
        if (!electronAvailable || !window.desktop) {
            return () => undefined;
        }
        let disposed = false;

        window.desktop
            .getWindowState()
            .then((initial) => {
                if (!disposed && initial) {
                    setState(initial);
                }
            })
            .catch(() => undefined);

        const unsubscribe = window.desktop.onWindowState((next) => {
            setState(next ?? defaultState);
        });

        return () => {
            disposed = true;
            if (typeof unsubscribe === "function") {
                unsubscribe();
            }
        };
    }, [electronAvailable]);

    const toggleMaximize = () => {
        if (!electronAvailable || !window.desktop) {
            return;
        }
        window.desktop
            .toggleMaximize()
            .then((next) => next && setState(next))
            .catch(() => undefined);
    };

    const minimize = () => {
        if (!electronAvailable || !window.desktop) {
            return;
        }
        window.desktop
            .minimize()
            .catch(() => undefined);
    };

    const close = () => {
        if (!electronAvailable || !window.desktop) {
            return;
        }
        window.desktop
            .close()
            .catch(() => undefined);
    };

    const ControlIcon = state.isMaximized ? Minimize2 : Maximize2;
    const showWindowControls = electronAvailable && !isMac;

    return (
        <div
            className={cn(
                "drag-region relative z-40 flex h-10 w-full items-center justify-between bg-gradient-to-r from-slate-950 via-indigo-900 to-slate-800 px-4 text-[0.7rem] font-medium uppercase tracking-[0.35em] text-slate-200 shadow-md",
                isMac && "pl-16"
            )}
        >
            <div className="pointer-events-none flex items-center gap-3">
                <span className="h-2.5 w-2.5 rounded-full bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.6)]" />
                <span className="select-none">Prompt Gen Studio</span>
            </div>
            {electronAvailable ? (
                <div className="no-drag flex items-center gap-2 text-xs normal-case tracking-normal">
                    {showWindowControls && (
                        <>
                            <button
                                type="button"
                                aria-label="最小化"
                                className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-700/40 text-slate-200 transition hover:bg-slate-600/60 hover:text-white"
                                onClick={minimize}
                            >
                                <Minus className="h-3.5 w-3.5" />
                            </button>
                            <button
                                type="button"
                                aria-label={state.isMaximized ? "还原窗口" : "最大化"}
                                className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-700/40 text-slate-200 transition hover:bg-slate-600/60 hover:text-white"
                                onClick={toggleMaximize}
                            >
                                <ControlIcon className="h-3.5 w-3.5" />
                            </button>
                            <button
                                type="button"
                                aria-label="关闭"
                                className="flex h-8 w-8 items-center justify-center rounded-full bg-rose-500/80 text-white transition hover:bg-rose-600"
                                onClick={close}
                            >
                                <X className="h-3.5 w-3.5" />
                            </button>
                        </>
                    )}
                </div>
            ) : (
                <div className="pointer-events-none select-none text-[0.65rem] font-semibold uppercase tracking-[0.4em] text-slate-400">
                    Web Preview
                </div>
            )}
        </div>
    );
}
