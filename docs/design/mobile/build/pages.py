"""IA and flows artboards (1280 wide, desktop Orion look, light)."""
from common import page

PCSS = """
.p{--bg:#fff;--gr:#f7f7f6;--bd:#ececea;--tx:#18181b;--mu:#52525b;--ac:#1f7a50;--upb:#ccfbf1;--upt:#115e59;--dnb:#ffe4e6;--dnt:#9f1239;--dgb:#fef3c7;--dgt:#92400e;--acs:#e6f4ec;--dn:#e5484d;
font-family:"TWK Lausanne",system-ui,-apple-system,"Segoe UI",sans-serif;font-size:14px;line-height:1.45;color:var(--tx);background:var(--bg);padding:48px;display:flex;flex-direction:column;gap:36px;font-variant-numeric:tabular-nums}
.p h1{font-size:28px;margin:0;font-weight:500}.p h2{font-size:15px;margin:0 0 4px;font-weight:500}.p h3{font-size:13px;margin:0;font-weight:500}
.p .mu{color:var(--mu)}.p .lead{font-size:15px;color:var(--mu);max-width:820px;margin-top:6px}
.p .blk{border-top:1px solid var(--bd);padding-top:20px;display:flex;flex-direction:column;gap:14px}
.p .tabsrow{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px;align-items:start}
.p .col{border:1px solid var(--bd);background:var(--bg);display:flex;flex-direction:column;border-radius:10px;overflow:hidden;box-shadow:0 1px 2px rgba(24,24,27,.05)}
.p .col>header{background:var(--gr);color:var(--tx);padding:10px 12px;font-weight:500;display:flex;justify-content:space-between;align-items:center}
.p .col>header small{font-weight:400;color:var(--mu)}
.p .it{padding:7px 12px;border-top:1px solid var(--bd);font-size:13px;display:flex;justify-content:space-between;gap:8px}
.p .it.l2{padding-left:26px;color:var(--mu)}.p .it.l3{padding-left:40px;color:var(--mu);font-size:12.5px}
.p .it.sh{background:var(--gr)}
.p .tag{font-size:11px;font-weight:500;border-radius:4px;padding:1px 6px;white-space:nowrap;flex:none;height:18px}
.p .tag.push{background:var(--acs);color:var(--ac)}.p .tag.new{background:var(--dgb);color:var(--dgt)}.p .tag.web{background:var(--gr);color:var(--mu);border:1px solid var(--bd)}
.p table{border-collapse:separate;border-spacing:0;width:100%;border:1px solid var(--bd);border-radius:10px;overflow:hidden}
.p th{background:var(--gr);text-align:left;font-size:11px;text-transform:uppercase;letter-spacing:.04em;padding:8px 12px;color:var(--mu);font-weight:500}
.p td{padding:9px 12px;border-top:1px solid var(--bd);vertical-align:top;font-size:13px}
.p code{font-family:ui-monospace,Menlo,monospace;font-size:12px;color:var(--mu)}
.p .grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px}.p .grid3{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}
.p .box{border:1px solid var(--bd);border-radius:10px;box-shadow:0 1px 2px rgba(24,24,27,.05);padding:14px;display:flex;flex-direction:column;gap:6px;font-size:13px}.p .box p{margin:0;color:var(--mu)}
.p .box ul{margin:0;padding-left:18px;color:var(--mu)}

.fl2{display:flex;flex-direction:column;gap:10px;padding:16px 0;border-top:1px solid var(--bd)}
.st2{display:flex;align-items:stretch;flex-wrap:nowrap}
.stp{flex:1;min-width:0;border:1px solid var(--tx);padding:10px;font-size:12px;background:var(--bg)}
.stp small{display:block;color:var(--mu);font-size:11px;margin-top:2px}
.stp.x{border-color:var(--dn);background:var(--dnb);color:var(--dnt)}.stp.x small{color:var(--dnt)}
.stp.k{border-color:var(--ac);background:var(--upb);color:var(--upt)}.stp.k small{color:var(--upt)}
.stp.n{border-style:dashed}
.a2{width:20px;flex:none;align-self:center;height:1px;background:var(--mu)}
"""


def tag(k, t):
    return '<span class="tag %s">%s</span>' % (k, t)


