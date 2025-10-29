/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 01:15:08
 * @FilePath: \electron-go-app\main.js
 * @LastEditTime: 2025-10-16 22:47:42
 */
const {
    app,
    BrowserWindow,
    Menu,
    ipcMain,
    globalShortcut,
    Tray,
    nativeImage
} = require("electron");
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
let mainWindow;
let tray;
let isClosePromptVisible = false;
let hasShownTrayBalloon = false;
let pendingCloseResolver = null;

const CLOSE_BEHAVIOR = {
    ASK: "ask",
    TRAY: "tray",
    QUIT: "quit"
};

const CLOSE_BEHAVIOR_LABEL = {
    [CLOSE_BEHAVIOR.ASK]: "关闭行为：每次询问",
    [CLOSE_BEHAVIOR.TRAY]: "关闭行为：最小化到托盘",
    [CLOSE_BEHAVIOR.QUIT]: "关闭行为：直接退出"
};

const CLOSE_PROMPT_REQUEST_CHANNEL = "window:close-prompt";
const CLOSE_PROMPT_DECISION_CHANNEL = "window:close-decision";

const preferencesFilePath = (() => {
    try {
        return path.join(app.getPath("userData"), "promptgen-preferences.json");
    } catch (error) {
        console.warn("[preferences] resolve path failed:", error);
        return null;
    }
})();

const loadPreferencesFromDisk = () => {
    if (!preferencesFilePath) {
        return {};
    }
    try {
        if (!fs.existsSync(preferencesFilePath)) {
            return {};
        }
        const raw = fs.readFileSync(preferencesFilePath, "utf-8");
        return raw ? JSON.parse(raw) : {};
    } catch (error) {
        console.warn("[preferences] load failed:", error);
        return {};
    }
};

const persistPreferencesToDisk = (next) => {
    if (!preferencesFilePath) {
        return;
    }
    try {
        fs.mkdirSync(path.dirname(preferencesFilePath), { recursive: true });
        fs.writeFileSync(preferencesFilePath, JSON.stringify(next, null, 2), "utf-8");
    } catch (error) {
        console.warn("[preferences] save failed:", error);
    }
};

const preferenceCache = loadPreferencesFromDisk();

const readCloseBehaviorPreference = () => {
    const stored = preferenceCache.closeBehavior;
    if (stored === CLOSE_BEHAVIOR.TRAY || stored === CLOSE_BEHAVIOR.QUIT) {
        return stored;
    }
    return CLOSE_BEHAVIOR.ASK;
};

let closeBehaviorPreference = readCloseBehaviorPreference();

function setCloseBehaviorPreference(nextBehavior) {
    if (
        nextBehavior !== CLOSE_BEHAVIOR.ASK &&
        nextBehavior !== CLOSE_BEHAVIOR.TRAY &&
        nextBehavior !== CLOSE_BEHAVIOR.QUIT
    ) {
        return;
    }
    closeBehaviorPreference = nextBehavior;
    preferenceCache.closeBehavior = nextBehavior;
    persistPreferencesToDisk(preferenceCache);
    refreshTrayMenu();
}

const resolveTrayIcon = () => {
    const candidates = [];
    if (process.platform === "win32") {
        candidates.push(
            app.isPackaged
                ? path.join(process.resourcesPath, "build", "favicon-256x256.ico")
                : path.join(__dirname, "build", "favicon-256x256.ico")
        );
    }
    candidates.push(
        app.isPackaged
            ? path.join(process.resourcesPath, "app", "frontend", "dist", "favicon.ico")
            : path.join(__dirname, "frontend", "public", "favicon.ico")
    );

    for (const candidate of candidates) {
        if (candidate && fs.existsSync(candidate)) {
            const image = nativeImage.createFromPath(candidate);
            if (!image.isEmpty()) {
                return image;
            }
        }
    }
    return null;
};

const showMainWindow = () => {
    if (!mainWindow || mainWindow.isDestroyed()) {
        mainWindow = createMainWindow();
    }
    if (mainWindow.isMinimized()) {
        mainWindow.restore();
    }
    if (!mainWindow.isVisible()) {
        mainWindow.show();
    }
    mainWindow.focus();
};

function refreshTrayMenu() {
    if (!tray) {
        return;
    }
    const versionLabel = `版本 ${app.getVersion()}`;
    const closeBehaviorLabel =
        CLOSE_BEHAVIOR_LABEL[closeBehaviorPreference] ?? CLOSE_BEHAVIOR_LABEL[CLOSE_BEHAVIOR.ASK];

    const menuTemplate = [
        { label: versionLabel, enabled: false },
        { type: "separator" },
        {
            label: closeBehaviorLabel,
            enabled: false
        },
        {
            label: "选择关闭行为",
            submenu: [
                {
                    label: "每次询问",
                    type: "radio",
                    checked: closeBehaviorPreference === CLOSE_BEHAVIOR.ASK,
                    click: () => {
                        setCloseBehaviorPreference(CLOSE_BEHAVIOR.ASK);
                    }
                },
                {
                    label: "最小化到托盘",
                    type: "radio",
                    checked: closeBehaviorPreference === CLOSE_BEHAVIOR.TRAY,
                    click: () => {
                        setCloseBehaviorPreference(CLOSE_BEHAVIOR.TRAY);
                    }
                },
                {
                    label: "直接退出",
                    type: "radio",
                    checked: closeBehaviorPreference === CLOSE_BEHAVIOR.QUIT,
                    click: () => {
                        setCloseBehaviorPreference(CLOSE_BEHAVIOR.QUIT);
                    }
                }
            ]
        },
        {
            label: "显示主窗口",
            click: () => {
                showMainWindow();
            }
        },
        { type: "separator" },
        {
            label: "退出 Prompt Gen",
            click: () => {
                app.isQuiting = true;
                if (backendProcess) {
                    backendProcess.kill();
                }
                app.quit();
            }
        }
    ];

    const contextMenu = Menu.buildFromTemplate(menuTemplate);
    tray.setContextMenu(contextMenu);
}

