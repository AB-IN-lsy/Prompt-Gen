/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 01:15:08
 * @FilePath: \electron-go-app\main.js
 * @LastEditTime: 2025-10-16 22:47:42
 */
const { app, BrowserWindow, Menu, ipcMain, globalShortcut } = require("electron");
const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");
const dotenv = require("dotenv");

function loadEnvFiles() {
    const baseDir = app.isPackaged
        ? path.join(process.resourcesPath, "app")
        : __dirname;
    const candidates = [".env.local", ".env"];
    for (const file of candidates) {
        const target = path.join(baseDir, file);
        if (fs.existsSync(target)) {
            dotenv.config({ path: target, override: false });
        }
    }
}

loadEnvFiles();

app.commandLine.appendSwitch("allow-file-access-from-files");

let backendProcess;

const DEVTOOLS_SHORTCUT = "CommandOrControl+Shift+I";
const enableDevTools =
    String(process.env.APP_ENABLE_DEVTOOLS ?? "").toLowerCase() === "true";
const forceDist = String(process.env.ELECTRON_FORCE_DIST ?? "").trim() === "1";
console.log(
    "[runtime]",
    JSON.stringify({
        packaged: app.isPackaged,
        forceDist,
        enableDevTools,
        nodeEnv: process.env.NODE_ENV ?? null
    })
);

const resolveBackendPath = () => {
    const binaryName = process.platform === "win32" ? "server.exe" : "server";
    const baseDir = app.isPackaged
        ? path.join(process.resourcesPath, "server")
        : path.join(__dirname, "backend", "bin");
    const candidates = [
        path.join(baseDir, binaryName),
        path.join(baseDir, "server")
    ];
    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }
    throw new Error(`backend binary not found in ${baseDir}`);
};

const startBackend = () => {
    if (backendProcess) {
        return;
    }
    let backendPath;
    try {
        backendPath = resolveBackendPath();
    } catch (error) {
        console.error("[backend] resolve failed:", error);
        return;
    }
    const cwd = app.isPackaged
        ? path.join(process.resourcesPath, "app")
        : __dirname;
    const spawnOptions = {
        env: { ...process.env },
        cwd,
        stdio: app.isPackaged ? "ignore" : "inherit",
        windowsHide: app.isPackaged,
    };
    backendProcess = spawn(backendPath, spawnOptions);
    if (app.isPackaged) {
        backendProcess.unref();
    }

    backendProcess.on("exit", (code, signal) => {
        backendProcess = null;
        if (!app.isQuiting) {
            console.warn(`[backend] exited (code=${code}, signal=${signal})`);
        }
    });
};

const isDev = !app.isPackaged && !forceDist;
console.log("[runtime]", "isDev=", isDev);
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

ipcMain.handle("window:execute-edit-command", (event, command) => {
    const webContents = event.sender;
    if (!webContents || typeof command !== "string") {
        return;
    }
    switch (command) {
        case "undo":
            webContents.undo();
            break;
        case "redo":
            webContents.redo();
            break;
        case "cut":
            webContents.cut();
            break;
        case "copy":
            webContents.copy();
            break;
        case "paste":
            webContents.paste();
            break;
        case "delete":
            webContents.delete();
            break;
        case "selectAll":
            webContents.selectAll();
            break;
        case "reload":
            webContents.reload();
            break;
        default:
            break;
    }
});

function createMainWindow() {
    const mainWindow = new BrowserWindow({
        width: DEFAULT_WINDOW.width,
        height: DEFAULT_WINDOW.height,
        show: false,
        backgroundColor: "#f8f9fa",
        frame: false,
        resizable: process.platform === "darwin",
        maximizable: true,
        fullscreenable: process.platform === "darwin",
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
        if (process.platform !== "darwin") {
            mainWindow.setAspectRatio(DEFAULT_WINDOW.width / DEFAULT_WINDOW.height);
        }
        emitWindowState(mainWindow);
        if (enableDevTools && !mainWindow.webContents.isDevToolsOpened()) {
            mainWindow.webContents.openDevTools({ mode: "detach" });
        }
    });

    mainWindow.webContents.once("did-finish-load", () => {
        mainWindow.webContents
            .executeJavaScript(
                "console.info('[renderer] root innerHTML', document.getElementById('root')?.innerHTML?.length ?? 0)"
            )
            .catch((error) => {
                console.error("[renderer] eval failed", error);
            });
    });

    mainWindow.webContents.on("did-fail-load", (_event, errorCode, errorDescription, validatedURL) => {
        console.error("[renderer] did-fail-load", { errorCode, errorDescription, validatedURL });
    });

    mainWindow.on("maximize", () => emitWindowState(mainWindow));
    mainWindow.on("unmaximize", () => emitWindowState(mainWindow));
    mainWindow.on("focus", () => emitWindowState(mainWindow));

    mainWindow.setMenuBarVisibility(false);

    return mainWindow;
}

app.whenReady().then(() => {
    startBackend();
    Menu.setApplicationMenu(null);
    createMainWindow();

    if (enableDevTools) {
        const registered = globalShortcut.register(DEVTOOLS_SHORTCUT, () => {
            const focused = BrowserWindow.getFocusedWindow();
            if (!focused) {
                return;
            }
            if (focused.webContents.isDevToolsOpened()) {
                focused.webContents.closeDevTools();
            } else {
                focused.webContents.openDevTools({ mode: "detach" });
            }
        });
        if (!registered) {
            console.warn("[devtools] failed to register shortcut", DEVTOOLS_SHORTCUT);
        }
    }

    app.on("activate", () => {
        if (BrowserWindow.getAllWindows().length === 0) {
            createMainWindow();
        }
    });
});

app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
        if (backendProcess) {
            backendProcess.kill();
        }
        app.quit();
    }
});

app.on("will-quit", () => {
    if (enableDevTools) {
        globalShortcut.unregister(DEVTOOLS_SHORTCUT);
    }
    globalShortcut.unregisterAll();
});
