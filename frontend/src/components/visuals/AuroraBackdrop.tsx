import { cn } from "../../lib/utils";

interface AuroraBackdropProps {
  className?: string;
}

/**
 * AuroraBackdrop creates a subtle animated gradient background inspired by Reactbits.
 * It relies purely on CSS animations so it stays lightweight while giving the shell more depth.
 */
export function AuroraBackdrop({ className }: AuroraBackdropProps) {
  return (
    <div
      className={cn(
        "pointer-events-none absolute inset-0 overflow-hidden",
        "bg-gradient-to-br from-[#f8fafc] via-white to-[#eef2ff] dark:from-[#020617] dark:via-[#0f172a] dark:to-[#0b1226]",
        className,
      )}
    >
      <div className="absolute inset-0 opacity-[0.15] mix-blend-soft-light">
        <div className="absolute -inset-[60%] bg-[radial-gradient(circle_at_center,rgba(148,163,255,0.4)_0%,transparent_60%)] blur-3xl" />
      </div>
      <div className="absolute -top-48 left-1/5 h-[34rem] w-[34rem] animate-aurora-sway rounded-full bg-gradient-to-br from-sky-400/35 via-indigo-500/25 to-purple-500/30 blur-3xl dark:from-sky-500/30 dark:via-indigo-500/25 dark:to-purple-500/30" />
      <div className="absolute -bottom-40 right-1/6 h-[32rem] w-[32rem] animate-aurora-sway-reverse rounded-full bg-gradient-to-br from-rose-400/30 via-orange-400/25 to-amber-400/30 blur-[110px] dark:from-rose-500/30 dark:via-orange-500/25 dark:to-amber-400/25" />
      <div className="absolute left-1/3 top-1/2 h-[26rem] w-[26rem] -translate-y-1/2 animate-aurora-pulse rounded-full bg-gradient-to-br from-emerald-300/25 via-teal-300/25 to-sky-300/25 blur-[120px] dark:from-emerald-400/25 dark:via-teal-400/20 dark:to-sky-400/25" />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(255,255,255,0.8),transparent_65%)] opacity-70 dark:bg-[radial-gradient(circle_at_top_right,rgba(148,163,255,0.06),transparent_70%)]" />
      <div className="absolute inset-0 bg-noise opacity-[0.035]" />
    </div>
  );
}

