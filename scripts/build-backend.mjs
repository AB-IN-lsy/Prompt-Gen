#!/usr/bin/env node

import { spawnSync } from "child_process";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, "..");
const binDir = path.join(projectRoot, "backend", "bin");

fs.mkdirSync(binDir, { recursive: true });

const isWindows = process.platform === "win32";
const targetBinary = path.join(binDir, isWindows ? "server.exe" : "server");
const staleBinary = path.join(binDir, isWindows ? "server" : "server.exe");

if (fs.existsSync(staleBinary)) {
  fs.rmSync(staleBinary, { force: true });
}

const buildArgs = ["build", "-o", targetBinary, "./backend/cmd/server"];
const buildEnv = {
  ...process.env,
};

const spawnOptions = {
  cwd: projectRoot,
  stdio: "inherit",
  env: buildEnv,
  shell: process.platform === "win32",
};

const result = spawnSync("go", buildArgs, spawnOptions);

if (result.error) {
  console.error("构建后端失败:", result.error);
  process.exit(1);
}

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
