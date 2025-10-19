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
const os = require("os");

function createPfxFile() {
    const rawBase64 = process.env.WINDOWS_CERT_BASE64;
    if (!rawBase64) {
        throw new Error("WINDOWS_CERT_BASE64 is not set. Please provide the base64 encoded PFX via repository secrets.");
    }

    const sanitized = rawBase64.replace(/\s+/g, "");
    let buffer;
    try {
        buffer = Buffer.from(sanitized, "base64");
    } catch (error) {
        throw new Error(`Failed to decode WINDOWS_CERT_BASE64: ${error.message}`);
    }

    if (!buffer || buffer.length === 0) {
        throw new Error("WINDOWS_CERT_BASE64 decoded to an empty buffer.");
    }

    const tempPath = path.join(
        os.tmpdir(),
        `promptgen-codesign-${Date.now()}-${Math.random().toString(16).slice(2)}.pfx`
    );
    fs.writeFileSync(tempPath, buffer);
    return tempPath;
}

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
        const pfxPath = createPfxFile();
        const args = [
            "sign",
            "/fd", "sha256",
            "/td", "sha256",
            "/f", pfxPath
        ];

        const password = process.env.WINDOWS_CERT_PASSWORD;
        if (password) {
            args.push("/p", password);
        }

        const description = process.env.WINDOWS_CERT_DESCRIPTION;
        if (description) {
            args.push("/d", description);
        }

        const timestampUrl = process.env.WINDOWS_CERT_TIMESTAMP_URL;
        if (timestampUrl) {
            args.push("/tr", timestampUrl);
        }

        args.push(file);

        execFile(signtool, args, { windowsHide: true }, (err, stdout, stderr) => {
            try {
                if (stdout) process.stdout.write(stdout);
                if (stderr) process.stderr.write(stderr);
                if (err) return reject(err);
                resolve();
            } finally {
                fs.rmSync(pfxPath, { force: true });
            }
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
