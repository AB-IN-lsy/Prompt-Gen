/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 01:15:08
 * @FilePath: \electron-go-app\main.js
 * @LastEditTime: 2025-10-10 01:15:11
 */
const { app, BrowserWindow, Menu, shell } = require("electron");
const path = require("path");

const isDev = process.env.NODE_ENV !== "production";
const DEV_SERVER_URL = process.env.VITE_DEV_SERVER_URL || "http://localhost:5173";

function buildAppMenu() {
    const template = [
        {
            label: process.platform === "darwin" ? "Prompt Gen" : "应用",
            submenu: [
                ...(process.platform === "darwin"
                    ? [
                          { role: "about", label: "关于" },
                          { type: "separator" }
                      ]
                    : []),
                { role: "quit", label: "退出" }
            ]
        },
        {
            label: "视图",
            submenu: [
                { role: "reload", label: "重新加载" },
                { role: "forceReload", label: "强制重新加载" },
                { type: "separator" },
                { role: "toggleDevTools", label: "切换开发者工具", visible: isDev }
            ]
        },
        {
            label: "窗口",
            submenu: [
                { role: "minimize", label: "最小化" },
                { role: "zoom", label: "缩放" },
                ...(process.platform === "darwin" ? [{ type: "separator" }, { role: "front", label: "全部置前" }] : [])
            ]
        },
        {
            label: "帮助",
            submenu: [
                {
                    label: "项目主页",
                    click: () => {
                        shell.openExternal("https://github.com/AB-IN-lsy/Prompt-Gen");
                    }
                }
            ]
        }
    ];

    const menu = Menu.buildFromTemplate(template);
    Menu.setApplicationMenu(menu);
}

function createMainWindow() {
    const mainWindow = new BrowserWindow({
        width: 1280,
        height: 800,
        show: true,
        backgroundColor: "#f8f9fa",
        webPreferences: {
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

    return mainWindow;
}

app.whenReady().then(() => {
    buildAppMenu();
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
