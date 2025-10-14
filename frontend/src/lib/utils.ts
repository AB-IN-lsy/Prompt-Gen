/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:52
 * @FilePath: \electron-go-app\frontend\src\lib\utils.ts
 * @LastEditTime: 2025-10-09 22:43:56
 */
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function clampTextWithOverflow(text: string, maxLength: number) {
  const trimmed = text.trim();
  if (maxLength <= 0) {
    const length = Array.from(trimmed).length;
    return { value: trimmed, overflow: 0, originalLength: length };
  }
  const runes = Array.from(trimmed);
  if (runes.length > maxLength) {
    return {
      value: runes.slice(0, maxLength).join(""),
      overflow: runes.length - maxLength,
      originalLength: runes.length,
    };
  }
  return { value: trimmed, overflow: 0, originalLength: runes.length };
}

export function formatOverflowLabel(value: string, overflow: number) {
  if (!overflow || overflow <= 0) {
    return value;
  }
  return `${value}…(+${overflow})`;
}
