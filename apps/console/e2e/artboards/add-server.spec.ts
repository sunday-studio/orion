import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 623 } });

const baseAgents = [
  { id: "a1", name: "edge-01" },
  { id: "a2", name: "edge-02" },
];
const newAgent = {
  id: "a5",
  name: "edge-05",
  os: "linux",
  arch: "amd64",
};

const summaryMocks = [
  {
    pattern: /\/v1\/health\/summary$/,
    body: { overall_health: "up", monitors: { total: 24, up: 24 }, agents: { total: 6 } },
  },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  { pattern: /\/v1\/incidents$/, body: { count: 0, incidents: [] } },
];

const agentsAfter = (extra: object[]) => ({
  pattern: /\/v1\/agents$/,
  body: { count: baseAgents.length + extra.length, agents: [...baseAgents, ...extra] },
});

test("Add server (waiting for the first install) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:30:00Z");
  await mockApi(page, [...summaryMocks, agentsAfter([])]);
  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
  await page.clock.fastForward("05:05");
  await page.getByText("Not connected after 5 minutes?").waitFor();
  await expectMatchesArtboard(page, "add-server", 623);
});

test("Add server (waiting) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:30:00Z");
  let registered = false;
  await mockApi(page, [
    ...summaryMocks,
    {
      pattern: /\/v1\/agents$/,
      body: () => ({ count: 3, agents: registered ? [...baseAgents, newAgent] : baseAgents }),
    },
  ]);
  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
  registered = true;
  await page.clock.fastForward(4000);
  await page.getByText("Registered as edge-05").waitFor();
  await expectMatchesArtboard(page, "add-server-waiting", 481);
});

test("Add server (connected) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:35:00Z");
  let registered = false;
  await mockApi(page, [
    ...summaryMocks,
    {
      pattern: /\/v1\/agents$/,
      body: () => ({
        count: 3,
        agents: registered
          ? [...baseAgents, { ...newAgent, last_seen: "2025-10-16T14:35:02Z" }]
          : baseAgents,
      }),
    },
  ]);
  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
  registered = true;
  await page.clock.fastForward(4000);
  await page.getByText("edge-05 is connected").waitFor();
  await expectMatchesArtboard(page, "add-server-connected", 708);
});

test("Add server (failed) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:30:00Z");
  await mockApi(page, [...summaryMocks, agentsAfter([])]);
  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
  await page.clock.fastForward("15:05");
  await page.getByText("We have not seen the server yet").waitFor();
  await expectMatchesArtboard(page, "add-server-failed", 523);
});
