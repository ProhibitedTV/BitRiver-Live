import { cpSync, existsSync, mkdirSync, rmSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const standaloneDir = path.join(root, ".next", "standalone");
const serverPath = path.join(standaloneDir, "server.js");

if (!existsSync(serverPath)) {
  console.error("Missing .next/standalone/server.js. Run next build before starting the Playwright server.");
  process.exit(1);
}

function copyDirectory(source, destination, { required = false, createWhenMissing = false } = {}) {
  if (!existsSync(source)) {
    if (required) {
      console.error(`Missing required standalone asset directory: ${path.relative(root, source)}`);
      process.exit(1);
    }
    if (createWhenMissing) {
      mkdirSync(destination, { recursive: true });
    }
    return;
  }

  rmSync(destination, { recursive: true, force: true });
  mkdirSync(path.dirname(destination), { recursive: true });
  cpSync(source, destination, { recursive: true });
}

copyDirectory(path.join(root, ".next", "static"), path.join(standaloneDir, ".next", "static"), { required: true });
copyDirectory(path.join(root, "public"), path.join(standaloneDir, "public"), { createWhenMissing: true });
