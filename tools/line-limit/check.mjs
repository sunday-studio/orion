#!/usr/bin/env node

import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import process from "node:process";

const defaultMaxLines = 500;
const sourceExtensions = new Set([
  ".cjs",
  ".css",
  ".go",
  ".js",
  ".jsx",
  ".mjs",
  ".sh",
  ".ts",
  ".tsx",
]);

const sourceRoots = ["apps"];
const ignoredDirectories = new Set([
  ".git",
  "assets",
  "coverage",
  "dist",
  "docs",
  "migrations",
  "node_modules",
  "public",
  "web",
]);

const ignoredExactPaths = new Set([
  "apps/console/src/orion-sdk/index.ts",
  "apps/core/openapi.yaml",
]);

const ignoredBasenames = new Set(["vite-env.d.ts"]);

function parseArgs(argv) {
  const options = {
    maxLines: defaultMaxLines,
    root: process.cwd(),
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];

    if (arg === "--max-lines") {
      const value = Number.parseInt(argv[index + 1], 10);
      if (!Number.isInteger(value) || value < 1) {
        throw new Error("--max-lines must be a positive integer");
      }
      options.maxLines = value;
      index += 1;
      continue;
    }

    if (arg === "--root") {
      const value = argv[index + 1];
      if (!value) {
        throw new Error("--root requires a path");
      }
      options.root = path.resolve(value);
      index += 1;
      continue;
    }

    if (arg === "--help" || arg === "-h") {
      printHelp();
      process.exit(0);
    }

    throw new Error(`unknown argument: ${arg}`);
  }

  return options;
}

function printHelp() {
  console.log(`Usage: node tools/line-limit/check.mjs [--max-lines 500] [--root PATH]

Fails when app source files exceed the configured line limit.
Docs, config, generated output, build output, assets, and migrations are excluded.`);
}

function toPortablePath(filePath) {
  return filePath.split(path.sep).join("/");
}

function isConfigFile(fileName) {
  return (
    fileName.endsWith(".config.cjs") ||
    fileName.endsWith(".config.js") ||
    fileName.endsWith(".config.mjs") ||
    fileName.endsWith(".config.ts") ||
    fileName.endsWith(".d.ts")
  );
}

function shouldIgnoreFile(root, filePath) {
  const relativePath = toPortablePath(path.relative(root, filePath));
  const fileName = path.basename(filePath);

  return (
    ignoredExactPaths.has(relativePath) ||
    ignoredBasenames.has(fileName) ||
    isConfigFile(fileName) ||
    !sourceExtensions.has(path.extname(filePath))
  );
}

function* walk(root, dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      if (!ignoredDirectories.has(entry.name)) {
        yield* walk(root, fullPath);
      }
      continue;
    }

    if (entry.isFile() && !shouldIgnoreFile(root, fullPath)) {
      yield fullPath;
    }
  }
}

function countLines(filePath) {
  const contents = readFileSync(filePath, "utf8");
  if (contents.length === 0) {
    return 0;
  }

  const trailingNewline = contents.endsWith("\n") || contents.endsWith("\r");
  return contents.split(/\r\n|\r|\n/).length - (trailingNewline ? 1 : 0);
}

function findViolations(root, maxLines) {
  const violations = [];

  for (const sourceRoot of sourceRoots) {
    const fullRoot = path.join(root, sourceRoot);
    if (!statSync(fullRoot, { throwIfNoEntry: false })?.isDirectory()) {
      continue;
    }

    for (const filePath of walk(root, fullRoot)) {
      const lineCount = countLines(filePath);
      if (lineCount > maxLines) {
        violations.push({
          lineCount,
          path: toPortablePath(path.relative(root, filePath)),
        });
      }
    }
  }

  return violations.sort((a, b) => {
    if (b.lineCount !== a.lineCount) {
      return b.lineCount - a.lineCount;
    }
    return a.path.localeCompare(b.path);
  });
}

function main() {
  const { maxLines, root } = parseArgs(process.argv.slice(2));
  const violations = findViolations(root, maxLines);

  if (violations.length === 0) {
    console.log(`line-limit: all app source files are at or below ${maxLines} lines`);
    return;
  }

  console.error(`line-limit: ${violations.length} app source files exceed ${maxLines} lines`);
  for (const violation of violations) {
    console.error(`${violation.lineCount.toString().padStart(5, " ")} ${violation.path}`);
  }
  process.exitCode = 1;
}

main();
