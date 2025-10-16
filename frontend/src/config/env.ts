/*
 * Centralised environment-driven constants for tuning prompt behaviour.
 * Defaults match previous hardcoded values to avoid behaviour changes when env vars are absent.
 */

function readIntFromEnv(key: string, fallback: number): number {
    const raw = import.meta.env[key as keyof ImportMetaEnv];
    if (raw === undefined || raw === null || raw === "") {
        return fallback;
    }
    const parsed = Number.parseInt(String(raw), 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export const KEYWORD_ROW_LIMIT = readIntFromEnv("VITE_KEYWORD_ROW_LIMIT", 3);
export const DEFAULT_KEYWORD_WEIGHT = readIntFromEnv("VITE_DEFAULT_KEYWORD_WEIGHT", 5);