const ensureTray = () => {
    if (tray && !tray.isDestroyed?.()) {
        return tray;
    }
    const icon = resolveTrayIcon();
    try {
        tray = new Tray(icon || nativeImage.createEmpty());
    } catch (error) {
        console.warn("[tray] init failed:", error);
        return null;
    }
    tray.setToolTip("Prompt Gen");
    tray.on("double-click", () => {
        showMainWindow();
    });
    tray.on("click", () => {
        showMainWindow();
    });
    refreshTrayMenu();
    return tray;
};

const hideWindowToTray = (win) => {
    ensureTray();
    if (!win) {
        return;
    }
    win.hide();
    if (
        process.platform === "win32" &&
        typeof tray?.displayBalloon === "function" &&
        !hasShownTrayBalloon
    ) {
        tray.displayBalloon({
            title: "Prompt Gen",
            content: "应用已最小化到系统托盘。",
            iconType: "info"
        });
        hasShownTrayBalloon = true;
    }
};

const promptCloseBehavior = (win) => {
    if (!win || win.isDestroyed()) {
        return Promise.resolve({
            behavior: CLOSE_BEHAVIOR.ASK,
            remember: false,
            cancelled: true
        });
    }
    if (pendingCloseResolver) {
        return Promise.resolve({
            behavior: CLOSE_BEHAVIOR.ASK,
            remember: false,
            cancelled: true
        });
    }

    return new Promise((resolve) => {
        pendingCloseResolver = resolve;
        const suggested =
            closeBehaviorPreference === CLOSE_BEHAVIOR.QUIT
                ? CLOSE_BEHAVIOR.QUIT
                : CLOSE_BEHAVIOR.TRAY;
        win.webContents.send(CLOSE_PROMPT_REQUEST_CHANNEL, {
            defaultBehavior: suggested
        });
    });
};

const handleMainWindowClose = (event, win) => {
    if (!win || win.isDestroyed()) {
        return;
    }
    if (app.isQuiting) {
        return;
    }

    const behavior = closeBehaviorPreference;
    if (behavior === CLOSE_BEHAVIOR.TRAY) {
        event.preventDefault();
        hideWindowToTray(win);
        return;
    }
    if (behavior === CLOSE_BEHAVIOR.QUIT) {
        app.isQuiting = true;
        return;
    }

    if (isClosePromptVisible) {
        event.preventDefault();
        return;
    }

    event.preventDefault();
    isClosePromptVisible = true;
    promptCloseBehavior(win)
        .then((result) => {
            if (!result || result.cancelled) {
                return;
            }
            if (result.remember) {
                setCloseBehaviorPreference(result.behavior);
            }
            if (result.behavior === CLOSE_BEHAVIOR.TRAY) {
                hideWindowToTray(win);
                return;
            }
            if (result.behavior === CLOSE_BEHAVIOR.QUIT) {
                app.isQuiting = true;
                win.close();
            }
        })
        .finally(() => {
            isClosePromptVisible = false;
        });
};

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

ipcMain.on(CLOSE_PROMPT_DECISION_CHANNEL, (_event, payload) => {
    if (!pendingCloseResolver) {
        return;
    }
    const incoming = payload && typeof payload === "object" ? payload : {};
    const candidateBehavior =
        incoming.behavior === CLOSE_BEHAVIOR.TRAY || incoming.behavior === CLOSE_BEHAVIOR.QUIT
            ? incoming.behavior
            : CLOSE_BEHAVIOR.ASK;
    const resolved = {
        behavior: candidateBehavior,
        remember: Boolean(incoming.remember),
        cancelled: Boolean(incoming.cancelled)
    };
    if (resolved.cancelled) {
        resolved.behavior = CLOSE_BEHAVIOR.ASK;
        resolved.remember = false;
    }
    pendingCloseResolver(resolved);
    pendingCloseResolver = null;
});

function createMainWindow() {
    if (mainWindow && !mainWindow.isDestroyed()) {
        return mainWindow;
    }

    mainWindow = new BrowserWindow({
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
    mainWindow.on("close", (event) => {
        handleMainWindowClose(event, mainWindow);
    });
    mainWindow.on("closed", () => {
        mainWindow = null;
        pendingCloseResolver = null;
        isClosePromptVisible = false;
    });

    return mainWindow;
}

app.whenReady().then(() => {
    startBackend();
    Menu.setApplicationMenu(null);
    createMainWindow();
    ensureTray();

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
