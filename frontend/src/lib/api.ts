/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:44:21
 * @FilePath: \electron-go-app\frontend\src\lib\api.ts
 * @LastEditTime: 2025-10-09 23:53:15
 */
import type { AxiosResponse } from "axios";
import { ApiError } from "./errors";
import { http, normaliseError } from "./http";

export type KeywordPolarity = "positive" | "negative";
export type KeywordSource = "local" | "api" | "manual";

/**
 * Keyword entity as returned by the backend. The Prompt Workbench primarily uses the
 * `id`, `word`, `polarity`, `source`, and `weight` fields.
 */
export interface Keyword {
  id: string;
  word: string;
  polarity: KeywordPolarity;
  source: KeywordSource;
  weight: number;
  topic?: string;
  language?: string;
  updated_at?: string;
}

export interface KeywordFilters {
  topic?: string;
  polarity?: KeywordPolarity;
  source?: KeywordSource;
  search?: string;
}

export interface KeywordCreateInput {
  word: string;
  polarity: KeywordPolarity;
  source?: Exclude<KeywordSource, "local">;
  topic?: string;
  weight?: number;
  language?: string;
}

export interface KeywordUpdateInput {
  word?: string;
  weight?: number;
  polarity?: KeywordPolarity;
  topic?: string;
  language?: string;
}

export interface KeywordSuggestion {
  word: string;
  polarity: KeywordPolarity;
  source: "ai" | "manual" | "import";
  confidence?: number;
}

export interface PromptKeywordRef {
  keywordId?: string;
  word: string;
  weight?: number;
}

export interface PromptPayload {
  topic: string;
  prompt: string;
  positiveKeywords: PromptKeywordRef[];
  negativeKeywords: PromptKeywordRef[];
  model: string;
  tags?: string[];
  status?: "draft" | "published" | "archived";
}

export interface PromptRegenerateResponse {
  prompt: string;
  metadata?: {
    model: string;
    latency_ms?: number;
  };
}

export interface PromptSummary {
  id: string;
  topic: string;
  title: string;
  status: "draft" | "published" | "archived";
  model: string;
  tags?: string[];
  updated_at: string;
}

