import { useEffect, useState } from "react";
import { Minus, Maximize2, Minimize2, X, Pin, PinOff } from "lucide-react";
import { cn } from "../../lib/utils";

type WindowState = {
    isMaximized: boolean;
    isAlwaysOnTop: boolean;
};

const defaultState: WindowState = {
    isMaximized: false,
    isAlwaysOnTop: false
};

const noop = () => undefined;

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
            return noop;
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

    const toggleAlwaysOnTop = () => {
        if (!electronAvailable || !window.desktop) {
            return;
        }
        window.desktop
            .toggleAlwaysOnTop()
            .then((next) => next && setState(next))
            .catch(() => undefined);
    };

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
    const pinActive = state.isAlwaysOnTop;
    const showWindowControls = electronAvailable && !isMac;
    const showMacPin = electronAvailable && isMac;

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
                    {(showWindowControls || showMacPin) && (
                        <button
                            type="button"
                            aria-label={pinActive ? "取消置顶" : "窗口置顶"}
                            className={cn(
                                "flex h-7 w-24 items-center justify-center gap-1 rounded-full border border-slate-600/60 bg-slate-800/70 px-3 text-[0.7rem] font-semibold uppercase tracking-[0.2em] transition-colors",
                                pinActive
                                    ? "border-emerald-400/70 bg-emerald-500/20 text-emerald-300 hover:bg-emerald-500/30"
                                    : "text-slate-300 hover:border-slate-400/60 hover:bg-slate-700/70 hover:text-slate-100"
                            )}
                            onClick={toggleAlwaysOnTop}
                        >
                            {pinActive ? <PinOff className="h-3.5 w-3.5" /> : <Pin className="h-3.5 w-3.5" />}
                            <span>{pinActive ? "Pin Off" : "Pin"}</span>
                        </button>
                    )}
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
