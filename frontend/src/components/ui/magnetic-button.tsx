import { forwardRef, useEffect, useRef, type MutableRefObject } from "react";
import { Button, type ButtonProps } from "./button";
import { cn } from "../../lib/utils";

const clamp = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), max);

export interface MagneticButtonProps extends ButtonProps {
  intensity?: number;
}

export const MagneticButton = forwardRef<HTMLButtonElement, MagneticButtonProps>(
  ({ className, children, intensity = 18, ...props }, ref) => {
    const innerRef = useRef<HTMLSpanElement>(null);
    const glowRef = useRef<HTMLSpanElement>(null);
    const localRef = useRef<HTMLButtonElement | null>(null);

    useEffect(() => {
      const node = localRef.current;
      if (!node) {
        return;
      }

      const handlePointerMove = (event: PointerEvent) => {
        if (node.disabled) {
          return;
        }
        const rect = node.getBoundingClientRect();
        const offsetX = event.clientX - rect.left - rect.width / 2;
        const offsetY = event.clientY - rect.top - rect.height / 2;
        const translateX = clamp((offsetX / rect.width) * intensity, -intensity, intensity);
        const translateY = clamp((offsetY / rect.height) * intensity, -intensity, intensity);

        node.style.transform = `translate3d(${translateX}px, ${translateY}px, 0)`;
        node.style.boxShadow = `0 25px 45px -24px rgba(59, 130, 246, 0.55)`;

        if (innerRef.current) {
          innerRef.current.style.transform = `translate3d(${translateX * 0.35}px, ${translateY * 0.35}px, 0)`;
        }
        if (glowRef.current) {
          const glowX = ((offsetX / rect.width) * 100).toFixed(2);
          const glowY = ((offsetY / rect.height) * 100).toFixed(2);
          glowRef.current.style.opacity = "0.85";
          glowRef.current.style.transform = `translate3d(${translateX * 0.25}px, ${translateY * 0.25}px, 0)`;
          glowRef.current.style.background = `radial-gradient(120px circle at ${50 + Number(glowX) * 0.6}% ${50 + Number(glowY) * 0.6}%, rgba(255,255,255,0.65), rgba(59,130,246,0.15) 45%, transparent 70%)`;
        }
      };

      const reset = () => {
        node.style.transform = "";
        node.style.boxShadow = "";
        if (innerRef.current) {
          innerRef.current.style.transform = "";
        }
        if (glowRef.current) {
          glowRef.current.style.opacity = "0";
          glowRef.current.style.transform = "";
        }
      };

      node.addEventListener("pointermove", handlePointerMove);
      node.addEventListener("pointerleave", reset);
      node.addEventListener("pointerup", reset);

      return () => {
        node.removeEventListener("pointermove", handlePointerMove);
        node.removeEventListener("pointerleave", reset);
        node.removeEventListener("pointerup", reset);
      };
    }, [intensity]);

    return (
      <Button
        ref={(node) => {
          localRef.current = node;
          if (typeof ref === "function") {
            ref(node);
          } else if (ref) {
            (ref as MutableRefObject<HTMLButtonElement | null>).current = node;
          }
        }}
        className={cn(
          "magnetic-button relative overflow-hidden rounded-2xl px-6 py-2 text-base font-semibold text-white",
          "bg-[radial-gradient(circle_at_20%_20%,rgba(255,255,255,0.3),rgba(59,130,246,0.6))]",
          "backdrop-blur-sm shadow-[0_20px_50px_-24px_rgba(59,130,246,0.55)] transition-all duration-200",
          "border border-white/20 dark:border-white/10",
          className,
        )}
        {...props}
      >
        <span
          ref={glowRef}
          className="pointer-events-none absolute inset-[-20%] rounded-[28px] opacity-0 transition-opacity duration-200"
          aria-hidden="true"
        />
        <span
          ref={innerRef}
          className="relative z-10 inline-flex items-center justify-center gap-2 transition-transform duration-200"
          data-magnetic-inner
        >
          {children}
        </span>
      </Button>
    );
  },
);

MagneticButton.displayName = "MagneticButton";
