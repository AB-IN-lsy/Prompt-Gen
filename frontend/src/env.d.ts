/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_KEYWORD_ROW_LIMIT?: string;
    readonly VITE_DEFAULT_KEYWORD_WEIGHT?: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}

type ElectronWindowState = {
    isMaximized: boolean;
    isAlwaysOnTop: boolean;
};

interface DesktopAPI {
    minimize(): Promise<void>;
    toggleMaximize(): Promise<ElectronWindowState>;
    close(): Promise<void>;
    toggleAlwaysOnTop(): Promise<ElectronWindowState>;
    getWindowState(): Promise<ElectronWindowState>;
    onWindowState(callback: (state: ElectronWindowState) => void): () => void;
}

declare global {
    interface Window {
        desktop?: DesktopAPI;
    }
}

export {};