def it(t, lvl=1, tags="", sh=False):
    cls = "it" + ("" if lvl == 1 else " l%d" % lvl) + (" sh" if sh else "")
    return '<div class="%s"><span>%s</span><span>%s</span></div>' % (cls, t, tags)


def col(title, sub, items):
    return '<div class="col"><header>%s<small>%s</small></header>%s</div>' % (title, sub, "".join(items))


P, N = tag("push", "push"), tag("new", "new API")


def ia():
    tabs = '<div class="tabsrow">%s</div>' % "".join([
        col("Notifications", "Home", [
            it("Overview strip", 1, "", True), it("Feed, grouped Needs you / Earlier"), it("Filter: All, Unread, Incidents, Servers, Monitors", 2),
            it("Notification row", 2, P), it("Opens incident, server, monitor or delivery", 3), it("Acknowledge inline", 3), it("All quiet empty state", 2)]),
        col("Incidents", "Tab 2", [
            it("Active", 1), it("Incident row", 2), it("Incident detail", 2, P),
            it("Timeline", 3), it("Notes", 3), it("Logs for this incident", 3), it("Sheet: Acknowledge", 3), it("Sheet: Resolve", 3),
            it("Sheet: Who is on this (take, cover)", 3), it("Sheet: Post status page update", 3),
            it("Needs review (triage inbox)", 1, P), it("Open incident or Dismiss", 2), it("Resolved", 1)]),
        col("Servers", "Tab 3", [
            it("Server list, filter: All, Needs attention, Maintenance"), it("Server detail", 2, P),
            it("Overview: metric tiles", 3), it("Monitors on this server", 3), it("Reports", 3), it("Logs for this server", 3)]),
        col("Monitors", "Tab 4", [
            it("Monitor list, filter: All, Down, Degraded, Paused"), it("Monitor detail", 2, P),
            it("Response time chart, uptime strip", 3), it("History", 3), it("Action: Run test now, result sheet", 3), it("Action: Pause, Resume", 3)]),
        col("More", "Tab 5", [
            it("Current Core card, Switch", 1), it("Core switcher sheet", 2), it("Add a Core", 3),
            it("Logs", 1), it("Events / Service logs, filters, histogram", 2), it("Live tail, Log detail", 2),
            it("Alert deliveries", 1, P), it("Delivery detail, Retry", 2),
            it("Search", 1), it("Notifications settings", 1), it("Settings", 1), it("Forget this Core", 1)]),
    ])
    pre = col("Connect (before a Core exists)", "Not in tabs", [
        it("Welcome"), it("Scan QR code", 2), it("Enter server address", 2), it("Insecure address warning", 3),
        it("Sign in (fallback)", 2), it("Paired, permissions", 1), it("Notifications, Critical alerts, Face ID", 2)])
    glob = col("Global, any screen", "Overlays", [
        it("Core switcher in every tab header"), it("Search icon opens Search"), it("Offline banner with last sync time"),
        it("Session expired sheet"), it("Device removed (revoked) full screen"), it("Face ID lock on open")])
    entry = col("Outside the app", "Entry points", [
        it("Push notification", 1, P), it("Lock screen actions: Acknowledge, Open", 2), it("Home screen widget", 1), it("Live Activity for an open incident", 1), it("Dynamic Island, compact", 2)])
    tbl = ("<table><tr><th>Notification</th><th>Opens</th><th>Back goes to</th><th>Lock screen actions</th></tr>"
           "<tr><td>Incident opened or escalated</td><td>Incident detail</td><td>Notifications feed</td><td>Acknowledge, Open</td></tr>"
           "<tr><td>Incident resolved, server recovered</td><td>Incident detail, resolved</td><td>Notifications feed</td><td>Open</td></tr>"
           "<tr><td>Server stopped reporting</td><td>Server detail</td><td>Notifications feed</td><td>Open</td></tr>"
           "<tr><td>Monitor failing, no incident yet</td><td>Monitor detail</td><td>Notifications feed</td><td>Run test, Open</td></tr>"
           "<tr><td>Triage candidate needs review</td><td>Incidents, Needs review</td><td>Notifications feed</td><td>Open incident, Dismiss</td></tr>"
           "<tr><td>Delivery failed (admins)</td><td>Delivery detail</td><td>Alert deliveries</td><td>Retry, Open</td></tr>"
           "<tr><td>This device was removed</td><td>Device removed screen</td><td>none</td><td>none</td></tr></table>")
    api = ("<table><tr><th>Surface</th><th>Core API</th><th>Status</th></tr>"
           "<tr><td>Connect, pair</td><td><code>/health</code>, <code>/v1/auth/login</code>, <code>POST /v1/devices/pair</code></td><td>%s</td></tr>"
           "<tr><td>Notification feed</td><td><code>/v1/alerts/deliveries?device=</code>, <code>/v1/overview</code>, <code>/v1/health/summary</code></td><td>%s device filter</td></tr>"
           "<tr><td>Push channel</td><td><code>mobile_push</code> channel type, <code>/v1/devices/{id}/push-token</code></td><td>%s</td></tr>"
           "<tr><td>Incidents, notes, actions, handoff, public update</td><td><code>/v1/incidents*</code></td><td>exists</td></tr>"
           "<tr><td>Needs review</td><td><code>/v1/incidents/candidates*</code></td><td>exists</td></tr>"
           "<tr><td>Servers, monitors, test, pause, resume</td><td><code>/v1/agents*</code>, <code>/v1/monitors*</code></td><td>exists</td></tr>"
           "<tr><td>Logs, live tail</td><td><code>/v1/logs*</code>, <code>/v1/logs/stream</code> (SSE)</td><td>exists</td></tr>"
           "<tr><td>Alert deliveries, retry</td><td><code>/v1/alerts/deliveries*</code></td><td>exists</td></tr>"
           "<tr><td>Search</td><td><code>/v1/search</code></td><td>exists</td></tr></table>") % (N, N, N)
    return page("Mobile IA", '''
<div><h1>Orion Mobile: information architecture</h1>
<div class="lead">The app is a notification channel next to email, webhook and the Console inbox. The notification is the front door; every screen exists to explain it or act on it. Configuration stays in Console.</div></div>
<div class="blk"><div><h2>Navigation model</h2><div class="mu">Five tabs. Notifications is home. Orion Core context (which Core you are looking at) sits in every header, never in the tab bar.</div></div>%s
<div class="grid3">%s%s%s</div></div>
<div class="blk"><div><h2>Notification to screen</h2><div class="mu">Every entry point opens with the tab stack already built, so Back lands on the feed, not on a dead end.</div></div>%s</div>
<div class="blk"><div><h2>Surface to Core API</h2><div class="mu">Everything except pairing, devices and the push channel is already in the Core API.</div></div>%s</div>
<div class="grid2"><div class="box"><h2>Rules</h2><ul><li>One Core is active at a time. Switching keeps each Core&rsquo;s tab state.</li><li>Push opens in the Core it came from, switching automatically.</li><li>Write actions are one tap plus a sheet: Acknowledge, Resolve, Note, Take, Pause, Retry. Public updates add a confirm step.</li><li>Cached data shows when Core is unreachable, with a banner and last sync time. Actions are disabled, not queued.</li><li>Which alerts reach the phone is decided by routes in Console. The app only controls quiet hours, mute and sounds on this device.</li></ul></div>
<div class="box"><h2>Stays in Console, not on mobile</h2><ul><li>Creating or editing monitors, rules, routes and channels</li><li>Status page building, domains and subscribers</li><li>API tokens, registration tokens, data lifecycle, audit log</li><li>Installing the Orion Server on a host</li></ul><h2 style="margin-top:8px">Added to Console for mobile</h2><ul><li>Settings, Mobile: pairing QR, device list, revoke %s</li><li>Alerts, Channels: add a Mobile channel and pick devices %s</li></ul></div></div>
''' % (tabs, pre, glob, entry, tbl, api, N, N), False, w=1280, h=2340, extra_css=PCSS, root_class="p")