export interface PaginatedPromptsResponse {
  items: PromptSummary[];
  total: number;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface AuthUser {
  id: number;
  username: string;
  email: string;
  avatar_url?: string | null;
  email_verified_at?: string | null;
  last_login_at?: string | null;
  created_at?: string;
  updated_at?: string;
}

export interface UserSettings {
  preferred_model: string;
  sync_enabled: boolean;
}

export interface AuthProfile {
  user: AuthUser;
  settings: UserSettings;
}

export interface AuthResponse {
  user: AuthUser;
  tokens: AuthTokens;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest extends LoginRequest {
  username: string;
  captcha_id?: string;
  captcha_code?: string;
  avatar_url?: string;
}

export interface UpdateCurrentUserRequest {
  username?: string;
  email?: string;
  avatar_url?: string | null;
  preferred_model?: string;
  sync_enabled?: boolean;
}

export interface EmailVerificationRequestResult {
  issued: boolean;
  token?: string;
  remainingAttempts?: number;
}

export interface CaptchaResponse {
  captcha_id: string;
  image: string;
}

/**
 * Fetches keywords from the backend. The backend supports optional topic/polarity filters.
 * Errors are normalised to {@link ApiError} before being rethrown.
 */
export async function fetchKeywords(filters: KeywordFilters = {}): Promise<Keyword[]> {
  try {
    const response: AxiosResponse<Keyword[]> = await http.get("/keywords", { params: filters });
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Generates fresh keyword suggestions using the LLM-powered backend helper. */
export async function generateKeywordSuggestions(payload: {
  topic: string;
  positiveSeeds?: string[];
  negativeSeeds?: string[];
  model: string;
}): Promise<KeywordSuggestion[]> {
  try {
    const response: AxiosResponse<{ suggestions: KeywordSuggestion[] }> = await http.post("/keywords/generate", payload);
    return response.data.suggestions;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Creates a new keyword and returns the persisted entity. */
export async function createKeyword(input: KeywordCreateInput): Promise<Keyword> {
  try {
    const response: AxiosResponse<Keyword> = await http.post("/keywords", input);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Updates an existing keyword (identified by `id`). */
export async function updateKeyword(id: string, input: KeywordUpdateInput): Promise<Keyword> {
  if (!id) {
    throw new ApiError({ message: "Keyword id is required" });
  }
  try {
    const response: AxiosResponse<Keyword> = await http.patch(`/keywords/${id}`, input);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Soft deletes a keyword. */
export async function deleteKeyword(id: string): Promise<void> {
  if (!id) {
    throw new ApiError({ message: "Keyword id is required" });
  }
  try {
    await http.delete(`/keywords/${id}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/**
 * Calls the prompt regeneration endpoint to produce a refined prompt using the selected
 * keywords and chosen model.
 */
export async function regeneratePrompt(payload: PromptPayload): Promise<PromptRegenerateResponse> {
  try {
    const response: AxiosResponse<PromptRegenerateResponse> = await http.post("/prompts/regenerate", payload);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Persists a prompt payload as a draft. */
export async function saveDraft(payload: PromptPayload): Promise<void> {
  try {
    await http.post("/prompts", { ...payload, status: payload.status ?? "draft" });
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Fetches a paginated prompt list for the “我的 Prompt”页面. */
export async function fetchPrompts(params: {
  status?: "draft" | "published" | "archived";
  tags?: string[];
  model?: string;
  search?: string;
  page?: number;
  size?: number;
} = {}): Promise<{ items: PromptSummary[]; total: number; meta?: unknown }> {
  try {
    const response: AxiosResponse<PaginatedPromptsResponse> & { meta?: unknown } = await http.get("/prompts", { params });
    return { items: response.data.items, total: response.data.total, meta: (response as any).meta };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Fetches a single prompt by ID, including its versions when supported by the backend. */
export async function fetchPromptById(id: string): Promise<any> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    const response = await http.get(`/prompts/${id}`);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Updates a prompt in place. */
export async function updatePrompt(id: string, payload: Partial<PromptPayload>): Promise<void> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    await http.patch(`/prompts/${id}`, payload);
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Publishes a prompt (transitioning it from draft to published). */
export async function publishPrompt(id: string): Promise<void> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    await http.post(`/prompts/${id}/publish`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 提交登录请求，返回用户信息与令牌。 */
export async function login(payload: LoginRequest): Promise<AuthResponse> {
  try {
    const response: AxiosResponse<AuthResponse> = await http.post("/auth/login", payload);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 注册新用户，成功后立即返回用户信息与令牌。 */
export async function register(payload: RegisterRequest): Promise<AuthResponse> {
  try {
    const response: AxiosResponse<AuthResponse> = await http.post("/auth/register", payload);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 上传头像文件并返回可公开访问的 URL。 */
export async function uploadAvatar(file: File): Promise<string> {
  const formData = new FormData();
  formData.append("avatar", file);

  try {
    const response: AxiosResponse<{ avatar_url: string }> = await http.post("/uploads/avatar", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return response.data.avatar_url;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 刷新 access token，通常由 http 拦截器内部调用。 */
export async function refreshTokens(refreshToken: string): Promise<AuthTokens> {
  try {
    const response: AxiosResponse<{ tokens: AuthTokens }> = await http.post("/auth/refresh", {
      refresh_token: refreshToken,
    });
    return response.data.tokens;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 注销当前刷新令牌。 */
export async function logout(refreshToken: string): Promise<void> {
  try {
    await http.post("/auth/logout", { refresh_token: refreshToken });
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 请求邮箱验证令牌，开发环境会直接返回 token 便于测试。 */
export async function requestEmailVerification(email: string): Promise<EmailVerificationRequestResult> {
  try {
    const response: AxiosResponse<{ issued: boolean; token?: string; remaining_attempts?: number }> = await http.post(
      "/auth/verify-email/request",
      {
        email,
      }
    );
    const data = response.data ?? {};
    return {
      issued: Boolean(data.issued),
      token: data.token,
      remainingAttempts:
        typeof data.remaining_attempts === "number" && Number.isFinite(data.remaining_attempts) ? data.remaining_attempts : undefined,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 使用邮件中的 token 完成邮箱验证。 */
export async function confirmEmailVerification(token: string): Promise<void> {
  try {
    await http.post("/auth/verify-email/confirm", { token });
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 获取当前登录用户资料及设置。 */
export async function fetchCurrentUser(): Promise<AuthProfile> {
  try {
    const response: AxiosResponse<AuthProfile> = await http.get("/users/me");
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 更新当前登录用户的基础资料和设置。 */
export async function updateCurrentUser(payload: UpdateCurrentUserRequest): Promise<AuthProfile> {
  try {
    const response: AxiosResponse<AuthProfile> = await http.put("/users/me", payload);
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 创建图形验证码并返回 base64 图片与标识。 */
export async function fetchCaptcha(): Promise<CaptchaResponse> {
  try {
    const response: AxiosResponse<CaptchaResponse> = await http.get("/auth/captcha");
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/**
 * Exposes helper utilities so other modules (e.g. login form) can set or clear the token
 * pair returned by the backend authentication endpoints.
 */
export { clearTokenPair as clearAuthTokens, getTokenPair as getAuthTokens, setTokenPair as setAuthTokens } from "./tokenStorage";
