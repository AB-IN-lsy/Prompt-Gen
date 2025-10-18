export type RuntimeMode = "online" | "local";

const MODE_STORAGE_KEY = "promptgen:app-mode";
export const LOCAL_MODE: RuntimeMode = "local";
export const ONLINE_MODE: RuntimeMode = "online";

const envDefaultMode =
  (import.meta.env?.VITE_DEFAULT_RUNTIME_MODE ?? "").toString().toLowerCase() ===
  LOCAL_MODE
    ? LOCAL_MODE
    : ONLINE_MODE;

function inferFallbackMode(): RuntimeMode {
  if (
    typeof window !== "undefined" &&
    window.location?.protocol === "file:"
  ) {
    return LOCAL_MODE;
  }
  return envDefaultMode === LOCAL_MODE ? LOCAL_MODE : ONLINE_MODE;
}

function accessStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage;
}

export function getStoredMode(): RuntimeMode {
  const storage = accessStorage();
  if (!storage) {
    return inferFallbackMode();
  }
  const raw = storage.getItem(MODE_STORAGE_KEY);
  if (raw === LOCAL_MODE || raw === ONLINE_MODE) {
    return raw;
  }
  const fallback = inferFallbackMode();
  storage.setItem(MODE_STORAGE_KEY, fallback);
  return fallback;
}

export function setStoredMode(mode: RuntimeMode): void {
  const storage = accessStorage();
  if (!storage) {
    return;
  }
  storage.setItem(MODE_STORAGE_KEY, mode);
}

export function resetStoredMode(): void {
  const storage = accessStorage();
  if (!storage) {
    return;
  }
  storage.removeItem(MODE_STORAGE_KEY);
}

export function isLocalMode(): boolean {
  return getStoredMode() === LOCAL_MODE;
}
