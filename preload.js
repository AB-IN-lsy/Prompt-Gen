const { contextBridge, ipcRenderer } = require("electron");

const WINDOW_STATE_CHANNEL = "window:state";

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
    }
};

contextBridge.exposeInMainWorld("desktop", desktopApi);
