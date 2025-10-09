/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:54:57
 * @FilePath: \electron-go-app\frontend\src\hooks\useAuth.ts
 * @LastEditTime: 2025-10-10 00:05:27
 */
import { create } from "zustand";
import { fetchCurrentUser, logout as apiLogout, type AuthProfile, type AuthTokens } from "../lib/api";
import { clearTokenPair, getTokenPair, setTokenPair } from "../lib/tokenStorage";
import { normaliseError } from "../lib/http";
import { ApiError } from "../lib/errors";
import { toast } from "sonner";
import i18n from "../i18n";

interface AuthState {
  profile: AuthProfile | null;
  isInitialized: boolean;
  initializing: boolean;
  setProfile: (profile: AuthProfile | null) => void;
  authenticate: (tokens: AuthTokens) => Promise<void>;
  initialize: () => Promise<void>;
  logout: () => Promise<void>;
}

export const useAuth = create<AuthState>((set, get) => ({
  profile: null,
  isInitialized: false,
  initializing: false,
  setProfile: (profile) => set({ profile, isInitialized: true }),
  authenticate: async (tokens) => {
    setTokenPair(tokens.access_token, tokens.refresh_token, tokens.expires_in);
    try {
      const profile = await fetchCurrentUser();
      set({ profile, isInitialized: true });
    } catch (error) {
      const normalised = normaliseError(error);
      if (normalised instanceof ApiError && normalised.isUnauthorized) {
        clearTokenPair();
        set({ profile: null, isInitialized: true });
      } else {
        console.error("Failed to fetch profile after authentication", normalised);
      }
      throw normalised;
    }
  },
  initialize: async () => {
    const { isInitialized, initializing } = get();
    if (isInitialized || initializing) {
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
}));

export function useIsAuthenticated(): boolean {
  const profile = useAuth((state) => state.profile);
  return Boolean(profile && getTokenPair());
}
