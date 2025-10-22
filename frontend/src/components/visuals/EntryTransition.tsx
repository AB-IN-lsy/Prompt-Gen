import { useTranslation } from "react-i18next";
import { cn } from "../../lib/utils";

interface EntryTransitionProps {
  active: boolean;
  visible: boolean;
}

/**
 * EntryTransition renders a short lived overlay when进入工作台或本地模式，带来轻盈的入场动画。
 */
export function EntryTransition({ active, visible }: EntryTransitionProps) {
  const { t } = useTranslation();
  if (!active) {
    return null;
  }
  return (
    <div
      className={cn(
        "fixed inset-0 z-[120] flex items-center justify-center overflow-hidden transition-opacity duration-600 ease-out",
        visible ? "pointer-events-auto opacity-100" : "pointer-events-none opacity-0",
        "bg-slate-900/65 backdrop-blur-[20px]",
      )}
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(99,102,241,0.6),transparent_60%)] opacity-60 animate-entry-glow" />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_bottom,rgba(20,184,166,0.35),transparent_65%)] opacity-50 animate-entry-glow [animation-delay:400ms]" />
      <div className="absolute left-1/2 top-1/2 h-[32rem] w-[32rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-gradient-to-tr from-indigo-500/40 via-sky-400/30 to-emerald-400/30 blur-3xl animate-entry-trace" />
      <div className="absolute left-1/2 top-1/2 h-[20rem] w-[20rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-gradient-to-tr from-white/15 via-white/8 to-transparent blur-2xl animate-entry-ripple" />
      <div className="relative z-10 flex flex-col items-center gap-3 text-center text-white">
        <div className="relative flex h-16 w-16 items-center justify-center overflow-hidden rounded-full bg-white/10 backdrop-blur-lg shadow-[0_0_40px_rgba(99,102,241,0.55)]">
          <span className="text-2xl font-semibold tracking-wider text-white/80 animate-pulse">⚡</span>
        </div>
        <p className="text-xs uppercase tracking-[0.4em] text-white/60">
          {t("app.entryTransition.welcome", "欢迎回来")}
        </p>
        <p className="text-sm font-medium text-white/80">
          {t("app.entryTransition.ready", "正在唤醒工作台…")}
        </p>
      </div>
    </div>
  );
}

