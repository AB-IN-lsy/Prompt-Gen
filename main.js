/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 01:15:08
 * @FilePath: \electron-go-app\main.js
 * @LastEditTime: 2025-10-10 01:15:11
 */
const { app, BrowserWindow } = require("electron");
const path = require("path");

const isDev = process.env.NODE_ENV !== "production";
const DEV_SERVER_URL = process.env.VITE_DEV_SERVER_URL || "http://localhost:5173";

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
        mainWindow.webContents.openDevTools({ mode: "detach" });
    } else {
        const indexHtml = path.join(__dirname, "frontend", "dist", "index.html");
        mainWindow.loadFile(indexHtml);
    }

    return mainWindow;
}

app.whenReady().then(() => {
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
