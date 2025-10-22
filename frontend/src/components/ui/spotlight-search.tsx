import {
  forwardRef,
  useCallback,
  useRef,
  useState,
  type CSSProperties,
  type InputHTMLAttributes,
  type MutableRefObject,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { Search } from "lucide-react";
import { cn } from "../../lib/utils";

export interface SpotlightSearchProps
  extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

export const SpotlightSearch = forwardRef<HTMLInputElement, SpotlightSearchProps>(
  ({ className, label, onFocus, onBlur, ...props }, ref) => {
    const wrapperRef = useRef<HTMLLabelElement | null>(null);
    const [isFocused, setIsFocused] = useState(false);

    const setPointer = useCallback(
      (event: ReactPointerEvent<HTMLLabelElement>) => {
        const node = wrapperRef.current;
        if (!node) return;
        const rect = node.getBoundingClientRect();
        const x = ((event.clientX - rect.left) / rect.width) * 100;
        const y = ((event.clientY - rect.top) / rect.height) * 100;
        node.style.setProperty("--spotlight-x", `${x}%`);
        node.style.setProperty("--spotlight-y", `${y}%`);
      },
      [],
    );

    const resetPointer = useCallback(() => {
      const node = wrapperRef.current;
      if (!node) return;
      node.style.setProperty("--spotlight-x", "50%");
      node.style.setProperty("--spotlight-y", "50%");
    }, []);

    const handleFocus = useCallback(
      (event: React.FocusEvent<HTMLInputElement>) => {
        setIsFocused(true);
        onFocus?.(event);
      },
      [onFocus],
    );

    const handleBlur = useCallback(
      (event: React.FocusEvent<HTMLInputElement>) => {
        setIsFocused(false);
        onBlur?.(event);
      },
      [onBlur],
    );

    const handlePointerMove = useCallback(
      (event: ReactPointerEvent<HTMLLabelElement>) => {
        setPointer(event);
      },
      [setPointer],
    );

    const handlePointerLeave = useCallback(() => {
      resetPointer();
    }, [resetPointer]);

    return (
      <label
        ref={(node) => {
          wrapperRef.current = node;
          if (wrapperRef.current) {
            wrapperRef.current.style.setProperty("--spotlight-x", "50%");
            wrapperRef.current.style.setProperty("--spotlight-y", "50%");
          }
        }}
        className={cn(
          "group relative isolate flex min-h-[56px] items-center overflow-hidden rounded-2xl border border-white/50 bg-white/70 px-4 shadow-lg transition duration-300 ease-out backdrop-blur-xl focus-within:border-primary/60 dark:border-slate-800/60 dark:bg-slate-900/60 dark:focus-within:border-primary/40",
          "before:pointer-events-none before:absolute before:-inset-20 before:-z-10 before:opacity-0 before:transition-opacity before:duration-300 before:content-['']",
          "focus-within:shadow-[0_25px_45px_-20px_rgba(59,130,246,0.55)] dark:focus-within:shadow-[0_20px_40px_-20px_rgba(56,189,248,0.35)]",
          className,
        )}
        style={
          {
            "--spotlight-x": "50%",
            "--spotlight-y": "50%",
            ...(isFocused && {
              background:
                "linear-gradient(135deg, rgba(59,130,246,0.18), rgba(99,102,241,0.12))",
            }),
          } as CSSProperties
        }
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
        data-spotlight={isFocused ? "active" : "idle"}
      >
        <div
          className={cn(
            "pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300",
            "group-data-[spotlight=active]:opacity-100",
          )}
          style={{
            background:
              "radial-gradient(180px circle at var(--spotlight-x) var(--spotlight-y), rgba(59,130,246,0.28), rgba(14,165,233,0.1), transparent 65%)",
          }}
        />
        <Search className="mr-3 h-5 w-5 text-slate-400 transition-colors group-focus-within:text-primary dark:text-slate-500" />
        <input
          ref={(node) => {
            if (typeof ref === "function") {
              ref(node);
            } else if (ref) {
              (ref as MutableRefObject<HTMLInputElement | null>).current = node;
            }
          }}
          className="peer relative z-10 h-12 flex-1 bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400 focus:outline-none dark:text-slate-200 dark:placeholder:text-slate-500"
          {...props}
          onFocus={handleFocus}
          onBlur={handleBlur}
        />
        {label ? (
          <span className="pointer-events-none ml-3 select-none rounded-full border border-white/60 bg-white/70 px-3 py-1 text-xs font-medium text-slate-500 shadow-sm transition-opacity dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-400">
            {label}
          </span>
        ) : null}
      </label>
    );
  },
);

SpotlightSearch.displayName = "SpotlightSearch";
