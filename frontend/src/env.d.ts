/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_PROMPT_KEYWORD_LIMIT?: string;
    readonly VITE_PROMPT_KEYWORD_MAX_LENGTH?: string;
    readonly VITE_PROMPT_TAG_LIMIT?: string;
    readonly VITE_PROMPT_TAG_MAX_LENGTH?: string;
    readonly VITE_KEYWORD_ROW_LIMIT?: string;
    readonly VITE_DEFAULT_KEYWORD_WEIGHT?: string;
    readonly VITE_AI_GENERATE_MIN_DURATION_MS?: string;
    readonly VITE_PROMPT_AUTOSAVE_DELAY_MS?: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}

type ElectronWindowState = {
    isMaximized: boolean;
    isAlwaysOnTop: boolean;
};

type DesktopCloseBehavior = "tray" | "quit";

interface DesktopClosePromptPayload {
    defaultBehavior: DesktopCloseBehavior;
}

interface DesktopCloseDecisionPayload {
    behavior: DesktopCloseBehavior | "ask";
    remember?: boolean;
    cancelled?: boolean;
}

interface DesktopAPI {
    minimize(): Promise<void>;
    toggleMaximize(): Promise<ElectronWindowState>;
    close(): Promise<void>;
    getWindowState(): Promise<ElectronWindowState>;
    onWindowState(callback: (state: ElectronWindowState) => void): () => void;
    executeEditCommand(command: "undo" | "redo" | "cut" | "copy" | "paste" | "delete" | "selectAll" | "reload"): Promise<void>;
    onClosePrompt?(callback: (payload: DesktopClosePromptPayload) => void): () => void;
    submitCloseDecision?(decision: DesktopCloseDecisionPayload): void;
}

declare global {
    interface Window {
        desktop?: DesktopAPI;
    }
}

export {};
