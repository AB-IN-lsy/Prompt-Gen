import { Button, type ButtonProps } from "./button";
import { forwardRef, useEffect, useRef, type MutableRefObject } from "react";
import { cn } from "../../lib/utils";

const clamp = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), max);

export interface MagneticButtonProps extends ButtonProps {
  intensity?: number;
}

export const MagneticButton = forwardRef<HTMLButtonElement, MagneticButtonProps>(
  ({ className, children, intensity = 16, ...props }, ref) => {
    const innerRef = useRef<HTMLSpanElement>(null);
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
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        const offsetX = x - rect.width / 2;
        const offsetY = y - rect.height / 2;
        const translateX = clamp((offsetX / rect.width) * intensity, -intensity, intensity);
        const translateY = clamp((offsetY / rect.height) * intensity, -intensity, intensity);

        node.style.transform = `translate3d(${translateX}px, ${translateY}px, 0) rotate3d(1, -1, 0, ${(translateX + translateY) * 0.35}deg)`;
        node.style.boxShadow = `0 18px 35px -18px rgba(59, 130, 246, 0.45)`;

        if (innerRef.current) {
          innerRef.current.style.transform = `translate3d(${translateX * 0.4}px, ${translateY * 0.4}px, 0)`;
        }
      };

      const reset = () => {
        node.style.transform = "";
        node.style.boxShadow = "";
        if (innerRef.current) {
          innerRef.current.style.transform = "";
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
          "magnetic-button relative overflow-hidden will-change-transform transition-transform duration-200",
          className,
        )}
        {...props}
      >
        <span ref={innerRef} className="relative inline-flex items-center justify-center gap-2 transition-transform duration-200" data-magnetic-inner>
          {children}
        </span>
      </Button>
    );
  },
);

MagneticButton.displayName = "MagneticButton";
