import { expect, test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 1048 } });

const incident = {
  id: "inc-231",
  number: 231,
  title: "api.example.com unreachable",
  severity: "critical",
  status: "open",
  monitor_name: "api-https",
  monitor_type: "http",
  agent_name: "edge-01",
  opened_at: "2025-10-16T14:02:08Z",
  latest_event:
    "The health check on api-https has timed out since 14:02. ops-team was notified; the on-call webhook returned 502, so nobody was paged.",
  allowed_actions: {
    acknowledge: { allowed: true },
    cover: { allowed: true },
    resolve: { allowed: true },
  },
};

const detail = (overrides: Record<string, unknown> = {}) => ({
  incident: {
    ...incident,
    display_id: "INC-0231",
    impacted_components: [
      { component_id: "c1", component_name: "Public API", impact: "major_outage" },
      { component_id: "c2", component_name: "Checkout", impact: "degraded" },
    ],
    ...overrides,
  },
  summary: {
    notified_at: "2025-10-16T14:02:11Z",
    notified_channels: ["ops-team"],
    source: { monitor_name: "api-https", target: "GET /health", server_name: "edge-01" },
    expected: { title: "200 OK", detail: "under 1.5s, valid TLS" },
    actual: {
      title: "Timeout",
      detail: "no response in 10s, 3 failures in a row",
      at: "2025-10-16T14:32:00Z",
    },
  },
  evidence: {
    latest_report: {
      id: "r1",
      health: "down",
      collected_at: "2025-10-16T14:32:00Z",
      payload: JSON.stringify({ failure_reason: "timeout" }),
    },
  },
  alert_deliveries: [
    { id: "d1", channel: "ops-team", status: "sent", created_at: "2025-10-16T14:02:11Z" },
    { id: "d2", channel: "oncall webhook", status: "failed", created_at: "2025-10-16T14:02:12Z" },
  ],
  monitor_reports: Array.from({ length: 12 }, (_, i) => ({ id: `r${i}`, health: "down" })),
  related_incidents: [{ id: "inc-229", display_id: "INC-0229", title: "Load balancer degraded" }],
  timeline: [
    {
      id: "t2",
      type: "alert",
      status: "failed",
      alert_delivery_id: "d2",
      message: "Delivery to oncall webhook failed after 3 attempts",
      created_at: "2025-10-16T14:04:12Z",
    },
    {
      id: "t3",
      type: "alert",
      status: "sent",
      message: "Notified ops-team (email)",
      created_at: "2025-10-16T14:02:11Z",
    },
    {
      id: "t4",
      type: "incident_opened",
      message: "Incident opened: api-https failed 3 checks in a row",
      created_at: "2025-10-16T14:02:08Z",
    },
  ],
});

const mocks = (overrides: Record<string, unknown> = {}, titleRequired = false) => [
  {
    pattern: /\/v1\/overview$/,
    body: { state: "issues", ongoing_incident_count: 2, health_issue_count: 0, stats: {} },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  { pattern: /\/v1\/incidents\/inc-231$/, body: detail(overrides) },
  {
    pattern: /\/v1\/incidents\/inc-231\/logs/,
    body: { entries: Array.from({ length: 14 }, (_, i) => ({ id: `l${i}`, body: "log" })) },
  },
  {
    pattern: /\/v1\/incidents\/inc-231\/public-update$/,
    body: {
      targets: [
        {
          status_page_id: "p1",
          title: "Example Status",
          subscriber_count: 128,
          title_required: titleRequired,
        },
      ],
    },
  },
];

test("Incident detail matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:20Z");
  await mockApi(page, mocks());
  await page.goto("/incidents/inc-231");
  await page.getByRole("heading", { name: "api.example.com unreachable" }).waitFor();
  await expectMatchesArtboard(page, "incident-detail", 1048);
});

test("Incident detail posts a note and assigns a responder", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:20Z");
  await mockApi(page, mocks());
  const bodies: Record<string, unknown> = {};
  await page.route(/\/v1\/incidents\/inc-231\/notes$/, async (route) => {
    bodies.note = route.request().postDataJSON();
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: {} }),
    });
  });
  await page.route(/\/v1\/incidents\/inc-231\/responder$/, async (route) => {
    bodies.responder = route.request().postDataJSON();
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: {} }),
    });
  });
  await page.goto("/incidents/inc-231");
  await page.getByLabel("Note").fill("Looking now");
  await page.getByRole("button", { name: "Post" }).click();
  await expect.poll(() => bodies.note).toEqual({ text: "Looking now" });
  await page.getByRole("button", { name: "Assign someone" }).click();
  await page.getByLabel("Responder").fill("sam");
  await page.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => bodies.responder).toEqual({ name: "sam" });
});

const okRoute = (body: unknown = {}) => ({
  contentType: "application/json",
  body: JSON.stringify({ success: true, data: body }),
});

test("Incident dialogs send acknowledge, cover and public update payloads", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:20Z");
  await mockApi(page, [
    ...mocks({}, true),
    {
      pattern: /\/v1\/incidents$/,
      body: {
        count: 2,
        incidents: [
          incident,
          { id: "inc-229", display_id: "INC-0229", title: "Load balancer degraded" },
        ],
      },
    },
  ]);
  const bodies: Record<
    string,
    { note?: string; covered_by?: string; title?: string; status?: string }
  > = {};
  for (const name of ["acknowledge", "cover", "public-updates"]) {
    await page.route(new RegExp(`/v1/incidents/inc-231/${name}$`), async (route) => {
      bodies[name] = route.request().postDataJSON();
      await route.fulfill(okRoute({}));
    });
  }
  await page.goto("/incidents/inc-231");

  await page.getByRole("button", { name: "Acknowledge" }).click();
  await expect(page.getByText("Tell the team you are on it.")).toBeVisible();
  await page.getByLabel("Note, optional").fill("Checking the load balancer.");
  await page.getByRole("dialog").getByRole("button", { name: "Acknowledge" }).click();
  await expect.poll(() => bodies.acknowledge?.note).toBe("Checking the load balancer.");

  await page.getByRole("button", { name: "Cover" }).click();
  await page.getByRole("combobox", { name: "Covered by" }).click();
  await page.getByRole("option", { name: /INC-0229/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Cover" }).click();
  await expect.poll(() => bodies.cover?.covered_by).toBe("inc-229");

  await page.getByRole("button", { name: "Draft update" }).click();
  await expect(page.getByText("No public incident exists for INC-0231 yet.")).toBeVisible();
  await page.getByLabel("Public title").fill("Elevated API errors");
  await page.getByRole("radio", { name: "Identified" }).click();
  await page.getByLabel("Message").fill("We found the cause.");
  await page.getByRole("button", { name: "Post update" }).click();
  await expect
    .poll(() => bodies["public-updates"])
    .toMatchObject({
      title: "Elevated API errors",
      status: "identified",
    });
});
