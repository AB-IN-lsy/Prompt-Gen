/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:44:21
 * @FilePath: \electron-go-app\frontend\src\lib\api.ts
 * @LastEditTime: 2025-10-09 23:53:15
 */
import type { AxiosResponse } from "axios";
import { PROMPT_KEYWORD_MAX_LENGTH } from "../config/prompt";
import { ApiError } from "./errors";
import { http, normaliseError } from "./http";
import { clampTextWithOverflow } from "./utils";
import { DEFAULT_KEYWORD_WEIGHT } from "../config/prompt";

export type KeywordPolarity = "positive" | "negative";
export type KeywordSource = "local" | "api" | "manual" | "model";

export function normaliseKeywordSource(
  value?: string | null,
): KeywordSource {
  if (!value) {
    return "manual";
  }
  const lower = value.toLowerCase();
  if (lower === "model") {
    return "model";
  }
  if (lower === "local") {
    return "local";
  }
  if (lower === "api") {
    return "api";
  }
  if (lower === "manual") {
    return "manual";
  }
  return "manual";
}

const MIN_KEYWORD_WEIGHT = 0;
const MAX_KEYWORD_WEIGHT = DEFAULT_KEYWORD_WEIGHT;

const clampKeywordWeight = (value?: number): number => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return DEFAULT_KEYWORD_WEIGHT;
  }
  if (value < MIN_KEYWORD_WEIGHT) {
    return MIN_KEYWORD_WEIGHT;
  }
  if (value > MAX_KEYWORD_WEIGHT) {
    return MAX_KEYWORD_WEIGHT;
  }
  return Math.round(value);
};

/**
 * Keyword entity as returned by the backend. The Prompt Workbench primarily uses the
 * `id`, `word`, `polarity`, `source`, and `weight` fields.
 */
