/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 23:45:51
 * @FilePath: \electron-go-app\frontend\scripts\no-js-check.mjs
 * @LastEditTime: 2025-10-10 23:47:16
 */
import { readdirSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const projectRoot = join(__dirname, "..");
const srcDir = join(projectRoot, "src");

function collectJsFiles(dir) {
    const entries = readdirSync(dir, { withFileTypes: true });
    const matches = [];
    for (const entry of entries) {
        const entryPath = join(dir, entry.name);
        if (entry.isDirectory()) {
            matches.push(...collectJsFiles(entryPath));
            continue;
        }
        if (entry.isFile() && entry.name.endsWith(".js")) {
            matches.push(entryPath);
        }
    }
    return matches;
}

const jsFiles = collectJsFiles(srcDir);
if (jsFiles.length > 0) {
    const relativePaths = jsFiles
        .map((file) => relative(projectRoot, file))
        .sort();
    console.error("Found forbidden JavaScript artifacts under src:\n" + relativePaths.join("\n"));
    process.exit(1);
}
