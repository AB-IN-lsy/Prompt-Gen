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
 *  - Base URL derived from `VITE_API_BASE_URL` (fallback `/api`).
 *  - Automatic `Authorization` header injection using the stored access token.
 *  - Envelope-aware response handling (unwraps `{ success, data, meta }`).
 *  - Normalised error mapping to `ApiError`.
 *  - 401 recovery via refresh token rotation with de-duplicated concurrent refreshes.
 */

import axios, {
  AxiosError,
  AxiosInstance,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from "axios";
import { ApiError, ErrorPayload } from "./errors";
import { clearTokenPair, getTokenPair, isAccessTokenExpired, setTokenPair } from "./tokenStorage";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api";
const REQUEST_TIMEOUT = 8_000;

interface EnvelopeSuccess<T> {
  success: true;
  data: T;
  meta?: unknown;
}

interface EnvelopeFailure {
  success: false;
  error?: ErrorPayload;
}

type Envelope<T> = EnvelopeSuccess<T> | EnvelopeFailure;

interface RefreshResponse {
  tokens: {
    access_token: string;
    refresh_token: string;
    expires_in?: number;
  };
}

interface AxiosRequestConfigWithRetry extends InternalAxiosRequestConfig {
  /** Internal flag to avoid infinite retry loops during refresh. */
  _retry?: boolean;
}

// http 实例负责处理业务正常请求，默认走应用的统一 Base URL。
const http: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT,
  withCredentials: false,
});

const refreshHttp: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT,
});

// 标记是否正在刷新；通过队列串联并发的刷新动作，避免重复请求。
let isRefreshing = false;
let refreshQueue: Array<(token: string | null, error?: ApiError) => void> = [];

function queueRefreshSubscriber(cb: (token: string | null, error?: ApiError) => void): void {
  refreshQueue.push(cb);
}

function flushRefreshQueue(token: string | null, error?: ApiError): void {
  refreshQueue.forEach((cb) => cb(token, error));
  refreshQueue = [];
}

async function performRefresh(): Promise<string | null> {
  const cached = getTokenPair();
  if (!cached?.refreshToken) {
    return null;
  }

  try {
    // 通过刷新接口获取新的 token 对，并在成功后更新本地缓存。
    const response = await refreshHttp.post<Envelope<RefreshResponse>>("/auth/refresh", {
      refresh_token: cached.refreshToken,
    });
    const { data } = unwrapResponse(response);
    const tokens = data.tokens;
    setTokenPair(tokens.access_token, tokens.refresh_token, tokens.expires_in);
    return tokens.access_token;
  } catch (error) {
    console.warn("Token refresh failed", error);
    clearTokenPair();
    throw normaliseError(error);
  }
}

function setAuthorizationHeader(config: InternalAxiosRequestConfig, token: string): InternalAxiosRequestConfig {
  if (!token) {
    return config;
  }

  // Axios v1 对 headers 做了封装，这里兼容两种写法。
  if (config.headers && typeof (config.headers as any).set === "function") {
    (config.headers as any).set("Authorization", `Bearer ${token}`);
  } else {
    config.headers = {
      ...(config.headers ?? {}),
      Authorization: `Bearer ${token}`,
    } as any;
  }

  return config;
}

function attachAuthorization(config: InternalAxiosRequestConfig): InternalAxiosRequestConfig {
  const cached = getTokenPair();
  if (cached?.accessToken) {
    return setAuthorizationHeader(config, cached.accessToken);
  }
  return config;
}

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const cached = getTokenPair();
  if (cached && isAccessTokenExpired(cached)) {
    // 如果访问令牌已过期，则优先刷新，避免发起失败请求。
    return handlePreemptiveRefresh(config);
  }
  return attachAuthorization(config);
});

async function handlePreemptiveRefresh(config: InternalAxiosRequestConfig): Promise<InternalAxiosRequestConfig> {
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
    .catch((error: ApiError) => {
      // 刷新失败时要通知队列里的所有等待者，便于它们走失败逻辑。
      flushRefreshQueue(null, error);
      throw error;
    })
    .finally(() => {
      isRefreshing = false;
    });
}

http.interceptors.response.use(
  (response) => unwrapResponse(response),
  async (error: AxiosError) => {
    if (error.config) {
      (error.config as AxiosRequestConfigWithRetry)._retry = (error.config as AxiosRequestConfigWithRetry)._retry ?? false;
    }

    if (shouldAttemptRefresh(error)) {
      return retryWithRefresh(error);
    }

    throw normaliseError(error);
  },
);

function shouldAttemptRefresh(error: AxiosError): boolean {
  const status = error.response?.status;
  if (status !== 401) {
    return false;
  }

  const config = error.config as AxiosRequestConfigWithRetry | undefined;
  if (!config || config._retry) {
    return false;
  }

  const cached = getTokenPair();
  if (!cached?.refreshToken) {
    return false;
  }

  return true;
}

async function retryWithRefresh(error: AxiosError): Promise<any> {
  const originalConfig = error.config as AxiosRequestConfigWithRetry;

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
  } catch (refreshError) {
    const apiError = refreshError instanceof ApiError ? refreshError : normaliseError(refreshError);
    flushRefreshQueue(null, apiError);
    throw apiError;
  } finally {
    isRefreshing = false;
  }
}

function unwrapResponse<T>(response: AxiosResponse<Envelope<T>>): AxiosResponse<T> {
  const payload = response.data;
  if (payload && typeof payload === "object" && "success" in payload) {
    if (payload.success) {
      // 将后端返回的 meta 附着到响应对象上，方便调用方读取分页等信息。
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (response as any).meta = payload.meta;
      (response as AxiosResponse<T>).data = payload.data;
      return response as AxiosResponse<T>;
    }

    throw new ApiError({
      status: response.status,
      code: payload.error?.code,
      message: payload.error?.message,
      details: payload.error?.details,
    });
  }

  // Non-envelope responses (should be rare) are passed through untouched.
  return response as AxiosResponse<T>;
}

function normaliseError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error;
  }

  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<EnvelopeFailure>;
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
