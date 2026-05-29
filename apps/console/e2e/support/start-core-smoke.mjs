import { spawn } from "node:child_process";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { get } from "node:http";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const consoleDir = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const repoDir = resolve(consoleDir, "../..");
const coreDir = join(repoDir, "apps/core");
const coreURL = "http://127.0.0.1:18999";
const consoleURL = "http://127.0.0.1:5173";
const adminUsername = "admin";
const adminPassword = "change-me";
const jwtSecret = "console-browser-smoke-secret-at-least-long-enough";
const children = [];
let workDir;
let shuttingDown = false;

const run = (command, args, options = {}) =>
  new Promise((resolveRun, rejectRun) => {
    const child = spawn(command, args, {
      stdio: "inherit",
      shell: false,
      ...options,
    });
    child.on("error", rejectRun);
    child.on("exit", (code, signal) => {
      if (code === 0) {
        resolveRun();
        return;
      }
      rejectRun(new Error(`${command} ${args.join(" ")} exited with ${code ?? signal}`));
    });
  });

const start = (command, args, options = {}) => {
  const child = spawn(command, args, {
    stdio: "inherit",
    shell: false,
    ...options,
  });
  children.push(child);
  child.on("exit", (code, signal) => {
    if (!shuttingDown && code !== 0) {
      console.error(`${command} ${args.join(" ")} exited with ${code ?? signal}`);
      void shutdown(1);
    }
  });
  return child;
};

const waitFor = async (url, timeoutMs) => {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    if (await requestOK(url)) return;
    await new Promise((resolveWait) => setTimeout(resolveWait, 250));
  }
  throw new Error(`Timed out waiting for ${url}`);
};

const requestOK = (url) =>
  new Promise((resolveRequest) => {
    const req = get(url, (res) => {
      res.resume();
      resolveRequest(Boolean(res.statusCode && res.statusCode >= 200 && res.statusCode < 500));
    });
    req.on("error", () => resolveRequest(false));
    req.setTimeout(1000, () => {
      req.destroy();
      resolveRequest(false);
    });
  });

const shutdown = async (exitCode = 0) => {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const child of children) {
    if (!child.killed) child.kill("SIGTERM");
  }
  await new Promise((resolveWait) => setTimeout(resolveWait, 500));
  for (const child of children) {
    if (!child.killed) child.kill("SIGKILL");
  }
  if (workDir) {
    await rm(workDir, { recursive: true, force: true }).catch(() => {});
  }
  process.exit(exitCode);
};

process.on("SIGINT", () => void shutdown(0));
process.on("SIGTERM", () => void shutdown(0));
process.on("exit", () => {
  for (const child of children) {
    if (!child.killed) child.kill("SIGTERM");
  }
});

workDir = await mkdtemp(join(tmpdir(), "orion-console-e2e-"));
const dataDir = join(workDir, "data");
await mkdir(dataDir, { recursive: true });

await run(
  "go",
  ["run", "./scripts/seed-demo-data", "-db", join(dataDir, "orion.db"), "-days", "14"],
  {
    cwd: coreDir,
  },
);

start("go", ["run", "."], {
  cwd: coreDir,
  env: {
    ...process.env,
    ORION_ADMIN_USERNAME: adminUsername,
    ORION_ADMIN_PASSWORD: adminPassword,
    ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS: "true",
    ORION_JWT_SECRET: jwtSecret,
    ORION_PORT: "18999",
    ORION_DATA_DIR: dataDir,
    ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS: "3600",
  },
});
await waitFor(`${coreURL}/health`, 60_000);

start("pnpm", ["dev", "--host", "127.0.0.1", "--port", "5173"], {
  cwd: consoleDir,
  env: {
    ...process.env,
    VITE_API_BASE_URL: `${coreURL}/v1`,
  },
});
await waitFor(consoleURL, 60_000);

setInterval(() => {}, 60_000);