export interface Keyword {
  id: string;
  keywordId?: number;
  word: string;
  polarity: KeywordPolarity;
  source: KeywordSource;
  weight: number;
  topic?: string;
  language?: string;
  updated_at?: string;
  overflow?: number;
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

export interface PromptListKeyword {
  keyword_id?: number;
  word: string;
  source?: string;
  polarity?: KeywordPolarity;
  weight?: number;
}

export interface PromptListItem {
  id: number;
  topic: string;
  model: string;
  status: "draft" | "published" | "archived";
  tags: string[];
  positive_keywords: PromptListKeyword[];
  negative_keywords: PromptListKeyword[];
  updated_at: string;
  published_at?: string | null;
}

export interface PromptListMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface PromptListResponse {
  items: PromptListItem[];
  meta: PromptListMeta;
}

export interface PromptExportResult {
  filePath: string;
  promptCount: number;
  generatedAt: string;
}

export interface PromptImportError {
  topic: string;
  reason: string;
}

export interface PromptImportResult {
  importedCount: number;
  skippedCount: number;
  errors: PromptImportError[];
}

export interface PromptVersionSummary {
  versionNo: number;
  model: string;
  createdAt: string;
}

export interface PromptVersionDetail {
  versionNo: number;
  model: string;
  body: string;
  instructions?: string | null;
  positive_keywords: PromptListKeyword[];
  negative_keywords: PromptListKeyword[];
  created_at: string;
}

export interface PromptDetailResponse {
  id: number;
  topic: string;
  body: string;
  instructions?: string | null;
  model: string;
  status: "draft" | "published" | "archived";
  tags: string[];
  positive_keywords: PromptListKeyword[];
  negative_keywords: PromptListKeyword[];
  workspace_token?: string;
  created_at: string;
  updated_at: string;
  published_at?: string | null;
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
  is_admin: boolean;
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

// 模型凭据状态（默认仅启用/禁用，也保留字符串向后兼容）
export type ModelStatus = "enabled" | "disabled" | string;

// 用户保存的模型凭据结构体，后端会脱敏返回
export interface UserModelCredential {
  id: number;
  provider: string;
  model_key: string;
  display_name: string;
  base_url?: string;
  extra_config: Record<string, unknown>;
  status: ModelStatus;
  last_verified_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ChatCompletionMessage {
  role: string;
  content: string;
}

export interface ChatCompletionChoice {
  index: number;
  message: ChatCompletionMessage;
  finish_reason?: string;
  logprobs?: unknown;
}

export interface ChatCompletionUsage {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  [key: string]: unknown;
}

export interface ChatCompletionResponse {
  id: string;
  object: string;
  created: number;
  model: string;
  choices: ChatCompletionChoice[];
  usage?: ChatCompletionUsage;
}

// 新增模型凭据时的入参
export interface CreateUserModelRequest {
  provider: string;
  model_key: string;
  display_name: string;
  base_url?: string;
  api_key: string;
  extra_config?: Record<string, unknown>;
}

// 更新模型凭据支持的字段（包括启用/禁用）
export interface UpdateUserModelRequest {
  display_name?: string;
  base_url?: string | null;
  api_key?: string;
  extra_config?: Record<string, unknown>;
  status?: ModelStatus;
}

export interface TestUserModelRequest {
  model?: string;
  prompt?: string;
  messages?: ChatCompletionMessage[];
}

export interface AuthResponse {
  user: AuthUser;
  tokens: AuthTokens;
}

export interface PromptKeywordInput {
  keyword_id?: number | string;
  word: string;
  polarity: KeywordPolarity;
  source?: string;
  weight?: number;
}

export interface PromptKeywordResult {
  keyword_id?: number;
  word: string;
  polarity: KeywordPolarity;
  source?: string;
  weight?: number;
}

export interface InterpretPromptResponse {
  topic: string;
  confidence: number;
  positive_keywords: PromptKeywordResult[];
  negative_keywords: PromptKeywordResult[];
  workspace_token?: string;
  instructions?: string;
}

export interface AugmentPromptKeywordsRequest {
  topic: string;
  model_key: string;
  language?: string;
  positive_limit?: number;
  negative_limit?: number;
  existing_positive: PromptKeywordInput[];
  existing_negative: PromptKeywordInput[];
  workspace_token?: string;
}

export interface AugmentPromptKeywordsResponse {
  positive: PromptKeywordResult[];
  negative: PromptKeywordResult[];
}

export interface ManualPromptKeywordRequest {
  topic: string;
  word: string;
  polarity: KeywordPolarity;
  workspace_token?: string;
  prompt_id?: number;
  language?: string;
  weight?: number;
}

export interface RemovePromptKeywordRequest {
  word: string;
  polarity: KeywordPolarity;
  workspace_token?: string | null;
}

export interface GeneratePromptRequest {
  topic: string;
  model_key: string;
  positive_keywords: PromptKeywordInput[];
  negative_keywords: PromptKeywordInput[];
  prompt_id?: number;
  language?: string;
  instructions?: string;
  tone?: string;
  temperature?: number;
  max_tokens?: number;
  include_keyword_reference?: boolean;
  workspace_token?: string;
}

export interface GeneratePromptResponse {
  prompt: string;
  model: string;
  duration_ms?: number;
  usage?: ChatCompletionUsage;
  positive_keywords?: PromptKeywordResult[];
  negative_keywords?: PromptKeywordResult[];
  workspace_token?: string;
}

export interface SavePromptRequest {
  prompt_id?: number;
  topic?: string;
  body?: string;
  model?: string;
  instructions?: string;
  publish?: boolean;
  status?: string;
  tags?: string[];
  positive_keywords: PromptKeywordInput[];
  negative_keywords: PromptKeywordInput[];
  workspace_token?: string;
}

export interface SavePromptResponse {
  prompt_id: number;
  status: string;
  version: number;
  task_id?: string;
  workspace_token?: string;
}

export interface LoginRequest {
  identifier: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
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

export interface ChangelogEntry {
  id: number;
  locale: string;
  badge: string;
  title: string;
  summary: string;
  items: string[];
  published_at: string;
  author_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface ChangelogPayload {
  locale: string;
  badge: string;
  title: string;
  summary: string;
  items: string[];
  published_at: string;
  translate_to?: string[];
  translation_model_key?: string;
}

export interface ChangelogCreateResult {
  entry: ChangelogEntry;
  translations: ChangelogEntry[];
}

/** 描述 IP Guard 黑名单中一条记录的结构。 */
export interface IpGuardEntry {
  ip: string;
  ttl_seconds: number;
  expires_at?: string | null;
}

/**
 * Fetches keywords from the backend. The backend supports optional topic/polarity filters.
 * Errors are normalised to {@link ApiError} before being rethrown.
 */
export async function fetchKeywords(
  filters: KeywordFilters = {},
): Promise<Keyword[]> {
  try {
    const response: AxiosResponse<Keyword[]> = await http.get("/keywords", {
      params: filters,
    });
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
    const response: AxiosResponse<{ suggestions: KeywordSuggestion[] }> =
      await http.post("/keywords/generate", payload);
    return response.data.suggestions;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Creates a new keyword and returns the persisted entity. */
export async function createKeyword(
  input: KeywordCreateInput,
): Promise<Keyword> {
  try {
    const response: AxiosResponse<Keyword> = await http.post(
      "/keywords",
      input,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** Updates an existing keyword (identified by `id`). */
export async function updateKeyword(
  id: string,
  input: KeywordUpdateInput,
): Promise<Keyword> {
  if (!id) {
    throw new ApiError({ message: "Keyword id is required" });
  }
  try {
    const response: AxiosResponse<Keyword> = await http.patch(
      `/keywords/${id}`,
      input,
    );
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

const normalisePromptKeyword = (keyword: PromptKeywordInput) => {
  const { keyword_id, weight, ...rest } = keyword;
  const { value } = clampTextWithOverflow(
    rest.word ?? "",
    PROMPT_KEYWORD_MAX_LENGTH,
  );
  const payload = {
    ...rest,
    word: value,
    weight: clampKeywordWeight(weight),
  };
  if (typeof keyword_id === "number") {
    return { ...payload, keyword_id };
  }
  if (typeof keyword_id === "string" && keyword_id.trim() !== "") {
    const parsed = Number(keyword_id);
    if (!Number.isNaN(parsed)) {
      return { ...payload, keyword_id: parsed };
    }
  }
  return payload;
};

export async function interpretPromptDescription(payload: {
  description: string;
  model_key: string;
  language?: string;
  workspace_token?: string | null;
}): Promise<InterpretPromptResponse> {
  try {
    const requestBody = {
      description: payload.description,
      model_key: payload.model_key,
      language: payload.language,
      workspace_token: payload.workspace_token ?? undefined,
    };
    const response: AxiosResponse<InterpretPromptResponse> = await http.post(
      "/prompts/interpret",
      requestBody,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function augmentPromptKeywords(
  payload: AugmentPromptKeywordsRequest,
): Promise<AugmentPromptKeywordsResponse> {
  try {
    const response: AxiosResponse<AugmentPromptKeywordsResponse> =
      await http.post("/prompts/keywords/augment", {
        topic: payload.topic,
        model_key: payload.model_key,
        language: payload.language,
        workspace_token: payload.workspace_token ?? undefined,
        positive_limit: payload.positive_limit,
        negative_limit: payload.negative_limit,
        existing_positive: payload.existing_positive.map(
          normalisePromptKeyword,
        ),
        existing_negative: payload.existing_negative.map(
          normalisePromptKeyword,
        ),
      });
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function createManualPromptKeyword(
  payload: ManualPromptKeywordRequest,
): Promise<PromptKeywordResult> {
  try {
    const normalizedWeight = clampKeywordWeight(
      payload.weight ?? DEFAULT_KEYWORD_WEIGHT,
    );
    const response: AxiosResponse<PromptKeywordResult> = await http.post(
      "/prompts/keywords/manual",
      {
        ...payload,
        workspace_token: payload.workspace_token ?? undefined,
        weight: normalizedWeight,
      },
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function removePromptKeyword(
  payload: RemovePromptKeywordRequest,
): Promise<void> {
  try {
    await http.post("/prompts/keywords/remove", {
      word: payload.word,
      polarity: payload.polarity,
      workspace_token: payload.workspace_token ?? undefined,
    });
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function syncPromptWorkspaceKeywords(payload: {
  workspace_token: string;
  positive_keywords: PromptKeywordInput[];
  negative_keywords: PromptKeywordInput[];
}): Promise<void> {
  try {
    await http.post("/prompts/keywords/sync", {
      workspace_token: payload.workspace_token,
      positive_keywords: payload.positive_keywords.map(normalisePromptKeyword),
      negative_keywords: payload.negative_keywords.map(normalisePromptKeyword),
    });
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function generatePromptPreview(
  payload: GeneratePromptRequest,
): Promise<GeneratePromptResponse> {
  try {
    const response: AxiosResponse<GeneratePromptResponse> = await http.post(
      "/prompts/generate",
      {
        topic: payload.topic,
        model_key: payload.model_key,
        language: payload.language,
        instructions: payload.instructions,
        tone: payload.tone,
        temperature: payload.temperature,
        max_tokens: payload.max_tokens,
        prompt_id: payload.prompt_id,
        include_keyword_reference: payload.include_keyword_reference,
        workspace_token: payload.workspace_token ?? undefined,
        positive_keywords: payload.positive_keywords.map(
          normalisePromptKeyword,
        ),
        negative_keywords: payload.negative_keywords.map(
          normalisePromptKeyword,
        ),
      },
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function savePrompt(
  payload: SavePromptRequest,
): Promise<SavePromptResponse> {
  try {
    const response: AxiosResponse<SavePromptResponse> = await http.post(
      "/prompts",
      {
        prompt_id: payload.prompt_id,
        topic: payload.topic,
        body: payload.body,
        model: payload.model,
        instructions: payload.instructions,
        publish: payload.publish ?? false,
        status: payload.status,
        tags: payload.tags,
        workspace_token: payload.workspace_token ?? undefined,
        positive_keywords: payload.positive_keywords.map(
          normalisePromptKeyword,
        ),
        negative_keywords: payload.negative_keywords.map(
          normalisePromptKeyword,
        ),
      },
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 列出当前用户配置的所有模型凭据。 */
export async function fetchUserModels(): Promise<UserModelCredential[]> {
  try {
    const response: AxiosResponse<UserModelCredential[]> =
      await http.get("/models");
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 创建新的模型凭据。 */
export async function createUserModel(
  payload: CreateUserModelRequest,
): Promise<UserModelCredential> {
  try {
    const response: AxiosResponse<UserModelCredential> = await http.post(
      "/models",
      payload,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 更新现有模型凭据，支持替换 API Key、修改状态等。 */
export async function updateUserModel(
  id: number,
  payload: UpdateUserModelRequest,
): Promise<UserModelCredential> {
  try {
    const response: AxiosResponse<UserModelCredential> = await http.put(
      `/models/${id}`,
      payload,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 测试模型凭据的连通性，返回一次 Chat Completion 结果。 */
export async function testUserModel(
  id: number,
  payload: TestUserModelRequest = {},
): Promise<ChatCompletionResponse> {
  try {
    const response: AxiosResponse<ChatCompletionResponse> = await http.post(
      `/models/${id}/test`,
      payload,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 删除模型凭据。 */
export async function deleteUserModel(id: number): Promise<void> {
  try {
    await http.delete(`/models/${id}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 获取当前用户的 Prompt 列表，支持按状态与关键词查询。 */
export async function fetchMyPrompts(params: {
  status?: "draft" | "published" | "archived";
  query?: string;
  page?: number;
  pageSize?: number;
} = {}): Promise<PromptListResponse> {
  try {
    const response: AxiosResponse<{ items: PromptListItem[] }> & {
      meta?: PromptListMeta;
    } = await http.get("/prompts", {
      params: {
        status: params.status,
        q: params.query,
        page: params.page,
        page_size: params.pageSize,
      },
    });
    const meta = (response as typeof response & { meta?: PromptListMeta }).meta;
    const fallbackMeta: PromptListMeta = {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
      total_items: response.data?.items?.length ?? 0,
      total_pages: 1,
    };
    return {
      items: response.data?.items ?? [],
      meta: meta ?? fallbackMeta,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 导出当前用户的 Prompt 并返回生成的本地文件路径。 */
export async function exportPrompts(): Promise<PromptExportResult> {
  try {
    const response: AxiosResponse<{
      file_path?: string;
      prompt_count?: number;
      generated_at?: string;
    }> = await http.post("/prompts/export");
    const data = response.data ?? {};
    return {
      filePath: data.file_path ?? "",
      promptCount:
        typeof data.prompt_count === "number" && Number.isFinite(data.prompt_count)
          ? data.prompt_count
          : 0,
      generatedAt: data.generated_at ?? "",
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 导入导出的 Prompt JSON 文件，支持合并或覆盖模式。 */
export async function importPrompts(
  file: File,
  mode: "merge" | "overwrite" = "merge",
): Promise<PromptImportResult> {
  const formData = new FormData();
  formData.append("file", file);
  formData.append("mode", mode);
  try {
    const response: AxiosResponse<{
      imported_count?: number;
      skipped_count?: number;
      errors?: Array<{ topic?: string; reason?: string }>;
    }> = await http.post("/prompts/import", formData, {
      params: { mode },
    });
    const data = response.data ?? {};
    const errors = (data.errors ?? []).map((item) => ({
      topic: item.topic ?? "",
      reason: item.reason ?? "",
    }));
    return {
      importedCount:
        typeof data.imported_count === "number" && Number.isFinite(data.imported_count)
          ? data.imported_count
          : 0,
      skippedCount:
        typeof data.skipped_count === "number" && Number.isFinite(data.skipped_count)
          ? data.skipped_count
          : 0,
      errors,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 获取单条 Prompt 的详情，同时返回最新工作区 token。 */
export async function fetchPromptDetail(
  id: number,
): Promise<PromptDetailResponse> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    const response: AxiosResponse<PromptDetailResponse> = await http.get(
      `/prompts/${id}`,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 获取指定 Prompt 的历史版本列表。 */
export async function fetchPromptVersions(
  promptId: number,
  limit?: number,
): Promise<PromptVersionSummary[]> {
  if (!promptId) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    const response: AxiosResponse<{ versions: Array<{ version_no: number; model: string; created_at: string }> }> =
      await http.get(`/prompts/${promptId}/versions`, {
        params: typeof limit === "number" && Number.isFinite(limit) ? { limit } : undefined,
      });
    const entries = response.data?.versions ?? [];
    return entries.map((item) => ({
      versionNo: item.version_no,
      model: item.model,
      createdAt: item.created_at,
    }));
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 获取 Prompt 指定版本的详细内容。 */
export async function fetchPromptVersion(
  promptId: number,
  versionNo: number,
): Promise<PromptVersionDetail> {
  if (!promptId || !versionNo) {
    throw new ApiError({ message: "Prompt id and version are required" });
  }
  try {
    const response: AxiosResponse<{
      version_no: number;
      model: string;
      body: string;
      instructions?: string | null;
      positive_keywords: PromptListKeyword[];
      negative_keywords: PromptListKeyword[];
      created_at: string;
    }> = await http.get(`/prompts/${promptId}/versions/${versionNo}`);
    const data = response.data;
    return {
      versionNo: data.version_no,
      model: data.model,
      body: data.body,
      instructions: data.instructions,
      positive_keywords: data.positive_keywords ?? [],
      negative_keywords: data.negative_keywords ?? [],
      created_at: data.created_at,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 删除指定 Prompt。 */
export async function deletePrompt(id: number): Promise<void> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    await http.delete(`/prompts/${id}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 提交登录请求，返回用户信息与令牌。 */
export async function login(payload: LoginRequest): Promise<AuthResponse> {
  try {
    const response: AxiosResponse<AuthResponse> = await http.post(
      "/auth/login",
      payload,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 注册新用户，成功后立即返回用户信息与令牌。 */
export async function register(
  payload: RegisterRequest,
): Promise<AuthResponse> {
  try {
    const response: AxiosResponse<AuthResponse> = await http.post(
      "/auth/register",
      payload,
    );
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
    const response: AxiosResponse<{ avatar_url: string }> = await http.post(
      "/uploads/avatar",
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
      },
    );
    return response.data.avatar_url;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 刷新 access token，通常由 http 拦截器内部调用。 */
export async function refreshTokens(refreshToken: string): Promise<AuthTokens> {
  try {
    const response: AxiosResponse<{ tokens: AuthTokens }> = await http.post(
      "/auth/refresh",
      {
        refresh_token: refreshToken,
      },
    );
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
export async function requestEmailVerification(
  email: string,
): Promise<EmailVerificationRequestResult> {
  try {
    const response: AxiosResponse<{
      issued: boolean;
      token?: string;
      remaining_attempts?: number;
    }> = await http.post("/auth/verify-email/request", {
      email,
    });
    const data = response.data ?? {};
    return {
      issued: Boolean(data.issued),
      token: data.token,
      remainingAttempts:
        typeof data.remaining_attempts === "number" &&
        Number.isFinite(data.remaining_attempts)
          ? data.remaining_attempts
          : undefined,
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
  console.info("[frontend] fetchCurrentUser called");
  try {
    const response: AxiosResponse<AuthProfile> = await http.get("/users/me");
    console.info("[frontend] fetchCurrentUser success", response.status);
    return response.data;
  } catch (error) {
    console.error("[frontend] fetchCurrentUser failed", error);
    throw normaliseError(error);
  }
}

/** 更新当前登录用户的基础资料和设置。 */
export async function updateCurrentUser(
  payload: UpdateCurrentUserRequest,
): Promise<AuthProfile> {
  try {
    const response: AxiosResponse<AuthProfile> = await http.put(
      "/users/me",
      payload,
    );
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 创建图形验证码并返回 base64 图片与标识。 */
export async function fetchCaptcha(): Promise<CaptchaResponse> {
  try {
    const response: AxiosResponse<CaptchaResponse> =
      await http.get("/auth/captcha");
    return response.data;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function fetchChangelogEntries(
  locale?: string,
): Promise<ChangelogEntry[]> {
  try {
    const response: AxiosResponse<{ items: ChangelogEntry[] }> = await http.get(
      "/changelog",
      {
        params: locale ? { locale } : undefined,
      },
    );
    return response.data.items ?? [];
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function createChangelogEntry(
  payload: ChangelogPayload,
): Promise<ChangelogCreateResult> {
  try {
    const response: AxiosResponse<{
      entry: ChangelogEntry;
      translations?: ChangelogEntry[];
    }> = await http.post("/changelog", payload);
    return {
      entry: response.data.entry,
      translations: response.data.translations ?? [],
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function updateChangelogEntry(
  id: number,
  payload: ChangelogPayload,
): Promise<ChangelogEntry> {
  try {
    const response: AxiosResponse<{ entry: ChangelogEntry }> = await http.put(
      `/changelog/${id}`,
      payload,
    );
    return response.data.entry;
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function deleteChangelogEntry(id: number): Promise<void> {
  try {
    await http.delete(`/changelog/${id}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 拉取 IP Guard 黑名单列表，仅管理员可用。 */
export async function fetchIpGuardBans(): Promise<IpGuardEntry[]> {
  try {
    const response: AxiosResponse<{ items: IpGuardEntry[] }> =
      await http.get("/ip-guard/bans");
    return response.data.items ?? [];
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 解除指定 IP 的封禁记录。 */
export async function removeIpGuardBan(ip: string): Promise<void> {
  const trimmed = ip.trim();
  if (!trimmed) {
    throw new ApiError({ message: "IP address is required" });
  }
  try {
    const encoded = encodeURIComponent(trimmed);
    await http.delete(`/ip-guard/bans/${encoded}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

/**
 * Exposes helper utilities so other modules (e.g. login form) can set or clear the token
 * pair returned by the backend authentication endpoints.
 */
export {
  clearTokenPair as clearAuthTokens,
  getTokenPair as getAuthTokens,
  setTokenPair as setAuthTokens,
} from "./tokenStorage";
