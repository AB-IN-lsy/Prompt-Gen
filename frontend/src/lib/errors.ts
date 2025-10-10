/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:15:09
 * @FilePath: \electron-go-app\frontend\src\lib\errors.ts
 * @LastEditTime: 2025-10-09 23:15:13
 */
/*
 * @fileoverview Defines the unified API error types for the PromptGen frontend.
 *
 * The backend returns responses in the following envelope shape:
 *
 * ```json
 * {
 *   "success": false,
 *   "error": {
 *     "code": "BAD_REQUEST",
 *     "message": "Invalid request",
 *     "details": { "field": "topic" }
 *   }
 * }
 * ```
 *
 * This module normalises that structure into a TypeScript-friendly `ApiError` class so the
 * rendering layer can rely on consistent fields (`status`, `code`, `message`, `details`).
 */

export type ErrorCode =
  | "BAD_REQUEST"
  | "UNAUTHORIZED"
  | "FORBIDDEN"
  | "NOT_FOUND"
  | "CONFLICT"
  | "TOO_MANY_REQUESTS"
  | "INTERNAL_ERROR"
  | "CAPTCHA_INVALID"
  | "CAPTCHA_EXPIRED"
  | "CAPTCHA_REQUIRED"
  | "EMAIL_NOT_VERIFIED"
  | "EMAIL_ALREADY_VERIFIED"
  | "VERIFICATION_TOKEN_INVALID"
  | string;

/** Shape of the error payload sent by the backend. */
export interface ErrorPayload {
  code?: ErrorCode;
  message?: string;
  details?: unknown;
}

export interface ApiErrorInit {
  status?: number;
  code?: ErrorCode;
  message?: string;
  details?: unknown;
  /**
   * Optional original error. When present, it will be assigned to the `cause` field
   * of the resulting `ApiError` instance so callers can access the low-level error
   * (e.g. AxiosError) for logging or diagnostics.
   */
  cause?: unknown;
}

/**
 * ApiError wraps the server error payload with additional HTTP context so components can
 * branch on either the status code or the domain-specific error code.
 */
export class ApiError extends Error {
  readonly status?: number;
  readonly code?: ErrorCode;
  readonly details?: unknown;
  readonly cause?: unknown;

  constructor(init: ApiErrorInit) {
    super(init.message ?? "Unexpected API error");
    this.name = "ApiError";
    this.status = init.status;
    this.code = init.code;
    this.details = init.details;
    if (init.cause !== undefined) {
      this.cause = init.cause;
    }
  }

  /** Returns true when the error represents an authentication failure (401). */
  get isUnauthorized(): boolean {
    return this.status === 401 || this.code === "UNAUTHORIZED";
  }

  /** Returns true when the error represents a client-side validation issue (400). */
  get isBadRequest(): boolean {
    return this.status === 400 || this.code === "BAD_REQUEST";
  }

  /** Returns true when the error indicates throttling. */
  get isRateLimited(): boolean {
    return this.status === 429 || this.code === "TOO_MANY_REQUESTS";
  }
}

/** Convenience helper to detect ApiError instances via duck-typing. */
export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError || (typeof error === "object" && error !== null && (error as any).name === "ApiError");
}
