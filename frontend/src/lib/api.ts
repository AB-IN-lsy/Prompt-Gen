/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:44:21
 * @FilePath: \electron-go-app\frontend\src\lib\api.ts
 * @LastEditTime: 2025-10-09 23:53:15
 */
import type { AxiosResponse } from "axios";
import {
  DEFAULT_KEYWORD_WEIGHT,
  PROMPT_KEYWORD_MAX_LENGTH,
  MY_PROMPTS_PAGE_SIZE,
  PUBLIC_PROMPT_LIST_PAGE_SIZE,
} from "../config/prompt";
import { ApiError } from "./errors";
import { http, normaliseError } from "./http";
import { clampTextWithOverflow } from "./utils";

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

const parseJsonSafe = <T>(value: unknown, fallback: T): T => {
  if (value == null) {
    return fallback;
  }
  if (typeof value === "object") {
    return value as T;
  }
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "") {
      return fallback;
    }
    try {
      return JSON.parse(trimmed) as T;
    } catch {
      return fallback;
    }
  }
  return fallback;
};

const parseStringArray = (value: unknown): string[] => {
  const parsed = parseJsonSafe<unknown[]>(value, []);
  if (!Array.isArray(parsed)) {
    if (typeof value === "string" && value.includes(",")) {
      return value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean);
    }
    return [];
  }
  return parsed
    .map((item) => {
      if (typeof item === "string") {
        return item.trim();
      }
      if (item && typeof item === "object" && "name" in item) {
        return String((item as { name?: string }).name ?? "").trim();
      }
      return "";
    })
    .filter(Boolean);
};

export interface PublicPromptKeywordItem {
  word: string;
  source?: string;
  weight?: number;
}

const parsePublicPromptKeywords = (value: unknown): PublicPromptKeywordItem[] => {
  const parsed = parseJsonSafe<unknown[]>(value, []);
  if (!Array.isArray(parsed)) {
    return [];
  }
  return parsed
    .map((item) => {
      if (typeof item === "string") {
        return { word: item };
      }
      if (item && typeof item === "object") {
        const record = item as Record<string, unknown>;
        const word = typeof record.word === "string" ? record.word : "";
        if (!word) {
          return null;
        }
        const weight =
          typeof record.weight === "number" && Number.isFinite(record.weight)
            ? record.weight
            : undefined;
        const source =
          typeof record.source === "string" && record.source.trim() !== ""
            ? record.source
            : undefined;
        return { word, source, weight };
      }
      return null;
    })
    .filter((item): item is PublicPromptKeywordItem => Boolean(item));
};

const normalizeGenerationProfilePayload = (
  profile?: PromptGenerationProfile,
): PromptGenerationProfile | undefined => {
  if (!profile) {
    return undefined;
  }
  const payload: PromptGenerationProfile = {};
  if (typeof profile.stepwise_reasoning === "boolean") {
    payload.stepwise_reasoning = profile.stepwise_reasoning;
  }
  if (typeof profile.temperature === "number" && Number.isFinite(profile.temperature)) {
    payload.temperature = profile.temperature;
  }
  if (typeof profile.top_p === "number" && Number.isFinite(profile.top_p)) {
    payload.top_p = profile.top_p;
  }
  if (typeof profile.max_output_tokens === "number" && Number.isFinite(profile.max_output_tokens)) {
    payload.max_output_tokens = Math.trunc(profile.max_output_tokens);
  }
  return payload;
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

export interface PromptGenerationProfile {
  stepwise_reasoning?: boolean;
  temperature?: number;
  top_p?: number;
  max_output_tokens?: number;
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
  is_favorited?: boolean;
  is_liked?: boolean;
  like_count?: number;
  generation_profile?: PromptGenerationProfile;
}

export interface PromptListMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
  current_count: number;
}

export interface PromptListResponse {
  items: PromptListItem[];
  meta: PromptListMeta;
}

export interface PublicPromptAuthor {
  id: number;
  username: string;
  avatar_url?: string;
  headline?: string;
  bio?: string;
  location?: string;
  website?: string;
  banner_url?: string;
}

