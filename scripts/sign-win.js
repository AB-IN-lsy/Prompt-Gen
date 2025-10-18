/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-19 00:41:07
 * @FilePath: \electron-go-app\scripts\sign-win.js
 * @LastEditTime: 2025-10-19 00:43:52
 */
// scripts/sign-win.js
// electron-builder 会在签名时用 require() 加载这个模块，并调用导出函数。
// 它会把要签名的文件路径作为第 1 个参数传进来。
const { execFile } = require("child_process");
const path = require("path");

function resolveSignTool() {
    // 1) 优先用 electron-builder 缓存里的 signtool（无需安装 SDK）
    const local = path.join(
        process.env.LOCALAPPDATA || "",
        "electron-builder",
        "Cache",
        "winCodeSign",
        "winCodeSign-2.6.0",
        "windows-10",
        "x64",
        "signtool.exe"
    );
    return local;
    // 如果你安装了 Windows SDK 并想用系统的 signtool，改成：
    // return "C:\\Program Files (x86)\\Windows Kits\\10\\bin\\x64\\signtool.exe";
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
