import assert from "node:assert/strict";
import { test } from "node:test";
import { NAV_GROUPS, isGroupOpen, resolveActiveNav, toggleGroup } from "./nav-model.ts";

const active = (path: string, search = "") => resolveActiveNav(path, search);

test("top-level pages have no group", () => {
  assert.deepEqual(active("/"), { groupId: null, pageId: "overview" });
  assert.deepEqual(active("/health-issues"), { groupId: null, pageId: "overview" });
  assert.deepEqual(active("/incidents"), { groupId: null, pageId: "incidents" });
  assert.deepEqual(active("/incidents/inc_1"), { groupId: null, pageId: "incidents" });
  assert.deepEqual(active("/incidents/triage"), { groupId: null, pageId: "incidents" });
});

test("Monitoring pages and their records", () => {
  assert.deepEqual(active("/services"), { groupId: "monitoring", pageId: "services" });
  assert.deepEqual(active("/services/svc_1"), { groupId: "monitoring", pageId: "services" });
  assert.deepEqual(active("/monitors/new/node"), { groupId: "monitoring", pageId: "monitors" });
  assert.deepEqual(active("/monitors/m1/edit"), { groupId: "monitoring", pageId: "monitors" });
  assert.deepEqual(active("/servers/new"), { groupId: "monitoring", pageId: "servers" });
  assert.deepEqual(active("/servers/n1", "?tab=usage"), {
    groupId: "monitoring",
    pageId: "servers",
  });
});

test("Logs pages follow the tab query, defaulting to App logs", () => {
  assert.deepEqual(active("/logs"), { groupId: "logs", pageId: "app-logs" });
  assert.deepEqual(active("/logs", "?tab=service"), { groupId: "logs", pageId: "app-logs" });
  assert.deepEqual(active("/logs", "?tab=status"), { groupId: "logs", pageId: "events" });
  assert.deepEqual(active("/logs", "?tab=internal"), { groupId: "logs", pageId: "core-logs" });
  assert.deepEqual(active("/logs", "?view=live&tab=status"), { groupId: "logs", pageId: "events" });
  assert.deepEqual(active("/logs", "?tab=unknown"), { groupId: "logs", pageId: "app-logs" });
});

test("Apps and the connect wizard highlight Logs > Apps", () => {
  for (const path of ["/logs/apps", "/logs/apps/new", "/logs/apps/app_1"]) {
    assert.deepEqual(active(path), { groupId: "logs", pageId: "apps" }, path);
  }
});

test("Notify holds the alert pages and status pages", () => {
  assert.deepEqual(active("/alerts"), { groupId: "notify", pageId: "notifications" });
  assert.deepEqual(active("/alerts/deliveries/d1"), { groupId: "notify", pageId: "notifications" });
  assert.deepEqual(active("/alerts/channels"), { groupId: "notify", pageId: "channels" });
  assert.deepEqual(active("/alerts/channels/c1"), { groupId: "notify", pageId: "channels" });
  assert.deepEqual(active("/alerts/rules"), { groupId: "notify", pageId: "rules" });
  assert.deepEqual(active("/alerts/routes"), { groupId: "notify", pageId: "routes" });
  assert.deepEqual(active("/status-pages"), { groupId: "notify", pageId: "status-pages" });
  assert.deepEqual(active("/status-pages/sp1"), { groupId: "notify", pageId: "status-pages" });
});

test("Settings pages", () => {
  for (const section of ["general", "lifecycle", "security", "system", "audit"]) {
    assert.deepEqual(active(`/settings/${section}`), { groupId: "settings", pageId: section });
  }
  assert.deepEqual(active("/settings"), { groupId: "settings", pageId: "general" });
});

test("pages the sidebar does not list light nothing", () => {
  assert.deepEqual(active("/design-system"), { groupId: null, pageId: null });
  assert.deepEqual(active("/nope"), { groupId: null, pageId: null });
  assert.deepEqual(active("/logsx"), { groupId: null, pageId: null });
});

test("a trailing slash does not change the answer", () => {
  assert.deepEqual(active("/monitors/"), { groupId: "monitoring", pageId: "monitors" });
});

test("every sidebar link resolves to its own page", () => {
  for (const group of NAV_GROUPS) {
    for (const page of group.pages) {
      const [path, query = ""] = page.to.split("?");
      const result = active(path, query ? `?${query}` : "");
      assert.deepEqual(result, { groupId: group.id, pageId: page.id }, page.to);
    }
  }
});

test("only the group holding the current page is open by default", () => {
  assert.equal(isGroupOpen("logs", "logs", {}), true);
  assert.equal(isGroupOpen("notify", "logs", {}), false);
  assert.equal(isGroupOpen("monitoring", null, {}), false);
});

test("a hand-made choice beats the default, other groups keep theirs", () => {
  const opened = toggleGroup("notify", "logs", {});
  assert.equal(isGroupOpen("notify", "logs", opened), true);
  assert.equal(isGroupOpen("logs", "logs", opened), true);
  const closed = toggleGroup("logs", "logs", opened);
  assert.equal(isGroupOpen("logs", "logs", closed), false);
  assert.equal(isGroupOpen("notify", "logs", closed), true);
  const reopened = toggleGroup("logs", "logs", closed);
  assert.equal(isGroupOpen("logs", "logs", reopened), true);
});