export interface PublicPromptListItem {
  id: number;
  title: string;
  topic: string;
  summary: string;
  model: string;
  language: string;
  status: "pending" | "approved" | "rejected" | string;
  tags: string[];
  download_count: number;
  visit_count: number;
  quality_score: number;
  created_at: string;
  updated_at: string;
  author_user_id: number;
  reviewer_user_id?: number | null;
  review_reason?: string | null;
  is_liked: boolean;
  like_count: number;
  author?: PublicPromptAuthor | null;
}

export interface PublicPromptListMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
  current_count: number;
}

export interface PublicPromptListResponse {
  items: PublicPromptListItem[];
  meta: PublicPromptListMeta;
}

export interface PublicPromptDetail extends PublicPromptListItem {
  body: string;
  instructions: string;
  positive_keywords: PublicPromptKeywordItem[];
  negative_keywords: PublicPromptKeywordItem[];
  source_prompt_id?: number | null;
}

export interface CreatorStats {
  prompt_count: number;
  total_downloads: number;
  total_likes: number;
  total_visits: number;
}

export interface CreatorProfileResponse {
  creator: PublicPromptAuthor | null;
  stats: CreatorStats;
  recent_prompts: PublicPromptListItem[];
}

export interface PromptCommentAuthor {
  id: number;
  username: string;
  email: string;
  avatar_url?: string;
}

export interface PromptComment {
  id: number;
  prompt_id: number;
  user_id: number;
  parent_id?: number | null;
  root_id?: number | null;
  body: string;
  status: "pending" | "approved" | "rejected" | string;
  like_count: number;
  is_liked?: boolean;
  reply_count: number;
  author?: PromptCommentAuthor | null;
  review_note?: string | null;
  reviewer_user_id?: number | null;
  created_at: string;
  updated_at: string;
  replies?: PromptComment[];
}

export interface PromptCommentListMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
  current_count: number;
}

export interface PromptCommentListResponse {
  items: PromptComment[];
  meta: PromptCommentListMeta;
}

export interface PromptCommentCreatePayload {
  body: string;
  parentId?: number | null;
}

export interface PromptCommentReviewPayload {
  status: "approved" | "rejected" | "pending";
  note?: string;
}

export interface PromptCommentLikeResult {
  liked: boolean;
  like_count: number;
}

export interface PublicPromptDownloadResult {
  promptId: number | null;
  status: string;
}

export interface PublicPromptLikeResult {
  liked: boolean;
  like_count: number;
}

export interface PublicPromptSubmitPayload {
  sourcePromptId?: number | null;
  title: string;
  topic: string;
  summary: string;
  body: string;
  instructions: string;
  positiveKeywords: string;
  negativeKeywords: string;
  tags: string;
  model: string;
  language?: string;
}

export interface PublicPromptSubmitResult {
  id: number;
  status: string;
  created: string;
}

