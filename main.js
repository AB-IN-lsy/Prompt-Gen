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
    nativeImage,
    screen
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

const hasSingleInstanceLock = app.requestSingleInstanceLock();
if (!hasSingleInstanceLock) {
    app.quit();
} else {
    app.on("second-instance", () => {
        if (splashWindow && !splashWindow.isDestroyed()) {
            splashWindow.show();
        }
        showMainWindow();
    });
}

const APP_START_TIME = process.hrtime.bigint();

function logStartupMetric(label) {
    try {
        const now = process.hrtime.bigint();
        const durationMs = Number(now - APP_START_TIME) / 1e6;
        console.info(`[startup] ${label} +${durationMs.toFixed(1)}ms`);
    } catch (error) {
        console.warn("[startup] failed to log metric", label, error);
    }
}

app.commandLine.appendSwitch("allow-file-access-from-files");

let backendProcess;
let mainWindow;
let tray;
let isClosePromptVisible = false;
let hasShownTrayBalloon = false;
let pendingCloseResolver = null;
let splashWindow;
let splashVisibleAt = 0;

const SPLASH_MIN_VISIBLE_MS = 450;

const splashMarkup = `
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <title>PromptGen 正在启动…</title>
    <style>
      :root {
        color-scheme: light dark;
      }
      body {
        margin: 0;
        height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: "Inter", "PingFang SC", "Microsoft YaHei", sans-serif;
        background: radial-gradient(circle at top, rgba(59,130,246,0.35), transparent 55%), rgba(15,23,42,0.92);
        color: #f8fafc;
      }
      .card {
        backdrop-filter: blur(18px);
        background: rgba(15,23,42,0.72);
        border: 1px solid rgba(148,163,184,0.28);
        border-radius: 20px;
        padding: 36px 42px;
        box-shadow: 0 20px 65px -30px rgba(59,130,246,0.65);
        text-align: center;
        min-width: 320px;
      }
      h1 {
        margin: 0;
        font-weight: 600;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        font-size: 16px;
        color: rgba(148,163,184,0.88);
      }
      p {
        margin: 18px 0 0;
        font-size: 14px;
        color: rgba(226,232,240,0.9);
      }
      .spinner {
        margin: 28px auto 0;
        width: 44px;
        height: 44px;
        border-radius: 50%;
        border: 3px solid rgba(148,163,184,0.25);
        border-top-color: rgba(96,165,250,0.9);
        animation: spin 1s linear infinite;
      }
      @keyframes spin {
        to {
          transform: rotate(360deg);
        }
      }
    </style>
  </head>
  <body>
    <div class="card">
      <h1>PromptGen Desktop</h1>
      <div class="spinner"></div>
      <p>正在加载工作台，请稍候…</p>
    </div>
  </body>
</html>
`;

function destroySplashWindow() {
    if (!splashWindow || splashWindow.isDestroyed()) {
        splashWindow = null;
        return;
    }
    const reference = splashWindow;
    splashWindow = null;
    reference.close();
}

function scheduleSplashClose() {
    if (!splashWindow) {
        return;
    }
    const elapsed = Date.now() - splashVisibleAt;
    const remaining = Math.max(SPLASH_MIN_VISIBLE_MS - elapsed, 0);
    if (remaining <= 0) {
        destroySplashWindow();
        return;
    }
    setTimeout(() => destroySplashWindow(), remaining);
}

function createSplashWindow() {
    if (splashWindow && !splashWindow.isDestroyed()) {
        return splashWindow;
    }
    splashWindow = new BrowserWindow({
        width: 460,
        height: 320,
        frame: false,
        resizable: false,
        movable: true,
        transparent: true,
        show: false,
        alwaysOnTop: true,
        skipTaskbar: true,
        backgroundColor: "#00000000",
        webPreferences: {
            devTools: false
        }
    });

    splashWindow.loadURL(`data:text/html;charset=utf-8,${encodeURIComponent(splashMarkup)}`).catch((error) => {
        console.error("[splash] load failed", error);
    });

    splashWindow.once("ready-to-show", () => {
        if (!splashWindow) {
            return;
        }
        splashWindow.show();
        splashVisibleAt = Date.now();
    });

    splashWindow.on("closed", () => {
        splashWindow = null;
    });

    return splashWindow;
}

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
    emitWindowVisibility(mainWindow, true);
};

