/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-19 00:41:07
 * @FilePath: \electron-go-app\scripts\sign-win.js
 * @LastEditTime: 2025-10-19 00:43:52
 */
// scripts/sign-win.js
// electron-builder 会在签名时用 require() 加载这个模块，并调用导出函数。
// 它会把要签名的文件路径作为第 1 个参数传进来。
const { execFile, execSync } = require("child_process");
const path = require("path");
const fs = require("fs");

function resolveSignTool() {
    const custom = process.env.SIGNTOOL_PATH;
    if (custom && fs.existsSync(custom)) {
        return custom;
    }

    const cached = path.join(
        process.env.LOCALAPPDATA || "",
        "electron-builder",
        "Cache",
        "winCodeSign",
        "winCodeSign-2.6.0",
        "windows-10",
        "x64",
        "signtool.exe"
    );
    if (fs.existsSync(cached)) {
        return cached;
    }

    const wellKnownPaths = [
        "C:\\Program Files (x86)\\Windows Kits\\10\\bin\\x64\\signtool.exe",
        "C:\\Program Files (x86)\\Windows Kits\\10\\App Certification Kit\\signtool.exe",
        "C:\\Program Files\\Microsoft SDKs\\Windows\\v10.0A\\bin\\NETFX 4.8 Tools\\signtool.exe"
    ];
    for (const candidate of wellKnownPaths) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }

    try {
        const output = execSync("where signtool", {
            stdio: ["ignore", "pipe", "ignore"]
        })
            .toString()
            .split(/\r?\n/)
            .map((line) => line.trim())
            .find((line) => line.toLowerCase().endsWith("signtool.exe"));
        if (output && fs.existsSync(output)) {
            return output;
        }
    } catch (error) {
        // ignore and fall through
    }

    throw new Error("signtool.exe not found. Please ensure Windows SDK is installed or set SIGNTOOL_PATH.");
}

// 注意：不带 /t 或 /tr（不访问时间戳服务器）
function signWithSigntool(file) {
    return new Promise((resolve, reject) => {
        const signtool = resolveSignTool();
        const args = [
            "sign",
            "/fd", "sha256",
            "/td", "sha256",
            "/as",
            "/n", "ab-in",      // 你的证书 Subject（也可改用 /sha1 <thumbprint>）
            file
        ];

        execFile(signtool, args, { windowsHide: true }, (err, stdout, stderr) => {
            if (stdout) process.stdout.write(stdout);
            if (stderr) process.stderr.write(stderr);
            if (err) return reject(err);
            resolve();
        });
    });
}

// electron-builder 调用签名钩子时会传入： (configuration, packager) 或 (filepath)
module.exports = async function sign(context) {
    // 新版通常传 { path } 或 直接传字符串；两种都兼容一下：
    const file = typeof context === "string" ? context : (context.path || context.file || context);
    if (!file) throw new Error("sign hook: no input file path");
    await signWithSigntool(file);
};
