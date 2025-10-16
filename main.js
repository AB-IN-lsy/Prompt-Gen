/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 01:15:08
 * @FilePath: \electron-go-app\main.js
 * @LastEditTime: 2025-10-16 22:47:42
 */
const { app, BrowserWindow, Menu, ipcMain } = require("electron");
const path = require("path");

const isDev = process.env.NODE_ENV !== "production";
const DEV_SERVER_URL = process.env.VITE_DEV_SERVER_URL || "http://localhost:5173";
const WINDOW_STATE_CHANNEL = "window:state";
const DEFAULT_WINDOW = {
    width: 1200,
    height: 720
};

const getWindowFromEvent = (event) => BrowserWindow.fromWebContents(event.sender);

const emitWindowState = (win) => {
    if (!win || win.isDestroyed()) {
        return;
    }
    win.webContents.send(WINDOW_STATE_CHANNEL, {
        isMaximized: win.isMaximized(),
        isAlwaysOnTop: win.isAlwaysOnTop()
    });
};

ipcMain.handle("window:get-state", (event) => {
    const win = getWindowFromEvent(event);
    if (!win) {
        return { isMaximized: false, isAlwaysOnTop: false };
    }
    return { isMaximized: win.isMaximized(), isAlwaysOnTop: win.isAlwaysOnTop() };
});

ipcMain.handle("window:minimize", (event) => {
    const win = getWindowFromEvent(event);
    if (!win) {
        return;
    }
    win.minimize();
});

ipcMain.handle("window:toggle-maximize", (event) => {
    const win = getWindowFromEvent(event);
    if (!win) {
        return { isMaximized: false, isAlwaysOnTop: false };
    }
    if (win.isMaximized()) {
        win.unmaximize();
    } else {
        win.maximize();
    }
    const next = { isMaximized: win.isMaximized(), isAlwaysOnTop: win.isAlwaysOnTop() };
    emitWindowState(win);
    return next;
});

ipcMain.handle("window:close", (event) => {
    const win = getWindowFromEvent(event);
    if (!win) {
        return;
    }
    win.close();
});

ipcMain.handle("window:toggle-always-on-top", (event) => {
    const win = getWindowFromEvent(event);
    if (!win) {
        return { isMaximized: false, isAlwaysOnTop: false };
    }
    const nextAlwaysOnTop = !win.isAlwaysOnTop();
    win.setAlwaysOnTop(nextAlwaysOnTop);
    const next = { isMaximized: win.isMaximized(), isAlwaysOnTop: nextAlwaysOnTop };
    emitWindowState(win);
    return next;
});

function createMainWindow() {
    const mainWindow = new BrowserWindow({
        width: DEFAULT_WINDOW.width,
        height: DEFAULT_WINDOW.height,
        show: false,
        backgroundColor: "#f8f9fa",
        frame: false,
        resizable: false,
        maximizable: true,
        titleBarStyle: process.platform === "darwin" ? "hiddenInset" : "hidden",
        titleBarOverlay: process.platform === "win32"
            ? { color: "#0f172a", symbolColor: "#ffffff", height: 36 }
            : undefined,
        webPreferences: {
            preload: path.join(__dirname, "preload.js"),
            nodeIntegration: false,
            contextIsolation: true
        }
    });

    if (isDev) {
        mainWindow.loadURL(DEV_SERVER_URL);
        if (process.env.ELECTRON_OPEN_DEVTOOLS === "1") {
            mainWindow.webContents.openDevTools({ mode: "detach" });
        }
    } else {
        const indexHtml = path.join(__dirname, "frontend", "dist", "index.html");
        mainWindow.loadFile(indexHtml);
    }

    mainWindow.once("ready-to-show", () => {
        mainWindow.show();
        mainWindow.setAspectRatio(DEFAULT_WINDOW.width / DEFAULT_WINDOW.height);
        emitWindowState(mainWindow);
    });

    mainWindow.on("maximize", () => emitWindowState(mainWindow));
    mainWindow.on("unmaximize", () => emitWindowState(mainWindow));
    mainWindow.on("focus", () => emitWindowState(mainWindow));

    mainWindow.setMenuBarVisibility(false);

    return mainWindow;
}

app.whenReady().then(() => {
    Menu.setApplicationMenu(null);
    createMainWindow();

    app.on("activate", () => {
        if (BrowserWindow.getAllWindows().length === 0) {
            createMainWindow();
        }
    });
});

app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
        app.quit();
    }
});