const toggleMainWindowVisibility = () => {
    if (!mainWindow || mainWindow.isDestroyed()) {
        showMainWindow();
        return;
    }
    if (!mainWindow.isVisible() || mainWindow.isMinimized()) {
        showMainWindow();
        return;
    }
    hideWindowToTray(mainWindow);
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
    emitWindowVisibility(win, false);
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
    logStartupMetric("backend process spawned");

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
const TOGGLE_VISIBILITY_SHORTCUT = process.platform === "darwin" ? "Command+Shift+M" : "Control+Shift+M";
const WINDOW_STATE_CHANNEL = "window:state";
const WINDOW_VISIBILITY_CHANNEL = "window:visibility";
const DEFAULT_WINDOW = {
    width: 1200,
    height: 720
};
const WINDOW_STATE_PREF_KEY = "windowState";
const WINDOW_STATE_SAVE_DELAY = 350;

let windowStateSaveTimer = null;
let lastKnownWindowBounds = null;

const coerceWindowBounds = (raw) => {
    if (!raw || typeof raw !== "object") {
        return null;
    }
    const width = Number(raw.width);
    const height = Number(raw.height);
    if (!Number.isFinite(width) || !Number.isFinite(height)) {
        return null;
    }
    const result = {
        width: Math.max(640, Math.round(width)),
        height: Math.max(480, Math.round(height)),
        isMaximized: Boolean(raw.isMaximized)
    };
    if (Number.isFinite(raw.x) && Number.isFinite(raw.y)) {
        result.x = Math.round(raw.x);
        result.y = Math.round(raw.y);
    }
    return result;
};

const ensureBoundsVisible = (state) => {
    if (!state || typeof state.x !== "number" || typeof state.y !== "number") {
        return state;
    }
    try {
        const displays = screen.getAllDisplays();
        if (!displays || displays.length === 0) {
            return state;
        }
        const rect = {
            x: state.x,
            y: state.y,
            width: state.width,
            height: state.height
        };
        const intersects = displays.some((display) => {
            const area = display.workArea || display.bounds;
            const horizontal =
                rect.x < area.x + area.width && rect.x + rect.width > area.x;
            const vertical =
                rect.y < area.y + area.height && rect.y + rect.height > area.y;
            return horizontal && vertical;
        });
        if (!intersects) {
            return { ...state, x: undefined, y: undefined };
        }
    } catch (error) {
        console.warn("[window] ensure bounds visible failed:", error);
    }
    return state;
};

const loadInitialWindowState = () => {
    const stored = coerceWindowBounds(preferenceCache[WINDOW_STATE_PREF_KEY]);
    const base = {
        width: DEFAULT_WINDOW.width,
        height: DEFAULT_WINDOW.height,
        isMaximized: false
    };
    if (!stored) {
        return base;
    }
    const merged = ensureBoundsVisible({
        ...base,
        ...stored
    });
    return {
        width: merged.width ?? base.width,
        height: merged.height ?? base.height,
        x: typeof merged.x === "number" ? merged.x : undefined,
        y: typeof merged.y === "number" ? merged.y : undefined,
        isMaximized: Boolean(merged.isMaximized)
    };
};

const rememberBoundsIfNeeded = (win) => {
    if (!win || win.isDestroyed()) {
        return;
    }
    if (win.isMaximized() || win.isMinimized()) {
        return;
    }
    lastKnownWindowBounds = win.getBounds();
};

const persistWindowState = (win, immediate = false) => {
    if (!win || win.isDestroyed()) {
        return;
    }
    const commit = () => {
        const isMaximized = win.isMaximized();
        let bounds = lastKnownWindowBounds;
        if (!bounds) {
            bounds = isMaximized ? win.getNormalBounds() : win.getBounds();
        }
        if (!bounds || !Number.isFinite(bounds.width) || !Number.isFinite(bounds.height)) {
            bounds = {
                width: DEFAULT_WINDOW.width,
                height: DEFAULT_WINDOW.height,
                x: undefined,
                y: undefined
            };
        }
        const payload = {
            width: bounds.width,
            height: bounds.height,
            x: typeof bounds.x === "number" ? bounds.x : undefined,
            y: typeof bounds.y === "number" ? bounds.y : undefined,
            isMaximized
        };
        preferenceCache[WINDOW_STATE_PREF_KEY] = payload;
        persistPreferencesToDisk(preferenceCache);
    };
    if (immediate) {
        commit();
        return;
    }
    if (windowStateSaveTimer) {
        clearTimeout(windowStateSaveTimer);
    }
    windowStateSaveTimer = setTimeout(commit, WINDOW_STATE_SAVE_DELAY);
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

const emitWindowVisibility = (win, visible) => {
    if (!win || win.isDestroyed()) {
        return;
    }
    win.webContents.send(WINDOW_VISIBILITY_CHANNEL, { visible: Boolean(visible) });
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

    const initialWindowState = loadInitialWindowState();
    const windowOptions = {
        width: initialWindowState.width,
        height: initialWindowState.height,
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
    };
    if (typeof initialWindowState.x === "number" && typeof initialWindowState.y === "number") {
        windowOptions.x = initialWindowState.x;
        windowOptions.y = initialWindowState.y;
    }

    mainWindow = new BrowserWindow(windowOptions);
    logStartupMetric("main window instantiated");

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
        logStartupMetric("main window ready-to-show");
        scheduleSplashClose();
        if (initialWindowState.isMaximized) {
            mainWindow.maximize();
            mainWindow.show();
        } else {
            mainWindow.show();
        }
        if (process.platform !== "darwin" && !mainWindow.isMaximized()) {
            mainWindow.setAspectRatio(DEFAULT_WINDOW.width / DEFAULT_WINDOW.height);
        }
        emitWindowState(mainWindow);
        emitWindowVisibility(mainWindow, true);
        if (enableDevTools && !mainWindow.webContents.isDevToolsOpened()) {
            mainWindow.webContents.openDevTools({ mode: "detach" });
        }
        lastKnownWindowBounds = initialWindowState.isMaximized
            ? mainWindow.getNormalBounds()
            : mainWindow.getBounds();
    });

    mainWindow.webContents.once("did-finish-load", () => {
        logStartupMetric("renderer did-finish-load");
        scheduleSplashClose();
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
        destroySplashWindow();
    });

    const handleBoundsChange = () => {
        rememberBoundsIfNeeded(mainWindow);
        persistWindowState(mainWindow);
    };

    mainWindow.on("move", handleBoundsChange);
    mainWindow.on("resize", handleBoundsChange);

    mainWindow.on("maximize", () => {
        emitWindowState(mainWindow);
        persistWindowState(mainWindow, true);
    });
    mainWindow.on("unmaximize", () => {
        emitWindowState(mainWindow);
        rememberBoundsIfNeeded(mainWindow);
        persistWindowState(mainWindow, true);
    });
    mainWindow.on("focus", () => emitWindowState(mainWindow));
    mainWindow.on("restore", () => {
        emitWindowState(mainWindow);
        emitWindowVisibility(mainWindow, true);
        rememberBoundsIfNeeded(mainWindow);
        persistWindowState(mainWindow, true);
    });
    mainWindow.on("minimize", () => {
        emitWindowVisibility(mainWindow, false);
        persistWindowState(mainWindow, true);
    });
    mainWindow.on("show", () => {
        emitWindowVisibility(mainWindow, true);
        rememberBoundsIfNeeded(mainWindow);
    });
    mainWindow.on("hide", () => emitWindowVisibility(mainWindow, false));

    mainWindow.setMenuBarVisibility(false);
    mainWindow.on("close", (event) => {
        handleMainWindowClose(event, mainWindow);
        persistWindowState(mainWindow, true);
    });
    mainWindow.on("closed", () => {
        mainWindow = null;
        pendingCloseResolver = null;
        isClosePromptVisible = false;
    });

    return mainWindow;
}

app.whenReady().then(() => {
    logStartupMetric("app.whenReady resolved");
    createSplashWindow();
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

    const toggleRegistered = globalShortcut.register(TOGGLE_VISIBILITY_SHORTCUT, () => {
        toggleMainWindowVisibility();
    });
    if (!toggleRegistered) {
        console.warn("[shortcut] failed to register", TOGGLE_VISIBILITY_SHORTCUT);
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
        destroySplashWindow();
        app.quit();
    }
});

app.on("will-quit", () => {
    if (enableDevTools) {
        globalShortcut.unregister(DEVTOOLS_SHORTCUT);
    }
    globalShortcut.unregisterAll();
});
