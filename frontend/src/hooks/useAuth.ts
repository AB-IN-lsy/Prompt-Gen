/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:54:57
 * @FilePath: \electron-go-app\frontend\src\hooks\useAuth.ts
 * @LastEditTime: 2025-10-10 00:05:27
 */
import { create } from "zustand";
import {
  fetchCurrentUser,
  logout as apiLogout,
  type AuthProfile,
  type AuthTokens,
} from "../lib/api";
import {
  clearTokenPair,
  getTokenPair,
  setTokenPair,
} from "../lib/tokenStorage";
import { normaliseError } from "../lib/http";
import { ApiError } from "../lib/errors";
import { toast } from "sonner";
import i18n from "../i18n";
import {
  getStoredMode,
  isLocalMode,
  LOCAL_MODE,
  resetStoredMode,
  setStoredMode,
} from "../lib/runtimeMode";

interface AuthState {
  profile: AuthProfile | null;
  isInitialized: boolean;
  initializing: boolean;
  setProfile: (profile: AuthProfile | null) => void;
  authenticate: (tokens: AuthTokens) => Promise<void>;
  initialize: () => Promise<void>;
  logout: () => Promise<void>;
  enterOffline: () => Promise<void>;
}

export const useAuth = create<AuthState>((set, get) => {
  const defaultMode = (
    import.meta.env.VITE_DEFAULT_RUNTIME_MODE ?? ""
  ).toLowerCase();
  if (defaultMode === LOCAL_MODE && getStoredMode() !== LOCAL_MODE) {
    setStoredMode(LOCAL_MODE);
  }

  return {
    profile: null,
    isInitialized: false,
    initializing: false,
    setProfile: (profile) => set({ profile, isInitialized: true }),
    authenticate: async (tokens) => {
      setTokenPair(
        tokens.access_token,
        tokens.refresh_token,
        tokens.expires_in,
      );
      try {
        const profile = await fetchCurrentUser();
        set({ profile, isInitialized: true });
      } catch (error) {
        const normalised = normaliseError(error);
        if (normalised instanceof ApiError && normalised.isUnauthorized) {
          clearTokenPair();
          set({ profile: null, isInitialized: true });
        } else {
          console.error(
            "Failed to fetch profile after authentication",
            normalised,
          );
        }
        throw normalised;
      }
    },
    initialize: async () => {
      const { isInitialized, initializing } = get();
      if (isInitialized || initializing) {
        return;
      }
      const mode = getStoredMode();
      if (mode === LOCAL_MODE) {
        set({ initializing: true });
        clearTokenPair();
        try {
          const profile = await fetchCurrentUser();
          set({ profile, initializing: false, isInitialized: true });
        } catch (error) {
          const normalised = normaliseError(error);
          console.error("Failed to bootstrap offline profile", normalised);
          resetStoredMode();
          set({ profile: null, initializing: false, isInitialized: true });
        }
        return;
      }
      if (!getTokenPair()) {
        set({ profile: null, isInitialized: true });
        return;
      }
      set({ initializing: true });
      try {
        const profile = await fetchCurrentUser();
        set({ profile });
      } catch (error) {
        const normalised = normaliseError(error);
        if (normalised instanceof ApiError && normalised.isUnauthorized) {
          clearTokenPair();
          set({ profile: null });
        } else {
          console.error("Failed to bootstrap auth", normalised);
        }
      } finally {
        set({ initializing: false, isInitialized: true });
      }
    },
    logout: async () => {
      const localMode = isLocalMode();
      if (localMode) {
        clearTokenPair();
        resetStoredMode();
        set({ profile: null, isInitialized: true });
        toast.success(i18n.t("appShell.logoutSuccess"));
        return;
      }
      const tokens = getTokenPair();
      try {
        if (tokens?.refreshToken) {
          await apiLogout(tokens.refreshToken);
        }
      } catch (error) {
        console.warn("Logout request failed", error);
      } finally {
        clearTokenPair();
        set({ profile: null, isInitialized: true });
        toast.success(i18n.t("appShell.logoutSuccess"));
      }
    },
    enterOffline: async () => {
      set({ initializing: true });
      clearTokenPair();
      setStoredMode(LOCAL_MODE);
      try {
        const profile = await fetchCurrentUser();
        set({ profile, initializing: false, isInitialized: true });
      } catch (error) {
        resetStoredMode();
        set({ profile: null, initializing: false, isInitialized: true });
        throw normaliseError(error);
      }
    },
  };
});

export function useIsAuthenticated(): boolean {
  const profile = useAuth((state) => state.profile);
  if (isLocalMode()) {
    return Boolean(profile);
  }
  return Boolean(profile && getTokenPair());
}
