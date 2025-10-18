export type RuntimeMode = "online" | "local";

const MODE_STORAGE_KEY = "promptgen:app-mode";

export const LOCAL_MODE: RuntimeMode = "local";
export const ONLINE_MODE: RuntimeMode = "online";

function accessStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage;
}

export function getStoredMode(): RuntimeMode {
  const storage = accessStorage();
  if (!storage) {
    return ONLINE_MODE;
  }
  const raw = storage.getItem(MODE_STORAGE_KEY);
  return raw === LOCAL_MODE ? LOCAL_MODE : ONLINE_MODE;
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
