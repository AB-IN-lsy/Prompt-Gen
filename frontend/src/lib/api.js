import { ApiError } from "./errors";
import { http, normaliseError } from "./http";
/**
 * Fetches keywords from the backend. The backend supports optional topic/polarity filters.
 * Errors are normalised to {@link ApiError} before being rethrown.
 */
export async function fetchKeywords(filters = {}) {
    try {
        const response = await http.get("/keywords", { params: filters });
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Generates fresh keyword suggestions using the LLM-powered backend helper. */
export async function generateKeywordSuggestions(payload) {
    try {
        const response = await http.post("/keywords/generate", payload);
        return response.data.suggestions;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Creates a new keyword and returns the persisted entity. */
export async function createKeyword(input) {
    try {
        const response = await http.post("/keywords", input);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Updates an existing keyword (identified by `id`). */
export async function updateKeyword(id, input) {
    if (!id) {
        throw new ApiError({ message: "Keyword id is required" });
    }
    try {
        const response = await http.patch(`/keywords/${id}`, input);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Soft deletes a keyword. */
export async function deleteKeyword(id) {
    if (!id) {
        throw new ApiError({ message: "Keyword id is required" });
    }
    try {
        await http.delete(`/keywords/${id}`);
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/**
 * Calls the prompt regeneration endpoint to produce a refined prompt using the selected
 * keywords and chosen model.
 */
export async function regeneratePrompt(payload) {
    try {
        const response = await http.post("/prompts/regenerate", payload);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Persists a prompt payload as a draft. */
export async function saveDraft(payload) {
    try {
        await http.post("/prompts", { ...payload, status: payload.status ?? "draft" });
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Fetches a paginated prompt list for the “我的 Prompt”页面. */
export async function fetchPrompts(params = {}) {
    try {
        const response = await http.get("/prompts", { params });
        return { items: response.data.items, total: response.data.total, meta: response.meta };
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Fetches a single prompt by ID, including its versions when supported by the backend. */
export async function fetchPromptById(id) {
    if (!id) {
        throw new ApiError({ message: "Prompt id is required" });
    }
    try {
        const response = await http.get(`/prompts/${id}`);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Updates a prompt in place. */
export async function updatePrompt(id, payload) {
    if (!id) {
        throw new ApiError({ message: "Prompt id is required" });
    }
    try {
        await http.patch(`/prompts/${id}`, payload);
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** Publishes a prompt (transitioning it from draft to published). */
export async function publishPrompt(id) {
    if (!id) {
        throw new ApiError({ message: "Prompt id is required" });
    }
    try {
        await http.post(`/prompts/${id}/publish`);
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 提交登录请求，返回用户信息与令牌。 */
export async function login(payload) {
    try {
        const response = await http.post("/auth/login", payload);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 注册新用户，成功后立即返回用户信息与令牌。 */
export async function register(payload) {
    try {
        const response = await http.post("/auth/register", payload);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 上传头像文件并返回可公开访问的 URL。 */
export async function uploadAvatar(file) {
    const formData = new FormData();
    formData.append("avatar", file);
    try {
        const response = await http.post("/uploads/avatar", formData, {
            headers: { "Content-Type": "multipart/form-data" },
        });
        return response.data.avatar_url;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 刷新 access token，通常由 http 拦截器内部调用。 */
export async function refreshTokens(refreshToken) {
    try {
        const response = await http.post("/auth/refresh", {
            refresh_token: refreshToken,
        });
        return response.data.tokens;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 注销当前刷新令牌。 */
export async function logout(refreshToken) {
    try {
        await http.post("/auth/logout", { refresh_token: refreshToken });
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 获取当前登录用户资料及设置。 */
export async function fetchCurrentUser() {
    try {
        const response = await http.get("/users/me");
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 更新当前登录用户的基础资料和设置。 */
export async function updateCurrentUser(payload) {
    try {
        const response = await http.put("/users/me", payload);
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/** 创建图形验证码并返回 base64 图片与标识。 */
export async function fetchCaptcha() {
    try {
        const response = await http.get("/auth/captcha");
        return response.data;
    }
    catch (error) {
        throw normaliseError(error);
    }
}
/**
 * Exposes helper utilities so other modules (e.g. login form) can set or clear the token
 * pair returned by the backend authentication endpoints.
 */
export { clearTokenPair as clearAuthTokens, getTokenPair as getAuthTokens, setTokenPair as setAuthTokens } from "./tokenStorage";
