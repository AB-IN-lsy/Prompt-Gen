/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_PROMPT_KEYWORD_LIMIT?: string;
    readonly VITE_PROMPT_KEYWORD_MAX_LENGTH?: string;
    readonly VITE_PROMPT_TAG_LIMIT?: string;
    readonly VITE_PROMPT_TAG_MAX_LENGTH?: string;
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
    executeEditCommand(command: "undo" | "redo" | "cut" | "copy" | "paste" | "delete" | "selectAll" | "reload"): Promise<void>;
}

declare global {
    interface Window {
        desktop?: DesktopAPI;
    }
}

export {};
