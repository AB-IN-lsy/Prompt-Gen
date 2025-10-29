import { Button } from "./button";
import { cn } from "../../lib/utils";

interface PaginationControlsProps {
  page: number;
  totalPages: number;
  currentCount: number;
  onPrev: () => void;
  onNext: () => void;
  prevLabel: string;
  nextLabel: string;
  pageLabel: string;
  countLabel: string;
  className?: string;
}

export function PaginationControls({
  page,
  totalPages,
  currentCount,
  onPrev,
  onNext,
  prevLabel,
  nextLabel,
  pageLabel,
  countLabel,
  className,
}: PaginationControlsProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-2xl border border-white/60 bg-white/80 px-4 py-3 text-sm text-slate-500 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-400 md:flex-row md:items-center md:justify-between",
        className,
      )}
    >
      <div className="flex flex-col gap-1 text-xs md:flex-row md:items-center md:gap-4 md:text-sm">
        <span>{pageLabel}</span>
        <span>{countLabel}</span>
      </div>
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={onPrev}
          disabled={page <= 1}
        >
          {prevLabel}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={onNext}
          disabled={page >= totalPages}
        >
          {nextLabel}
        </Button>
      </div>
    </div>
  );
}
