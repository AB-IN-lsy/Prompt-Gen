/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:15:30
 * @FilePath: \electron-go-app\frontend\src\lib\tokenStorage.ts
 * @LastEditTime: 2025-10-09 23:23:08
 */
/*
 * @fileoverview Lightweight wrapper around `localStorage` for persisting auth tokens.
 *
 * The backend issues `access_token`, `refresh_token` and `expires_in` seconds.
 * We persist them locally so the Axios client can send the `Authorization` header
 * and attempt silent refreshes. A small skew is subtracted from the expiry timestamp
 * to minimise the probability of sending an already-expired access token.
 */

export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  /** Epoch milliseconds indicating when the access token should be considered expired. */
  accessTokenExpiresAt?: number;
}

const STORAGE_KEY = "promptgen.auth.tokens";
const EXPIRY_SKEW_MS = 5_000; // subtract 5 seconds to compensate for clock drift.

function now(): number {
  return Date.now();
}

/** Retrieves the token pair from localStorage, returning `null` if absent or malformed. */
export function getTokenPair(): TokenPair | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as TokenPair | undefined;
    if (!parsed || typeof parsed.accessToken !== "string" || typeof parsed.refreshToken !== "string") {
      return null;
    }
    return parsed;
  } catch (error) {
    console.warn("Failed to parse auth tokens from storage", error);
    return null;
  }
}

/**
 * Persists the issued tokens and their expiry time.
 * @param accessToken - The short-lived JWT used for resource requests.
 * @param refreshToken - The refresh token used to obtain a new access token.
 * @param expiresInSeconds - The access token lifetime.
 */
export function setTokenPair(accessToken: string, refreshToken: string, expiresInSeconds?: number): void {
  if (typeof window === "undefined") {
    return;
  }

  const expiresAt = expiresInSeconds ? now() + expiresInSeconds * 1000 - EXPIRY_SKEW_MS : undefined;
  const payload: TokenPair = { accessToken, refreshToken, accessTokenExpiresAt: expiresAt };
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
}

/** Clears the locally persisted tokens. */
export function clearTokenPair(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(STORAGE_KEY);
}

/**
 * Checks if the cached access token is considered expired (or about to expire).
 * This allows us to proactively refresh before sending a request when possible.
 */
export function isAccessTokenExpired(tokens: TokenPair | null): boolean {
  if (!tokens?.accessTokenExpiresAt) {
    return false;
  }
  return tokens.accessTokenExpiresAt <= now();
}
