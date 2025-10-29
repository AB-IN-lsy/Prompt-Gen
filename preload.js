const { contextBridge, ipcRenderer } = require("electron");

const WINDOW_STATE_CHANNEL = "window:state";
const CLOSE_PROMPT_CHANNEL = "window:close-prompt";
const CLOSE_PROMPT_DECISION_CHANNEL = "window:close-decision";

const desktopApi = {
    minimize: () => ipcRenderer.invoke("window:minimize"),
    toggleMaximize: () => ipcRenderer.invoke("window:toggle-maximize"),
    close: () => ipcRenderer.invoke("window:close"),
    toggleAlwaysOnTop: () => ipcRenderer.invoke("window:toggle-always-on-top"),
    getWindowState: () => ipcRenderer.invoke("window:get-state"),
    onWindowState: (callback) => {
        if (typeof callback !== "function") {
            return () => undefined;
        }
        const wrapped = (_event, state) => callback(state);
        ipcRenderer.on(WINDOW_STATE_CHANNEL, wrapped);
        return () => {
            ipcRenderer.removeListener(WINDOW_STATE_CHANNEL, wrapped);
        };
    },
    executeEditCommand: (command) => ipcRenderer.invoke("window:execute-edit-command", command),
    onClosePrompt: (callback) => {
        if (typeof callback !== "function") {
            return () => undefined;
        }
        const wrapped = (_event, payload) => callback(payload ?? {});
        ipcRenderer.on(CLOSE_PROMPT_CHANNEL, wrapped);
        return () => {
            ipcRenderer.removeListener(CLOSE_PROMPT_CHANNEL, wrapped);
        };
    },
    submitCloseDecision: (decision) => {
        ipcRenderer.send(CLOSE_PROMPT_DECISION_CHANNEL, decision);
    }
};

contextBridge.exposeInMainWorld("desktop", desktopApi);
