const parseNumber = (value: string | undefined, fallback: number): number => {
  if (value == null) {
    return fallback;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
};

const parseEase = (
  value: string | undefined,
  fallback: [number, number, number, number],
): [number, number, number, number] => {
  if (!value || value.trim().length === 0) {
    return fallback;
  }
  const parts = value.split(",").map((part) => Number(part.trim()));
  if (parts.length !== 4 || parts.some((part) => !Number.isFinite(part))) {
    return fallback;
  }
  return parts as [number, number, number, number];
};

export const CARD_ANIMATION_DURATION = parseNumber(
  import.meta.env.VITE_CARD_ANIMATION_DURATION,
  0.35,
);

export const CARD_ANIMATION_OFFSET = parseNumber(
  import.meta.env.VITE_CARD_ANIMATION_OFFSET,
  24,
);

export const CARD_ANIMATION_STAGGER = parseNumber(
  import.meta.env.VITE_CARD_ANIMATION_STAGGER,
  0.05,
);

export const CARD_ANIMATION_EASE = parseEase(
  import.meta.env.VITE_CARD_ANIMATION_EASE,
  [0.33, 1, 0.68, 1],
);

export interface CardMotionOptions {
  index?: number;
  offset?: number;
}

export const buildCardMotion = ({ index = 0, offset }: CardMotionOptions = {}) => {
  const resolvedOffset = offset ?? CARD_ANIMATION_OFFSET;
  const useTranslate = Math.abs(resolvedOffset) > 0;

  const initial: Record<string, number> = { opacity: 0 };
  if (useTranslate) {
    initial.y = resolvedOffset;
  }

  const animate: Record<string, number> = { opacity: 1 };
  if (useTranslate) {
    animate.y = 0;
  }

  const exit: Record<string, number> = { opacity: 0 };
  if (useTranslate) {
    exit.y = -resolvedOffset;
  }

  return {
    initial,
    animate,
    exit,
    transition: {
      duration: CARD_ANIMATION_DURATION,
      delay: index * CARD_ANIMATION_STAGGER,
      ease: CARD_ANIMATION_EASE,
    },
  };
};
