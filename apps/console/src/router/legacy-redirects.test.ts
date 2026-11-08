import assert from "node:assert/strict";
import { test } from "node:test";
import { resolveLegacyRedirect } from "./legacy-redirects.ts";

test("current URLs are left alone", () => {
  for (const [path, search] of [
    ["/", ""],
    ["/incidents", ""],
    ["/monitors", "?new=1"],
    ["/servers/abc", "?tab=usage"],
    ["/servers/abc", "?tab=reports"],
    ["/logs", "?tab=status"],
    ["/logs/apps", ""],
    ["/logs/apps/new", ""],
    ["/logs/apps/app_1", "?tab=ingest"],
    ["/alerts", ""],
    ["/alerts/channels", ""],
    ["/alerts/channels/ch_1", ""],
    ["/settings/general", ""],
    ["/status-pages/sp_1", "?tab=setup"],
  ] as const) {
    assert.equal(resolveLegacyRedirect(path, search), null, `${path}${search}`);
  }
});

test("Settings tabs become Settings pages", () => {
  assert.equal(resolveLegacyRedirect("/settings", ""), "/settings/general");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=lifecycle"), "/settings/lifecycle");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=security"), "/settings/security");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=system"), "/settings/system");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=audit"), "/settings/audit");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=general"), "/settings/general");
  assert.equal(resolveLegacyRedirect("/settings/", "?tab=audit"), "/settings/audit");
});

test("an unknown Settings tab lands on General and other params survive", () => {
  assert.equal(resolveLegacyRedirect("/settings", "?tab=nope"), "/settings/general");
  assert.equal(resolveLegacyRedirect("/settings", "?tab=audit&q=login"), "/settings/audit?q=login");
});

test("Apps moves from Settings to Logs", () => {
  assert.equal(resolveLegacyRedirect("/settings", "?tab=apps"), "/logs/apps");
  assert.equal(resolveLegacyRedirect("/settings/apps", ""), "/logs/apps");
  assert.equal(resolveLegacyRedirect("/settings/apps/app_1", ""), "/logs/apps/app_1");
  assert.equal(
    resolveLegacyRedirect("/settings/apps/app_1", "?tab=retention"),
    "/logs/apps/app_1?tab=retention",
  );
});

test("the connect wizard moves under Logs", () => {
  assert.equal(resolveLegacyRedirect("/apps/new", ""), "/logs/apps/new");
  assert.equal(resolveLegacyRedirect("/apps/new", "?step=2"), "/logs/apps/new?step=2");
});

test("Alerts tabs become Notify pages", () => {
  assert.equal(resolveLegacyRedirect("/alerts", "?tab=channels"), "/alerts/channels");
  assert.equal(resolveLegacyRedirect("/alerts", "?tab=rules"), "/alerts/rules");
  assert.equal(resolveLegacyRedirect("/alerts", "?tab=routes"), "/alerts/routes");
  assert.equal(resolveLegacyRedirect("/alerts", "?tab=logs"), "/alerts");
  assert.equal(resolveLegacyRedirect("/alerts", "?tab=nope"), null);
});

test("the server tab apps is now usage", () => {
  assert.equal(resolveLegacyRedirect("/servers/n1", "?tab=apps"), "/servers/n1?tab=usage");
  assert.equal(resolveLegacyRedirect("/servers/n1", "?tab=apps&x=1"), "/servers/n1?tab=usage&x=1");
});

test("/overview goes to the root", () => {
  assert.equal(resolveLegacyRedirect("/overview", ""), "/");
});