FLOWS = [
    ("1 First connect with QR", [("Install app", ""), ("Welcome", "Scan QR"), ("Console shows QR", "Settings, Mobile"), ("Scan, exchange code", "One time, 10 min"), ("Allow notifications", "Optional Critical alerts"), ("k", "Feed, first test push", "Send test"), ("x", "Code expired", "Generate a new one")]),
    ("2 Connect by address", [("Welcome", "Enter address"), ("Check address", "Orion Core found"), ("Sign in", "Username, password"), ("Allow notifications", ""), ("k", "Feed", ""), ("x", "http:// warning", "Continue anyway or stop")]),
    ("3 Push to acknowledged", [("x", "Incident opens", "Rule matches"), ("Push on lock screen", "Acknowledge, Open"), ("Incident detail", "Timeline"), ("Acknowledge sheet", "Note, take role"), ("Fix on host", "Outside the app"), ("k", "Recovers, resolved push", "Timeline closed")]),
    ("4 Resolve with a note", [("Incident detail", ""), ("Resolve sheet", "What fixed it"), ("Confirm", ""), ("k", "Resolved, back to feed", "Timeline kept"), ("x", "Offline", "Action disabled, retry")]),
    ("5 Take or cover an incident", [("Incident detail", "Unassigned"), ("Who is on this", "Take, cover, hand off"), ("Confirm", ""), ("Others get push", "Responder changed"), ("k", "Name on incident", "")]),
    ("6 Triage a candidate", [("Push: needs review", ""), ("Incidents, Needs review", "Signal and reason"), ("Open incident", "Or Dismiss"), ("k", "Incident created, rules fire", ""), ("Signal recovers", "Candidate clears itself")]),
    ("7 Investigate with logs", [("Incident detail", "Logs tab"), ("Log list", "Filtered to incident"), ("Live tail", "Pause, filters"), ("Log detail", "Fields, copy"), ("k", "Surrounding lines", "Cause found")]),
    ("8 Quiet a noisy monitor", [("Monitor detail", ""), ("Run test now", "Result sheet"), ("Pause", "Confirm"), ("k", "Paused chip in list", "Resume any time"), ("x", "Test fails", "Open incident")]),
    ("9 Public status update", [("Incident detail", ""), ("Post update sheet", "Preview public text"), ("Confirm subscribers", "Count shown"), ("k", "Posted, in timeline", ""), ("x", "Page not published", "Open Console to publish")]),
    ("10 Failed delivery", [("x", "Webhook fails", "502"), ("Alert deliveries", "Failed filter"), ("Delivery detail", "Attempts"), ("Retry", ""), ("k", "Delivered", ""), ("x", "Still failing", "Fix channel in Console")]),
    ("11 Quiet hours and Critical alerts", [("More, Notifications", ""), ("Set 22:00 to 07:00", ""), ("Critical still sounds", "Toggle"), ("Send test notification", "Check phone"), ("k", "Rules respected", "")]),
    ("12 Second Core", [("Header, Core chip", ""), ("Core switcher sheet", "Unread counts"), ("Add a Core", "Same connect flow"), ("k", "Feed for Home lab", ""), ("Push for other Core", "Switches Core on open")]),
    ("13 Core unreachable", [("x", "Request fails", ""), ("Offline banner", "Last sync time"), ("Cached feed", "Actions disabled"), ("Retry", ""), ("k", "Back online", "Banner clears"), ("Push still arrives", "Relay is separate")]),
    ("14 Device removed or session expired", [("x", "Admin revokes device", "Console"), ("Next request 401", ""), ("Device removed screen", "Scan new QR"), ("Or session expired sheet", "Sign in again"), ("k", "Back where you were", "")]),
    ("15 Widget and Live Activity", [("x", "Incident opens", "Live Activity starts"), ("Lock screen card", "Stage, timer"), ("Tap Acknowledge", "Opens sheet"), ("k", "Resolves, activity ends", ""), ("Widget refresh", "Counts only")]),
    ("16 Lock and privacy", [("Open app", "Face ID"), ("Fails 3 times", "Passcode"), ("Notification text", "Built on device"), ("k", "Details never leave your Core", "Relay sees opaque id")]),
]


def flows():
    out = []
    for title, steps in FLOWS:
        cells = []
        for s in steps:
            if len(s) == 3:
                k, t, sm = s
            else:
                k, (t, sm) = "", s
            cells.append('<div class="stp %s">%s%s</div>' % (k, t, ("<small>%s</small>" % sm) if sm else ""))
        out.append('<div class="fl2"><b>%s</b><div class="st2">%s</div></div>' % (title, '<span class="a2"></span>'.join(cells)))
    h = 2100
    return page("Mobile flows", '<div><h1>Orion Mobile: flows</h1><div class="lead" style="margin-bottom:6px">Sixteen journeys. Red steps are failures, green steps are the goal.</div></div><div>%s</div>' % "".join(out),
                False, w=1280, h=h, extra_css=PCSS, root_class="p"), h
