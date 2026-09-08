import { cpSync, existsSync, mkdirSync, rmSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const source = resolve(root, "out");
const target = resolve(root, "..", "cmd", "frpp", "out");

if (!existsSync(source)) {
  throw new Error(`Missing static export directory: ${source}`);
}

rmSync(target, { force: true, recursive: true });
mkdirSync(dirname(target), { recursive: true });
cpSync(source, target, { recursive: true });

console.log(`Copied static export to ${target}`);
