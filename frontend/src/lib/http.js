/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:16:10
 * @FilePath: \electron-go-app\frontend\src\lib\http.ts
 * @LastEditTime: 2025-10-09 23:16:15
 */
/*
 * @fileoverview Axios instance pre-configured for the PromptGen backend contract.
 *
 * Features:
 *  - Base URL derived from `VITE_API_BASE_URL` (fallback `http://localhost:9090/api`).
 *  - Automatic `Authorization` header injection using the stored access token.
 *  - Envelope-aware response handling (unwraps `{ success, data, meta }`).
 *  - Normalised error mapping to `ApiError`.
 *  - 401 recovery via refresh token rotation with de-duplicated concurrent refreshes.
 */
import axios from "axios";
import { ApiError } from "./errors";
import { clearTokenPair, getTokenPair, isAccessTokenExpired, setTokenPair } from "./tokenStorage";
function resolveDefaultBaseUrl() {
    const fallback = "http://localhost:9090/api";
    const envBaseUrl = (import.meta.env?.VITE_API_BASE_URL ?? "");
    if (envBaseUrl.trim().length > 0) {
        return envBaseUrl;
    }
    if (typeof window === "undefined") {
        return fallback;
    }
    return fallback;
}
const API_BASE_URL = resolveDefaultBaseUrl();
const REQUEST_TIMEOUT = 8_000;
// http 实例负责处理业务正常请求，默认走应用的统一 Base URL。
const http = axios.create({
    baseURL: API_BASE_URL,
    timeout: REQUEST_TIMEOUT,
    withCredentials: false,
});
const refreshHttp = axios.create({
    baseURL: API_BASE_URL,
    timeout: REQUEST_TIMEOUT,
});
// 标记是否正在刷新；通过队列串联并发的刷新动作，避免重复请求。
let isRefreshing = false;
let refreshQueue = [];
function queueRefreshSubscriber(cb) {
    refreshQueue.push(cb);
}
function flushRefreshQueue(token, error) {
    refreshQueue.forEach((cb) => cb(token, error));
    refreshQueue = [];
}
async function performRefresh() {
    const cached = getTokenPair();
    if (!cached?.refreshToken) {
        return null;
    }
    try {
        // 通过刷新接口获取新的 token 对，并在成功后更新本地缓存。
        const response = await refreshHttp.post("/auth/refresh", {
            refresh_token: cached.refreshToken,
        });
        const { data } = unwrapResponse(response);
        const tokens = data.tokens;
        setTokenPair(tokens.access_token, tokens.refresh_token, tokens.expires_in);
        return tokens.access_token;
    }
    catch (error) {
        console.warn("Token refresh failed", error);
        clearTokenPair();
        throw normaliseError(error);
    }
}
function setAuthorizationHeader(config, token) {
    if (!token) {
        return config;
    }
    // Axios v1 对 headers 做了封装，这里兼容两种写法。
    if (config.headers && typeof config.headers.set === "function") {
        config.headers.set("Authorization", `Bearer ${token}`);
    }
    else {
        config.headers = {
            ...(config.headers ?? {}),
            Authorization: `Bearer ${token}`,
        };
    }
    return config;
}
function attachAuthorization(config) {
    const cached = getTokenPair();
    if (cached?.accessToken) {
        return setAuthorizationHeader(config, cached.accessToken);
    }
    return config;
}
http.interceptors.request.use((config) => {
    const cached = getTokenPair();
    if (cached && isAccessTokenExpired(cached)) {
        // 如果访问令牌已过期，则优先刷新，避免发起失败请求。
        return handlePreemptiveRefresh(config);
    }
    return attachAuthorization(config);
});
async function handlePreemptiveRefresh(config) {
    if (isRefreshing) {
        return new Promise((resolve, reject) => {
            queueRefreshSubscriber((token, error) => {
                if (error) {
                    reject(error);
                    return;
                }
                if (token) {
                    setAuthorizationHeader(config, token);
                }
                resolve(config);
            });
        });
    }
    isRefreshing = true;
    return performRefresh()
        .then((token) => {
        flushRefreshQueue(token ?? null);
        if (token) {
            setAuthorizationHeader(config, token);
        }
        return config;
    })
        .catch((error) => {
        // 刷新失败时要通知队列里的所有等待者，便于它们走失败逻辑。
        flushRefreshQueue(null, error);
        throw error;
    })
        .finally(() => {
        isRefreshing = false;
    });
}
http.interceptors.response.use((response) => unwrapResponse(response), async (error) => {
    if (error.config) {
        error.config._retry = error.config._retry ?? false;
    }
    if (shouldAttemptRefresh(error)) {
        return retryWithRefresh(error);
    }
    throw normaliseError(error);
});
function shouldAttemptRefresh(error) {
    const status = error.response?.status;
    if (status !== 401) {
        return false;
    }
    const config = error.config;
    if (!config || config._retry) {
        return false;
    }
    const cached = getTokenPair();
    if (!cached?.refreshToken) {
        return false;
    }
    return true;
}
async function retryWithRefresh(error) {
    const originalConfig = error.config;
    if (isRefreshing) {
        return new Promise((resolve, reject) => {
            queueRefreshSubscriber((token, refreshError) => {
                if (refreshError) {
                    reject(refreshError);
                    return;
                }
                if (!token) {
                    reject(normaliseError(error));
                    return;
                }
                originalConfig._retry = true;
                setAuthorizationHeader(originalConfig, token);
                resolve(http(originalConfig));
            });
        });
    }
    isRefreshing = true;
    try {
        const token = await performRefresh();
        flushRefreshQueue(token ?? null);
        if (!token) {
            throw normaliseError(error);
        }
        originalConfig._retry = true;
        setAuthorizationHeader(originalConfig, token);
        return http(originalConfig);
    }
    catch (refreshError) {
        const apiError = refreshError instanceof ApiError ? refreshError : normaliseError(refreshError);
        flushRefreshQueue(null, apiError);
        throw apiError;
    }
    finally {
        isRefreshing = false;
    }
}
function unwrapResponse(response) {
    const payload = response.data;
    if (payload && typeof payload === "object" && "success" in payload) {
        if (payload.success) {
            // 将后端返回的 meta 附着到响应对象上，方便调用方读取分页等信息。
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            response.meta = payload.meta;
            response.data = payload.data;
            return response;
        }
        throw new ApiError({
            status: response.status,
            code: payload.error?.code,
            message: payload.error?.message,
            details: payload.error?.details,
        });
    }
    // Non-envelope responses (should be rare) are passed through untouched.
    return response;
}
function normaliseError(error) {
    if (error instanceof ApiError) {
        return error;
    }
    if (axios.isAxiosError(error)) {
        const axiosError = error;
        const status = axiosError.response?.status;
        const payload = axiosError.response?.data;
        const message = payload?.error?.message ?? axiosError.message ?? "Network error";
        return new ApiError({
            status,
            code: payload?.error?.code,
            message,
            details: payload?.error?.details,
            cause: error,
        });
    }
    return new ApiError({
        message: error instanceof Error ? error.message : "Unknown error",
        cause: error,
    });
}
export { http, normaliseError };
