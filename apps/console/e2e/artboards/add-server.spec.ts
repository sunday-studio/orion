import { type Page, test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 623 } });

const created = {
  id: "enr-1",
  status: "waiting",
  token: "orn_enr_7Qx2m9a1c4",
  platform: "linux",
  core_url: "https://core.example.com",
  created_at: "2025-10-16T14:30:00Z",
  expires_at: "2025-10-16T14:45:00Z",
  install_command:
    "curl -fsSL https://core.example.com/install.sh | sudo sh -s -- --token orn_enr_7Qx2m9a1c4",
};

const summaryMocks = [
  {
    pattern: /\/v1\/overview$/,
    body: {
      state: "ok",
      ongoing_incident_count: 0,
      health_issue_count: 0,
      stats: {},
      ongoing_incidents: [],
      recent_incidents: [],
    },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin", auth_enabled: true } },
];

const withEnrollment = (live: () => object): Parameters<typeof mockApi>[1] => [
  ...summaryMocks,
  { pattern: /\/v1\/enrollments$/, body: created },
  { pattern: /\/v1\/enrollments\/enr-1$/, body: live },
];

const open = async (page: Page) => {
  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
};

test("Add server (waiting for the first install) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:30:00Z");
  await mockApi(
    page,
    withEnrollment(() => ({ ...created, token: undefined })),
  );
  await open(page);
  await page.clock.fastForward("05:05");
  await page.getByText("Not connected after 5 minutes?").waitFor();
  await expectMatchesArtboard(page, "add-server", 623);
});

test("Add server (waiting) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:34:45Z");
  let phase: "waiting" | "registered" = "waiting";
  await mockApi(
    page,
    withEnrollment(() =>
      phase === "waiting"
        ? { ...created, status: "waiting" }
        : {
            ...created,
            status: "registered",
            claimed_at: "2025-10-16T14:34:41Z",
            agent_id: "a5",
            agent_name: "edge-05",
            agent_os: "linux",
            agent_arch: "amd64",
            agent_version: "0.9.2",
          },
    ),
  );
  await open(page);
  phase = "registered";
  await page.clock.fastForward(4000);
  await page.getByText("Registered as edge-05").waitFor();
  await expectMatchesArtboard(page, "add-server-waiting", 481);
});

test("Add server (connected) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:35:05Z");
  let reporting = false;
  await mockApi(
    page,
    withEnrollment(() =>
      reporting
        ? {
            ...created,
            status: "reporting",
            claimed_at: "2025-10-16T14:34:41Z",
            agent_id: "a5",
            agent_name: "edge-05",
            agent_os: "linux",
            agent_arch: "amd64",
            agent_version: "0.9.2",
            first_report_at: "2025-10-16T14:35:02Z",
          }
        : { ...created },
    ),
  );
  await open(page);
  reporting = true;
  await page.clock.fastForward(4000);
  await page.getByText("edge-05 is connected").waitFor();
  await expectMatchesArtboard(page, "add-server-connected", 708);
});

test("Add server (failed) matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:30:00Z");
  let expired = false;
  await mockApi(
    page,
    withEnrollment(() => (expired ? { ...created, status: "expired" } : { ...created })),
  );
  await open(page);
  expired = true;
  await page.clock.fastForward(4000);
  await page.getByText("We have not seen the server yet").waitFor();
  await expectMatchesArtboard(page, "add-server-failed", 523);
});
