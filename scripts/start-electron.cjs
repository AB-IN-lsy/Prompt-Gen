#!/usr/bin/env node
const { spawn } = require("node:child_process");
const electronPath = require("electron");

const child = spawn(electronPath, ["."], {
  stdio: "inherit",
  env: process.env,
});

child.on("exit", (code, signal) => {
  if (typeof code === "number") {
    process.exit(code);
    return;
  }
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(0);
});

child.on("error", (error) => {
  console.error("Failed to launch Electron:", error);
  process.exit(1);
});
