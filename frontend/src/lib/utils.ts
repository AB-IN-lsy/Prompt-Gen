/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:52
 * @FilePath: \electron-go-app\frontend\src\lib\utils.ts
 * @LastEditTime: 2025-10-09 22:43:56
 */
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import { API_BASE_URL } from "./http";
import { isLocalMode } from "./runtimeMode";

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

const LOCAL_ASSET_ORIGIN_FALLBACK = "http://127.0.0.1:9090";

function deriveAssetOriginFromApiBase(apiBaseUrl: string): string {
  try {
    const parsed = new URL(apiBaseUrl);
    const segments = parsed.pathname.split("/").filter(Boolean);
    if (
      segments.length > 0 &&
      segments[segments.length - 1].toLowerCase() === "api"
    ) {
      segments.pop();
    }
    parsed.pathname = segments.length > 0 ? `/${segments.join("/")}` : "";
    parsed.search = "";
    parsed.hash = "";
    const normalised = parsed.toString();
    return normalised.endsWith("/") ? normalised.slice(0, -1) : normalised;
  } catch {
    return LOCAL_ASSET_ORIGIN_FALLBACK;
  }
}

function resolveAssetOrigin(): string {
  const derived = deriveAssetOriginFromApiBase(API_BASE_URL);
  if (isLocalMode()) {
    return derived || LOCAL_ASSET_ORIGIN_FALLBACK;
  }
  return derived;
}

export function resolveAssetUrl(path?: string | null): string | null {
  if (path == null) {
    return null;
  }
  const trimmed = path.trim();
  if (trimmed.length === 0) {
    return "";
  }
  if (
    /^[a-z][a-z0-9+\-.]*:\/\//i.test(trimmed) ||
    trimmed.startsWith("//") ||
    trimmed.startsWith("data:")
  ) {
    return trimmed;
  }
  try {
    const origin = resolveAssetOrigin();
    const base = origin.endsWith("/") ? origin : `${origin}/`;
    const target = trimmed.startsWith("/")
      ? new URL(trimmed, origin).toString()
      : new URL(trimmed, base).toString();
    return target.endsWith("/") && !trimmed.endsWith("/")
      ? target.slice(0, -1)
      : target;
  } catch {
    return trimmed;
  }
}