export interface PublicPromptReviewPayload {
  status: "approved" | "rejected";
  reason?: string;
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

export interface SharePromptResponse {
  payload: string;
  topic: string;
  payload_size?: number;
  generated_at?: string;
}

export interface ImportSharedPromptResult {
  prompt_id: number;
  topic: string;
  status: string;
  imported_at?: string;
}

export interface IngestPromptPayload {
  body: string;
  model_key?: string;
  language?: string;
}

export interface PromptVersionSummary {
  versionNo: number;
  model: string;
  createdAt: string;
  generation_profile?: PromptGenerationProfile;
}

export interface PromptVersionDetail {
  versionNo: number;
  model: string;
  body: string;
  instructions?: string | null;
  positive_keywords: PromptListKeyword[];
  negative_keywords: PromptListKeyword[];
  created_at: string;
  generation_profile?: PromptGenerationProfile;
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
  is_favorited?: boolean;
  is_liked?: boolean;
  like_count?: number;
  generation_profile?: PromptGenerationProfile;
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
  profile_headline?: string;
  profile_bio?: string;
  profile_location?: string;
  profile_website?: string;
  profile_banner_url?: string;
  is_admin: boolean;
  email_verified_at?: string | null;
  last_login_at?: string | null;
  created_at?: string;
  updated_at?: string;
}

export interface UserSettings {
  preferred_model: string;
  enable_animations: boolean;
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
  actual_model?: string;
  base_url?: string;
  extra_config: Record<string, unknown>;
  status: ModelStatus;
  last_verified_at?: string | null;
  created_at: string;
  updated_at: string;
  is_builtin?: boolean;
  daily_quota?: number | null;
  remaining_quota?: number | null;
  reset_after_seconds?: number | null;
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
  tags?: string[];
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
  existing_body?: string;
  description?: string;
  prompt_id?: number;
  language?: string;
  instructions?: string;
  tone?: string;
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
  stepwise_reasoning?: boolean;
  include_keyword_reference?: boolean;
  workspace_token?: string;
  generation_profile?: PromptGenerationProfile;
}

export interface GeneratePromptResponse {
  prompt: string;
  model: string;
  duration_ms?: number;
  usage?: ChatCompletionUsage;
  topic?: string;
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
  generation_profile?: PromptGenerationProfile;
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
  enable_animations?: boolean;
  profile_headline?: string | null;
  profile_bio?: string | null;
  profile_location?: string | null;
  profile_website?: string | null;
  profile_banner_url?: string | null;
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
    const generationProfile = normalizeGenerationProfilePayload(
      payload.generation_profile,
    );
    const temperatureValue =
      typeof payload.temperature === "number"
        ? payload.temperature
        : generationProfile?.temperature;
    const maxTokensValue =
      typeof payload.max_tokens === "number"
        ? payload.max_tokens
        : generationProfile?.max_output_tokens;
    const topPValue =
      typeof payload.top_p === "number" ? payload.top_p : generationProfile?.top_p;
    const stepwiseValue =
      typeof payload.stepwise_reasoning === "boolean"
        ? payload.stepwise_reasoning
        : generationProfile?.stepwise_reasoning;
    const existingBody = payload.existing_body?.trim();
    const response: AxiosResponse<GeneratePromptResponse> = await http.post(
      "/prompts/generate",
      {
        topic: payload.topic,
        model_key: payload.model_key,
        language: payload.language,
        instructions: payload.instructions,
        description: payload.description,
        tone: payload.tone,
        temperature: temperatureValue,
        max_tokens: maxTokensValue,
        top_p: topPValue,
        stepwise_reasoning: stepwiseValue,
        prompt_id: payload.prompt_id,
        include_keyword_reference: payload.include_keyword_reference,
        workspace_token: payload.workspace_token ?? undefined,
        existing_body: existingBody ? existingBody : undefined,
        positive_keywords: payload.positive_keywords.map(
          normalisePromptKeyword,
        ),
        negative_keywords: payload.negative_keywords.map(
          normalisePromptKeyword,
        ),
        generation_profile: generationProfile,
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
    const generationProfile = normalizeGenerationProfilePayload(
      payload.generation_profile,
    );
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
        generation_profile: generationProfile,
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
  favorited?: boolean;
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
        favorited:
          typeof params.favorited === "boolean"
            ? params.favorited
              ? 1
              : 0
            : undefined,
      },
    });
    const meta = (response as typeof response & { meta?: PromptListMeta }).meta;
    const effectivePageSize = params.pageSize ?? MY_PROMPTS_PAGE_SIZE;
    const fallbackMeta: PromptListMeta = {
      page: params.page ?? 1,
      page_size: effectivePageSize,
      total_items: response.data?.items?.length ?? 0,
      total_pages: 1,
      current_count: Math.min(
        response.data?.items?.length ?? 0,
        effectivePageSize,
      ),
    };
    const resolvedMeta = meta ?? fallbackMeta;
    const currentCount =
      resolvedMeta.current_count ?? response.data?.items?.length ?? 0;
    return {
      items: (response.data?.items ?? []).map((item) => ({
        ...item,
        is_favorited: Boolean(item?.is_favorited),
        is_liked: Boolean(item?.is_liked),
        like_count:
          typeof item?.like_count === "number" && Number.isFinite(item.like_count)
            ? item.like_count
            : 0,
      })),
      meta: {
        ...resolvedMeta,
        current_count: currentCount,
      },
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function fetchPublicPrompts(params: {
  query?: string;
  status?: string;
  page?: number;
  pageSize?: number;
  authorId?: number;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
} = {}): Promise<PublicPromptListResponse> {
  try {
    const response: AxiosResponse<{ items: any[] }> & {
      meta?: PublicPromptListMeta;
    } = await http.get("/public-prompts", {
      params: {
        q: params.query,
        status: params.status,
        page: params.page,
        page_size: params.pageSize,
        author_id: params.authorId,
        sort_by: params.sortBy,
        sort_order: params.sortOrder,
      },
    });
    const items = (response.data?.items ?? []).map((item) => {
      const tags = parseStringArray(item?.tags);
      return {
        id: Number(item?.id ?? 0),
        title: String(item?.title ?? item?.topic ?? ""),
        topic: String(item?.topic ?? ""),
        summary: String(item?.summary ?? ""),
        model: String(item?.model ?? ""),
        language: String(item?.language ?? "zh-CN"),
        status: String(item?.status ?? "pending"),
        tags,
        download_count: Number(item?.download_count ?? 0),
        visit_count:
          typeof item?.visit_count === "number" && Number.isFinite(item.visit_count)
            ? Number(item.visit_count)
            : 0,
        created_at: String(item?.created_at ?? ""),
        updated_at: String(item?.updated_at ?? ""),
        author_user_id: Number(item?.author_user_id ?? 0),
        reviewer_user_id:
          item?.reviewer_user_id === null || item?.reviewer_user_id === undefined
            ? undefined
            : Number(item.reviewer_user_id),
        review_reason:
          typeof item?.review_reason === "string" ? item.review_reason : undefined,
        is_liked: Boolean(item?.is_liked),
        like_count:
          typeof item?.like_count === "number" && Number.isFinite(item.like_count)
            ? item.like_count
            : 0,
        quality_score:
          typeof item?.quality_score === "number" && Number.isFinite(item.quality_score)
            ? item.quality_score
            : 0,
        author: parsePublicPromptAuthor(item?.author),
      } as PublicPromptListItem;
    });
    const effectivePageSize = params.pageSize ?? PUBLIC_PROMPT_LIST_PAGE_SIZE;
    const fallbackMeta: PublicPromptListMeta = {
      page: params.page ?? 1,
      page_size: effectivePageSize,
      total_items: items.length,
      total_pages: 1,
      current_count: Math.min(items.length, effectivePageSize),
    };
    const resolvedMeta = response.meta ?? fallbackMeta;
    const currentCount = resolvedMeta.current_count ?? items.length;
    return {
      items,
      meta: {
        ...resolvedMeta,
        current_count: currentCount,
      },
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function fetchPublicPromptDetail(
  id: number,
): Promise<PublicPromptDetail> {
  try {
    const response: AxiosResponse<Record<string, unknown>> = await http.get(
      `/public-prompts/${id}`,
    );
    const data = response.data ?? {};
    const tags = parseStringArray(data.tags);
    const positive = parsePublicPromptKeywords(data.positive_keywords);
    const negative = parsePublicPromptKeywords(data.negative_keywords);
    return {
      id: Number(data.id ?? id),
      title: String(data.title ?? data.topic ?? ""),
      topic: String(data.topic ?? ""),
      summary: String(data.summary ?? ""),
      model: String(data.model ?? ""),
      language: String(data.language ?? "zh-CN"),
      status: String(data.status ?? "pending"),
      source_prompt_id:
        typeof data.source_prompt_id === "number" && Number.isFinite(data.source_prompt_id)
          ? data.source_prompt_id
          : null,
      tags,
      download_count: Number(data.download_count ?? 0),
      visit_count:
        typeof data?.visit_count === "number" && Number.isFinite(data.visit_count)
          ? Number(data.visit_count)
          : 0,
      quality_score:
        typeof data?.quality_score === "number" && Number.isFinite(data.quality_score)
          ? Number(data.quality_score)
          : 0,
      created_at: String(data.created_at ?? ""),
      updated_at: String(data.updated_at ?? ""),
      author_user_id: Number(data.author_user_id ?? 0),
      reviewer_user_id:
        data.reviewer_user_id === null || data.reviewer_user_id === undefined
          ? undefined
          : Number(data.reviewer_user_id),
      review_reason:
        typeof data.review_reason === "string" ? data.review_reason : undefined,
      is_liked: Boolean(data?.is_liked),
      like_count:
        typeof data?.like_count === "number" && Number.isFinite(data.like_count)
          ? data.like_count
          : 0,
      author: parsePublicPromptAuthor(data.author),
      body: String(data.body ?? ""),
      instructions: String(data.instructions ?? ""),
      positive_keywords: positive,
      negative_keywords: negative,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function downloadPublicPrompt(
  id: number,
): Promise<PublicPromptDownloadResult> {
  try {
    const response: AxiosResponse<{
      prompt_id?: number;
      status?: string;
    }> = await http.post(`/public-prompts/${id}/download`);
    return {
      promptId:
        typeof response.data?.prompt_id === "number"
          ? response.data.prompt_id
          : null,
      status: String(response.data?.status ?? ""),
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function likePublicPrompt(id: number): Promise<PublicPromptLikeResult> {
  try {
    const response: AxiosResponse<{ liked?: boolean; like_count?: number }> = await http.post(`/public-prompts/${id}/like`);
    return {
      liked: Boolean(response.data?.liked),
      like_count:
        typeof response.data?.like_count === "number" && Number.isFinite(response.data.like_count)
          ? response.data.like_count
          : 0,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function unlikePublicPrompt(id: number): Promise<PublicPromptLikeResult> {
  try {
    const response: AxiosResponse<{ liked?: boolean; like_count?: number }> = await http.delete(`/public-prompts/${id}/like`);
    return {
      liked: Boolean(response.data?.liked),
      like_count:
        typeof response.data?.like_count === "number" && Number.isFinite(response.data.like_count)
          ? response.data.like_count
          : 0,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function likePromptComment(id: number): Promise<PromptCommentLikeResult> {
  try {
    const response: AxiosResponse<{ liked?: boolean; like_count?: number }> = await http.post(`/prompts/comments/${id}/like`);
    return {
      liked: Boolean(response.data?.liked),
      like_count:
        typeof response.data?.like_count === "number" && Number.isFinite(response.data.like_count)
          ? response.data.like_count
          : 0,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function unlikePromptComment(id: number): Promise<PromptCommentLikeResult> {
  try {
    const response: AxiosResponse<{ liked?: boolean; like_count?: number }> = await http.delete(`/prompts/comments/${id}/like`);
    return {
      liked: Boolean(response.data?.liked),
      like_count:
        typeof response.data?.like_count === "number" && Number.isFinite(response.data.like_count)
          ? response.data.like_count
          : 0,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function submitPublicPrompt(
  payload: PublicPromptSubmitPayload,
): Promise<PublicPromptSubmitResult> {
  try {
    const response: AxiosResponse<{
      id?: number;
      status?: string;
      created?: string;
    }> = await http.post("/public-prompts", {
      source_prompt_id:
        typeof payload.sourcePromptId === "number" && payload.sourcePromptId > 0
          ? payload.sourcePromptId
          : undefined,
      title: payload.title,
      topic: payload.topic,
      summary: payload.summary,
      body: payload.body,
      instructions: payload.instructions,
      positive_keywords: payload.positiveKeywords,
      negative_keywords: payload.negativeKeywords,
      tags: payload.tags,
      model: payload.model,
      language: payload.language ?? "zh-CN",
    });
    return {
      id: Number(response.data?.id ?? 0),
      status: String(response.data?.status ?? "pending"),
      created: String(response.data?.created ?? ""),
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function fetchCreatorProfile(id: number): Promise<CreatorProfileResponse> {
  try {
    const response: AxiosResponse<{
      creator?: unknown;
      stats?: {
        prompt_count?: number;
        total_downloads?: number;
        total_likes?: number;
        total_visits?: number;
      };
      recent_prompts?: any[];
    }> = await http.get(`/creators/${id}`);
    const creator = parsePublicPromptAuthor(response.data?.creator);
    const statsPayload = response.data?.stats ?? {};
    const stats: CreatorStats = {
      prompt_count: Number(statsPayload.prompt_count ?? 0),
      total_downloads: Number(statsPayload.total_downloads ?? 0),
      total_likes: Number(statsPayload.total_likes ?? 0),
      total_visits: Number(statsPayload.total_visits ?? 0),
    };
    const recentItems = (response.data?.recent_prompts ?? []).map((item) => {
      const tags = parseStringArray(item?.tags);
      return {
        id: Number(item?.id ?? 0),
        title: String(item?.title ?? item?.topic ?? ""),
        topic: String(item?.topic ?? ""),
        summary: String(item?.summary ?? ""),
        model: String(item?.model ?? ""),
        language: String(item?.language ?? "zh-CN"),
        status: String(item?.status ?? "approved"),
        tags,
        download_count: Number(item?.download_count ?? 0),
        visit_count: Number(item?.visit_count ?? 0),
        quality_score:
          typeof item?.quality_score === "number" && Number.isFinite(item.quality_score)
            ? item.quality_score
            : 0,
        created_at: String(item?.created_at ?? ""),
        updated_at: String(item?.updated_at ?? ""),
        author_user_id: Number(item?.author_user_id ?? creator?.id ?? 0),
        reviewer_user_id:
          item?.reviewer_user_id === null || item?.reviewer_user_id === undefined
            ? undefined
            : Number(item.reviewer_user_id),
        review_reason:
          typeof item?.review_reason === "string" ? item.review_reason : undefined,
        is_liked: Boolean(item?.is_liked),
        like_count:
          typeof item?.like_count === "number" && Number.isFinite(item.like_count)
            ? item.like_count
            : 0,
        author: parsePublicPromptAuthor(item?.author ?? response.data?.creator),
      } as PublicPromptListItem;
    });
    return {
      creator,
      stats,
      recent_prompts: recentItems,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function reviewPublicPrompt(
  id: number,
  payload: PublicPromptReviewPayload,
): Promise<void> {
  try {
    await http.post(`/public-prompts/${id}/review`, {
      status: payload.status,
      reason: payload.reason,
    });
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function deletePublicPrompt(id: number): Promise<void> {
  try {
    await http.delete(`/public-prompts/${id}`);
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function fetchPromptComments(
  promptId: number,
  params: { page?: number; pageSize?: number; status?: string } = {},
): Promise<PromptCommentListResponse> {
  try {
    const response: AxiosResponse<{ items: any[] }> & { meta?: PromptCommentListMeta } = await http.get(
      `/prompts/${promptId}/comments`,
      {
        params: {
          page: params.page,
          page_size: params.pageSize,
          status: params.status,
        },
      },
    );
    const items = (response.data?.items ?? []).map((item) => parsePromptComment(item));
    const fallbackMeta: PromptCommentListMeta = {
      page: params.page ?? 1,
      page_size: params.pageSize ?? items.length,
      total_items: items.length,
      total_pages: 1,
      current_count: items.length,
    };
    const meta = response.meta ?? fallbackMeta;
    return {
      items,
      meta: {
        ...meta,
        current_count: meta.current_count ?? items.length,
      },
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function createPromptComment(
  promptId: number,
  payload: PromptCommentCreatePayload,
): Promise<PromptComment> {
  try {
    const response: AxiosResponse<Record<string, unknown>> = await http.post(`/prompts/${promptId}/comments`, {
      body: payload.body,
      parent_id:
        typeof payload.parentId === "number" && payload.parentId > 0 ? payload.parentId : undefined,
    });
    return parsePromptComment(response.data);
  } catch (error) {
    throw normaliseError(error);
  }
}

export async function reviewPromptComment(
  commentId: number,
  payload: PromptCommentReviewPayload,
): Promise<PromptComment> {
  try {
    const response: AxiosResponse<Record<string, unknown>> = await http.post(`/prompts/comments/${commentId}/review`, {
      status: payload.status,
      note: payload.note,
    });
    return parsePromptComment(response.data);
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

/** 生成指定 Prompt 的分享串。 */
export async function sharePrompt(id: number): Promise<SharePromptResponse> {
  if (!id) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    const response: AxiosResponse<SharePromptResponse> = await http.post(
      `/prompts/${id}/share`,
    );
    const data = response.data ?? {};
    if (typeof data.payload !== "string" || data.payload.trim() === "") {
      throw new ApiError({ message: "invalid share response" });
    }
    return {
      payload: data.payload,
      topic: data.topic ?? "",
      payload_size: data.payload_size,
      generated_at: data.generated_at,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 通过分享串导入 Prompt。 */
export async function importSharedPrompt(payload: string): Promise<ImportSharedPromptResult> {
  if (!payload.trim()) {
    throw new ApiError({ message: "Share payload is required" });
  }
  try {
    const response: AxiosResponse<ImportSharedPromptResult> = await http.post(
      "/prompts/share/import",
      { payload },
    );
    return response.data ?? { prompt_id: 0, topic: "", status: "draft" };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 自动解析成品 Prompt 并生成草稿。 */
export async function ingestPrompt(params: IngestPromptPayload): Promise<PromptDetailResponse> {
  const body = params.body?.trim();
  if (!body) {
    throw new ApiError({ message: "Prompt body is required" });
  }
  const payload: Record<string, string> = { body };
  if (params.model_key && params.model_key.trim()) {
    payload.model_key = params.model_key.trim();
  }
  if (params.language && params.language.trim()) {
    payload.language = params.language.trim();
  }
  try {
    const response: AxiosResponse<PromptDetailResponse> = await http.post(
      "/prompts/ingest",
      payload,
    );
    const data = response.data ?? {};
    return {
      ...data,
      tags: Array.isArray(data?.tags) ? (data.tags as string[]) : [],
      is_favorited: Boolean(data?.is_favorited),
      is_liked: Boolean(data?.is_liked),
      like_count:
        typeof data?.like_count === "number" && Number.isFinite(data.like_count)
          ? data.like_count
          : 0,
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
    const data = response.data;
    return {
      ...data,
      is_favorited: Boolean(data?.is_favorited),
      is_liked: Boolean(data?.is_liked),
      like_count:
        typeof data?.like_count === "number" && Number.isFinite(data.like_count)
          ? data.like_count
          : 0,
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

/** 收藏或取消收藏 Prompt。 */
export async function updatePromptFavorite(params: {
  promptId: number;
  favorited: boolean;
}): Promise<{ favorited: boolean }> {
  if (!params.promptId) {
    throw new ApiError({ message: "Prompt id is required" });
  }
  try {
    const response: AxiosResponse<{ favorited: boolean }> = await http.patch(
      `/prompts/${params.promptId}/favorite`,
      { favorited: params.favorited },
    );
    return {
      favorited: Boolean(response.data?.favorited),
    };
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

export interface AdminMetricsDailyMetric {
  date: string;
  active_users: number;
  generate_requests: number;
  generate_success: number;
  generate_success_rate: number;
  average_latency_ms: number;
  save_requests: number;
}

export interface AdminMetricsTotals {
  active_users: number;
  generate_requests: number;
  generate_success: number;
  generate_success_rate: number;
  average_latency_ms: number;
  save_requests: number;
}

export interface AdminMetricsSnapshot {
  refreshed_at: string;
  range_days: number;
  daily: AdminMetricsDailyMetric[];
  totals: AdminMetricsTotals;
}

export async function fetchAdminMetrics(): Promise<AdminMetricsSnapshot> {
  try {
    const response: AxiosResponse<AdminMetricsSnapshot> = await http.get(
      "/admin/metrics",
    );
    const snapshot = response.data;
    return {
      refreshed_at: snapshot?.refreshed_at ?? "",
      range_days: snapshot?.range_days ?? 0,
      totals: snapshot?.totals ?? {
        active_users: 0,
        generate_requests: 0,
        generate_success: 0,
        generate_success_rate: 0,
        average_latency_ms: 0,
        save_requests: 0,
      },
      daily: Array.isArray(snapshot?.daily) ? snapshot.daily : [],
    };
  } catch (error) {
    throw normaliseError(error);
  }
}

export interface AdminUserPromptTotals {
  total: number;
  draft: number;
  published: number;
  archived: number;
}

export interface AdminUserPromptSummary {
  id: number;
  topic: string;
  status: string;
  updated_at: string;
  created_at: string;
}

export interface AdminUserOverviewItem {
  id: number;
  username: string;
  email: string;
  avatar_url?: string | null;
  is_admin: boolean;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
  is_online: boolean;
  prompt_totals: AdminUserPromptTotals;
  latest_prompt_at?: string | null;
  recent_prompts: AdminUserPromptSummary[];
}

export interface AdminUserOverviewResponse {
  items: AdminUserOverviewItem[];
  total: number;
  page: number;
  page_size: number;
  online_threshold_seconds: number;
}

export interface FetchAdminUsersParams {
  page?: number;
  pageSize?: number;
  query?: string;
}

export async function fetchAdminUsers(
  params?: FetchAdminUsersParams,
): Promise<AdminUserOverviewResponse> {
  try {
    const response: AxiosResponse<AdminUserOverviewResponse> = await http.get(
      "/admin/users",
      {
        params: {
          page: params?.page,
          page_size: params?.pageSize,
          query: params?.query,
        },
      },
    );
    const payload = response.data ?? {};
    const rawItems = Array.isArray(payload.items) ? payload.items : [];
    const items: AdminUserOverviewItem[] = rawItems.map((item) => ({
      id: item.id ?? 0,
      username: item.username ?? "",
      email: item.email ?? "",
      avatar_url: item.avatar_url ?? null,
      is_admin: Boolean(item.is_admin),
      last_login_at: item.last_login_at ?? null,
      created_at: item.created_at ?? "",
      updated_at: item.updated_at ?? "",
      is_online: Boolean(item.is_online),
      prompt_totals: {
        total: item.prompt_totals?.total ?? 0,
        draft: item.prompt_totals?.draft ?? 0,
        published: item.prompt_totals?.published ?? 0,
        archived: item.prompt_totals?.archived ?? 0,
      },
      latest_prompt_at: item.latest_prompt_at ?? null,
      recent_prompts: Array.isArray(item.recent_prompts)
        ? item.recent_prompts.map((prompt) => ({
            id: prompt.id ?? 0,
            topic: prompt.topic ?? "",
            status: prompt.status ?? "",
            updated_at: prompt.updated_at ?? "",
            created_at: prompt.created_at ?? "",
          }))
        : [],
    }));
    return {
      items,
      total: typeof payload.total === "number" ? payload.total : items.length,
      page: typeof payload.page === "number" ? payload.page : params?.page ?? 1,
      page_size:
        typeof payload.page_size === "number"
          ? payload.page_size
          : params?.pageSize ?? items.length,
      online_threshold_seconds:
        typeof payload.online_threshold_seconds === "number"
          ? payload.online_threshold_seconds
          : 0,
    };
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

function parsePromptComment(raw: Record<string, unknown>): PromptComment {
  const comment: PromptComment = {
    id: Number(raw.id ?? 0),
    prompt_id: Number(raw.prompt_id ?? 0),
    user_id: Number(raw.user_id ?? 0),
    parent_id:
      raw.parent_id === null || raw.parent_id === undefined
        ? null
        : Number(raw.parent_id),
    root_id:
      raw.root_id === null || raw.root_id === undefined
        ? null
        : Number(raw.root_id),
    body: String(raw.body ?? ""),
    status: String(raw.status ?? "approved"),
    like_count:
      typeof raw.like_count === "number" && Number.isFinite(raw.like_count)
        ? raw.like_count
        : 0,
    is_liked:
      typeof raw.is_liked === "boolean"
        ? raw.is_liked
        : raw.is_liked === undefined
        ? undefined
        : Boolean(raw.is_liked),
    reply_count:
      typeof raw.reply_count === "number" && Number.isFinite(raw.reply_count)
        ? raw.reply_count
        : 0,
    author: parsePromptCommentAuthor(raw.author),
    review_note:
      typeof raw.review_note === "string" ? raw.review_note : undefined,
    reviewer_user_id:
      raw.reviewer_user_id === null || raw.reviewer_user_id === undefined
        ? undefined
        : Number(raw.reviewer_user_id),
    created_at: String(raw.created_at ?? ""),
    updated_at: String(raw.updated_at ?? ""),
  };
  if (Array.isArray(raw.replies)) {
    comment.replies = raw.replies.map((item) =>
      parsePromptComment((item ?? {}) as Record<string, unknown>),
    );
  }
  return comment;
}

function parsePromptCommentAuthor(value: unknown): PromptCommentAuthor | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const record = value as Record<string, unknown>;
  return {
    id: Number(record.id ?? 0),
    username: String(record.username ?? ""),
    email: String(record.email ?? ""),
    avatar_url:
      typeof record.avatar_url === "string" && record.avatar_url !== ""
        ? record.avatar_url
        : undefined,
  };
}

function parsePublicPromptAuthor(value: unknown): PublicPromptAuthor | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const record = value as Record<string, unknown>;
  const id = Number(record.id ?? 0);
  if (!Number.isFinite(id) || id <= 0) {
    return null;
  }
  return {
    id,
    username: String(record.username ?? ""),
    avatar_url:
      typeof record.avatar_url === "string" && record.avatar_url.length > 0
        ? String(record.avatar_url)
        : undefined,
    headline:
      typeof record.headline === "string" ? record.headline : undefined,
    bio: typeof record.bio === "string" ? record.bio : undefined,
    location:
      typeof record.location === "string" ? record.location : undefined,
    website:
      typeof record.website === "string" ? record.website : undefined,
    banner_url:
      typeof record.banner_url === "string" ? record.banner_url : undefined,
  };
}
