"""Mobile screens, part B: servers, monitors, logs, deliveries, more, states."""
from common import ic
from screens_a import hd, tabs, sc, sheet


def spark(color="var(--ac)", pts=None, w=72, h=24, dash=False):
    pts = pts or [14, 12, 15, 11, 13, 9, 12, 10, 8, 11, 7, 9]
    n = len(pts)
    p = " ".join("%d,%d" % (i * (w - 2) / (n - 1) + 1, h - v) for i, v in enumerate(pts))
    return '<svg width="%d" height="%d" viewBox="0 0 %d %d" aria-hidden="true"><polyline points="%s" fill="none" stroke="%s" stroke-width="1.6"/></svg>' % (w, h, w, h, p, color)


def chart(h=140, bad=False):
    base = [30, 28, 31, 29, 33, 30, 28, 32, 31, 29, 30, 34, 31, 30, 33, 100, 100, 100] if bad else [30, 28, 31, 29, 33, 30, 28, 32, 31, 29, 30, 34, 31, 30, 33]
    n = len(base)
    w = 326
    pts = " ".join("%d,%d" % (i * w / (n - 1), h - 16 - v * 0.9) for i, v in enumerate(base))
    area = "0,%d %s %d,%d" % (h - 16, pts, w, h - 16)
    grid = "".join('<line x1="0" x2="%d" y1="%d" y2="%d" stroke="var(--bd)" stroke-dasharray="3 4"/>' % (w, y, y) for y in (20, 60, 100))
    return ('<svg width="100%%" height="%d" viewBox="0 0 %d %d" role="img" aria-label="Response time, last 24 hours">%s'
            '<polygon points="%s" fill="var(--acs)"/><polyline points="%s" fill="none" stroke="var(--ac)" stroke-width="1.8"/>'
            '<line x1="0" x2="%d" y1="40" y2="40" stroke="var(--dnx)" stroke-dasharray="5 4"/></svg>') % (h, w, h, grid, area, pts, w)


def uptime(n=30, bad=(27, 28, 29), warn=(12,)):
    return '<div class="uptime">%s</div>' % "".join('<i class="%s"></i>' % ("d" if i in bad else "w" if i in warn else "") for i in range(n))


# ---------- servers ----------
def srv_row(shape, name, sub, cpu, mem, last, badge=""):
    return ('<a href="#" class="li" style="min-height:68px"><span class="s %s"></span><div class="t"><b>%s</b><small>%s</small></div>'
            '<div class="col g4" style="align-items:flex-end"><span class="xs mu" style="white-space:nowrap">CPU %s</span><span class="xs mu" style="white-space:nowrap">%s</span>%s</div></a>') % (shape, name, sub, cpu, last, badge)


def servers_list(d):
    rows = (srv_row("dn", "edge-01", "Ubuntu 24.04 &middot; 2 monitors down", "&ndash;", "&ndash;", "31m ago", '<span class="b dn">Not reporting</span>') +
            srv_row("dg", "db-02", "Debian 12 &middot; disk 91%", "34%", "71%", "12s ago") +
            srv_row("up", "edge-02", "Ubuntu 24.04", "12%", "44%", "8s ago") +
            srv_row("up", "edge-04", "Ubuntu 24.04", "9%", "39%", "5s ago") +
            srv_row("up", "cache-01", "Debian 12", "21%", "58%", "14s ago") +
            srv_row("mt", "build-01", "macOS 15 &middot; maintenance", "&ndash;", "&ndash;", "Paused", ""))
    return (hd(right=("search",)) + '<h1 class="ttl">Servers</h1><div class="sub">5 of 6 reporting</div>' +
            sc('<div class="chips"><span class="chip on">All 6</span><span class="chip">Needs attention 2</span><span class="chip">Maintenance 1</span></div><div class="list">%s</div>' % rows) + tabs("Servers"))


def tile(label, val, unit, color="var(--ac)", pts=None):
    return ('<div class="card cp col g4"><span class="xs mu">%s</span><div class="row sp"><b style="font-size:22px;font-weight:500">%s<small class="mu" style="font-size:13px;font-weight:400">%s</small></b>%s</div></div>') % (label, val, unit, spark(color, pts, 56, 22))


def server_detail(d):
    mons = ('<div class="list"><a href="#" class="li"><span class="s dn"></span><div class="t"><b>api-https</b><small>HTTP &middot; timeout after 10s</small></div>%s</a>'
            '<a href="#" class="li"><span class="s un"></span><div class="t"><b>edge-01-disk</b><small>Agent &middot; no data</small></div>%s</a></div>') % (ic("chev", 18), ic("chev", 18))
    return ('<div class="hd" style="padding-bottom:0"><a href="#" class="core" aria-label="Back">%s<span>Servers</span></a><a href="#" class="ibtn" aria-label="More">%s</a></div>'
            '<div class="sc" style="padding:4px 0 0;gap:12px"><div class="pad col g8"><div class="row g8"><span class="s dn"></span><h1 style="font-size:24px;margin:0;font-weight:500">edge-01</h1><span class="b dn">Not reporting</span></div>'
            '<span class="sm mu">Ubuntu 24.04 &middot; orion-agent 0.14 &middot; last report 31 min ago</span></div>'
            '<div class="call dg" style="margin:0 16px">%s<div>edge-01 stopped reporting at 14:01. Its 2 monitors show Unknown. Check the host or put it in maintenance.</div></div>'
            '<div class="ut"><a href="#" class="on">Overview</a><a href="#">Monitors 2</a><a href="#">Reports</a><a href="#">Logs</a></div>'
            '<div class="pad" style="display:grid;grid-template-columns:1fr 1fr;gap:8px">%s%s%s%s</div><div class="sec">Monitors on this server</div>%s</div>') % (
        ic("back", 22), ic("more", 22), ic("alert", 20), tile("CPU, last seen", "41", "%", "var(--fa)"), tile("Memory", "62", "%", "var(--fa)"),
        tile("Disk", "58", "%", "var(--fa)"), tile("Load 1m", "0.84", "", "var(--fa)"), mons)


# ---------- monitors ----------
def mon_row(shape, name, kind, up, state=""):
    return ('<a href="#" class="li" style="min-height:64px"><span class="s %s"></span><div class="t"><b>%s</b><small>%s &middot; %s</small></div>%s<span class="mono" style="width:52px;text-align:right">%s</span></a>') % (
        shape, name, kind, up, spark("var(--dnx)" if shape == "dn" else "var(--ac)", [12, 12, 13, 12, 12, 3, 2] if shape == "dn" else None, 56, 22), up.split()[0])


def monitors_list(d):
    rows = (mon_row("dn", "api-https", "HTTP", "99.20% 30d") + mon_row("dg", "disk-usage", "Agent", "99.80% 30d") +
            mon_row("up", "web-https", "HTTP", "100% 30d") + mon_row("up", "lb-https", "HTTP", "99.97% 30d") +
            mon_row("up", "nightly-backup", "Heartbeat", "100% 30d") + mon_row("un", "legacy-tcp", "TCP", "Paused"))
    return (hd(right=("search", "filter")) + '<h1 class="ttl">Monitors</h1><div class="sub">21 of 24 up</div>' +
            sc('<div class="chips"><span class="chip on">All 24</span><span class="chip"><span class="s dn"></span>Down 1</span><span class="chip"><span class="s dg"></span>Degraded 1</span><span class="chip">Paused 1</span></div><div class="list">%s</div>' % rows) + tabs("Monitors"))


def monitor_body(d):
    return ('<div class="hd" style="padding-bottom:0"><a href="#" class="core" aria-label="Back">%s<span>Monitors</span></a><a href="#" class="ibtn" aria-label="More">%s</a></div>'
            '<div class="sc" style="padding:4px 16px 0;gap:12px"><div class="row g8"><span class="s dn"></span><h1 style="font-size:24px;margin:0;font-weight:500">api-https</h1><span class="b dn">Down</span></div>'
            '<span class="mono">GET https://api.example.com/health</span>'
            '<div class="card cp col g8"><div class="row sp sm"><b class="med">Response time</b><span class="mu">24 hours</span></div>%s<div class="row sp xs mu"><span>14:00</span><span>timeout 10s</span></div></div>'
            '<div class="card cp col g8"><div class="row sp sm"><b class="med">Uptime, 30 days</b><b>99.20%%</b></div>%s</div>'
            '<div class="st4"><div><span>Avg</span><b>184<small>ms</small></b></div><div><span>Checks</span><b>43k</b></div><div><span>Last</span><b style="color:var(--dnt)">Fail</b></div></div></div>') % (
        ic("back", 22), ic("more", 22), chart(120, True), uptime())


def monitor_detail(d):
    return monitor_body(d) + '<div class="bar"><a href="#" class="btn p f">%s Run test now</a><a href="#" class="btn f">%s Pause</a></div>' % (ic("play", 16), ic("pause", 16))


def monitor_test(d):
    return monitor_body(d) + sheet("Test run",
        '<div class="call" style="background:var(--dnb);color:var(--dnt);border-color:transparent">%s<div><b>Failed</b><br>Timed out after 10 s. No response from api.example.com:443.</div></div>'
        '<div class="card cp col g8 sm"><div class="row sp"><span class="mu">DNS</span><span>12 ms</span></div><div class="row sp"><span class="mu">Connect</span><span style="color:var(--dnt)">timeout</span></div><div class="row sp"><span class="mu">TLS</span><span class="fa">&ndash;</span></div></div>'
        '<p class="xs mu" style="margin:0">A test run does not change the monitor or open an incident.</p>'
        '<div class="row g8"><a href="#" class="btn f">Close</a><a href="#" class="btn p f">Run again</a></div>' % ic("x", 20))


# ---------- logs ----------
def lg(t, lv, msg, src=""):
    c = {"E": "e", "W": "w", "I": "i"}[lv]
    return '<div class="log"><time>%s</time><span class="lv %s">%s</span><p>%s</p></div>' % (t, c, {"E": "ERR", "W": "WARN", "I": "INFO"}[lv], msg)


LOGS = (lg("14:32:08", "E", "upstream timed out (110: Connection timed out) while connecting to upstream, client 10.0.4.2") +
        lg("14:32:07", "E", "health check failed: GET /health context deadline exceeded") +
        lg("14:32:02", "W", "retrying connection to db-02:5432, attempt 3") +
        lg("14:31:58", "I", "reload requested by systemd") +
        lg("14:31:40", "E", "worker 4 exited with status 137 (OOM kill)") +
        lg("14:31:12", "I", "request GET /v1/orders 200 41ms") + lg("14:31:02", "W", "slow query 2.4s on orders") +
        lg("14:30:55", "I", "request GET /health 200 3ms"))


def logs(d):
    hist = '<div class="row" style="gap:2px;height:36px;align-items:flex-end;margin:0 16px 12px">%s</div>' % "".join(
        '<i style="flex:1;background:%s;height:%dpx"></i>' % ("var(--dnx)" if i > 20 else "var(--bd)", h) for i, h in enumerate([8, 6, 9, 7, 8, 6, 10, 7, 9, 8, 7, 6, 9, 8, 7, 10, 8, 9, 12, 16, 24, 30, 34, 28]))
    return (hd(back="More", right=("search",)) + '<h1 class="ttl">Logs</h1><div class="ut"><a href="#">Events</a><a href="#" class="on">Service logs</a></div>' +
            '<div class="row g8" style="padding:0 16px 12px"><div class="fld" style="flex:1;min-height:44px"><span class="fa">%s</span><span class="v fa">level:error api</span></div><a href="#" class="btn" style="min-width:44px;padding:0 12px"><span class="s dn" style="width:8px;height:8px;border-radius:50%%;background:var(--dnx)"></span>Live</a></div>' % ic("search", 18) +
            sc('<div class="chips"><span class="chip on">Error+ %s</span><span class="chip">api-gateway %s</span><span class="chip">Last hour %s</span></div>%s<div style="background:var(--sf);border-top:1px solid var(--bd)">%s</div>' % (ic("x", 12), ic("down", 12), ic("down", 12), hist, LOGS)) + tabs("More"))


def logs_live(d):
    return ('<div class="hd" style="padding-bottom:0"><a href="#" class="core" aria-label="Back">%s<span>Logs</span></a><span class="b" style="background:var(--dnb);color:var(--dnt)"><span class="s dn" style="width:8px;height:8px;border-radius:50%%"></span>Live</span></div>'
            '<div class="row sp" style="padding:8px 16px"><span class="sm mu">api-gateway &middot; Error+ &middot; 14 lines/s</span></div>'
            '<div class="sc" style="background:var(--sf);border-top:1px solid var(--bd);position:relative">%s%s</div>'
            '<div class="bar" style="padding-bottom:30px"><a href="#" class="btn f">%s Pause</a><a href="#" class="btn f">Filters</a></div>') % (
        ic("back", 22), '<div style="position:absolute;left:0;right:0;top:12px;display:flex;justify-content:center"><span class="btn sm p" style="min-height:32px">%s 18 new lines</span></div>' % ic("down", 14) + LOGS, "", ic("pause", 16))


def log_detail(d):
    kv = lambda k, v: '<div class="row sp sm" style="padding:10px 0;border-top:1px solid var(--bd);align-items:flex-start"><span class="mu">%s</span><span class="mono" style="color:var(--tx);text-align:right;max-width:210px;word-break:break-all">%s</span></div>' % (k, v)
    return (LOGS and hd(back="Logs", right=("more",)) + '<div class="sc" style="padding:4px 16px 0;gap:12px"><div class="row g8"><span class="lv e">ERR</span><span class="mono">14:32:08.412</span></div>'
            '<p style="margin:0;font-size:16px;line-height:1.4">upstream timed out (110: Connection timed out) while connecting to upstream, client 10.0.4.2</p>'
            '<div class="card cp" style="padding:4px 14px">%s%s%s%s%s</div>'
            '<div class="row g8"><a href="#" class="btn f">%s Copy</a><a href="#" class="btn f">Surrounding lines</a></div></div>' % (
                kv("service", "api-gateway"), kv("host", "edge-01"), kv("request_id", "9f2c-71ab-44e0"), kv("upstream", "10.0.4.9:8080"), kv("trace", "b81f0e"), ic("copy", 16)))


# ---------- deliveries ----------
def dlv(icon, title, rule, st, when, retry=False):
    stb = {"Sent": "up", "Failed": "dn", "Retrying": "dg"}[st]
    r = '<span class="btn sm" style="margin-top:8px">Retry</span>' if retry else ""
    return ('<a href="#" class="li" style="align-items:flex-start;padding:14px 16px"><span style="color:var(--mu);padding-top:2px">%s</span><div class="t"><b style="white-space:normal">%s</b><small>%s</small>%s</div>'
            '<div class="col g4" style="align-items:flex-end"><span class="b %s">%s</span><span class="xs mu">%s</span></div></a>') % (ic(icon, 20), title, rule, r, stb, st, when)


def deliveries(d):
    rows = (dlv("hook", "ops-pager &middot; INC-0231", "Rule: Critical to on-call &middot; 502 Bad Gateway", "Failed", "14:03", True) +
            dlv("phone", "This iPhone &middot; INC-0231", "Rule: Critical to on-call", "Sent", "14:02") +
            dlv("mail", "ops@example.com &middot; INC-0231", "Rule: Critical to on-call", "Sent", "14:02") +
            dlv("inbox", "Console inbox &middot; INC-0231", "Rule: Critical to on-call", "Sent", "14:02") +
            dlv("phone", "This iPhone &middot; INC-0230", "Rule: High to team", "Sent", "Yesterday"))
    return (hd(back="More", right=("search",)) + '<h1 class="ttl">Alert deliveries</h1><div class="seg"><a href="#" class="on">All</a><a href="#">Failed 1</a><a href="#">This device</a></div>' +
            sc('<div class="list">%s</div>' % rows) + tabs("More"))


def delivery_detail(d):
    at = lambda n, t, r, bad=False: '<div class="row sp sm" style="padding:10px 0;border-top:1px solid var(--bd)"><span>Attempt %s &middot; %s</span><span style="color:%s">%s</span></div>' % (n, t, "var(--dnt)" if bad else "var(--upt)", r)
    return (hd(back="Deliveries", right=("more",)) + '<div class="sc" style="padding:4px 16px 0;gap:12px"><div class="row g8"><span class="mono">INC-0231</span><span class="b dn">Failed</span></div>'
            '<h1 style="font-size:22px;margin:0;font-weight:500">ops-pager webhook</h1><span class="sm mu">Rule: Critical to on-call &middot; opened 14:02</span>'
            '<div class="card cp" style="padding:4px 14px">%s%s%s</div><div class="call dg">%s<div>ops-pager returned 502 each time. Check the endpoint in Console, Alerts, Channels. Retrying here sends the same payload again.</div></div></div>'
            '<div class="bar"><a href="#" class="btn p f">%s Retry delivery</a></div>') % (
        at(1, "14:02:11", "502", True) + "", at(2, "14:02:41", "502", True), at(3, "14:03:41", "502", True), ic("alert", 20), ic("refresh", 16))


# ---------- more ----------
def more(d):
    li = lambda i, t, s="", b="": '<a href="#" class="li"><span style="color:var(--mu)">%s</span><div class="t"><b>%s</b>%s</div>%s%s</a>' % (ic(i, 22), t, ("<small>%s</small>" % s) if s else "", b, ic("chev", 18))
    return (hd(right=()) + '<h1 class="ttl">More</h1>' + sc(
        '<div class="pad" style="margin-bottom:8px"><a href="#" class="card cp row" style="text-decoration:none;color:inherit"><span class="s up"></span><div class="col g4" style="flex:1"><b class="med">Production</b><span class="xs mu">orion.example.com &middot; Orion 0.14 &middot; 2 issues</span></div><span class="btn sm">Switch</span></a></div>'
        '<div class="list">%s%s%s</div><div class="sec">App</div><div class="list">%s%s</div>'
        '<div class="pad" style="padding-top:20px"><a href="#" class="btn f" style="width:100%%;color:var(--dnt)">Forget this Core</a><p class="xs fa ctr" style="margin:14px 0 0">Orion Mobile 1.0 (24) &middot; This device: iPhone 15</p></div>' % (
            li("log", "Logs", "Events and service logs"), li("send", "Alert deliveries", "What was sent where", '<span class="b dn">1 failed</span>'), li("search", "Search", "Servers, monitors, incidents"),
            li("bell", "Notifications", "Quiet hours, Critical alerts"), li("gear", "Settings", "Face ID, theme, time zone"))) + tabs("More"))


def search(d):
    res = ('<div class="sec">Incidents</div><div class="list"><a href="#" class="li"><span class="s dn"></span><div class="t"><b>api.example.com unreachable</b><small>INC-0231 &middot; Open</small></div></a></div>'
           '<div class="sec">Monitors</div><div class="list"><a href="#" class="li"><span class="s dn"></span><div class="t"><b>api-https</b><small>HTTP &middot; Down</small></div></a><a href="#" class="li"><span class="s up"></span><div class="t"><b>api-grpc</b><small>TCP &middot; Up</small></div></a></div>'
           '<div class="sec">Servers</div><div class="list"><a href="#" class="li"><span class="s dg"></span><div class="t"><b>api-worker-02</b><small>Debian 12 &middot; Degraded</small></div></a></div>')
    return ('<div class="hd" style="padding-bottom:8px"><div class="fld f" style="flex:1;min-height:44px"><span class="fa">%s</span><span class="v">api</span><span class="fa">%s</span></div><a href="#" class="btn g" style="padding:0 0 0 14px">Cancel</a></div>' % (ic("search", 18), ic("x", 16)) +
            sc('<div class="chips"><span class="chip on">All</span><span class="chip">Incidents</span><span class="chip">Servers</span><span class="chip">Monitors</span></div>' + res))


def settings(d):
    li = lambda t, s, right: '<div class="li"><div class="t"><b>%s</b>%s</div>%s</div>' % (t, ("<small>%s</small>" % s) if s else "", right)
    tg = lambda on: '<span class="tg%s"></span>' % (" on" if on else "")
    ch = lambda v: '<span class="row g8 sm mu">%s%s</span>' % (v, ic("chev", 16))
    return (hd(back="More", right=()) + '<h1 class="ttl">Settings</h1>' + sc(
        '<div class="sec" style="padding-top:4px">Security</div><div class="list">%s%s</div>'
        '<div class="sec">Display</div><div class="list">%s%s</div>'
        '<div class="sec">This device</div><div class="list">%s%s%s</div>'
        '<p class="hint pad">Device name and pairing are managed in Console, Settings, Mobile. An admin can remove this device there.</p>' % (
            li("Face ID lock", "Ask when the app opens", tg(True)), li("Lock after", "", ch("Immediately")),
            li("Theme", "", ch("System")), li("Times shown in", "", ch("Core time zone (UTC)")),
            li("Name", "", '<span class="sm mu">iPhone 15</span>'), li("Paired", "", '<span class="sm mu">Oct 16, 2026</span>'), li("Clear cached data", "Servers, monitors, incidents", ic("chev", 16)))) + tabs("More"))


def settings_notifications(d):
    li = lambda t, s, right: '<div class="li"><div class="t"><b style="white-space:normal">%s</b>%s</div>%s</div>' % (t, ("<small style='white-space:normal'>%s</small>" % s) if s else "", right)
    tg = lambda on: '<span class="tg%s"></span>' % (" on" if on else "")
    ch = lambda v: '<span class="row g8 sm mu">%s%s</span>' % (v, ic("chev", 16))
    return (hd(back="More", right=()) + '<h1 class="ttl">Notifications</h1>' + sc(
        '<div class="pad" style="margin-bottom:4px"><div class="call"><span style="color:var(--mu)">%s</span><div class="mu">Production decides what reaches this phone through routes in Console. These settings only change how this phone behaves.</div></div></div>'
        '<div class="sec">Production</div><div class="list">%s%s%s</div>'
        '<div class="sec">Quiet hours</div><div class="list">%s%s%s</div><div class="pad" style="padding-top:16px"><a href="#" class="btn f" style="width:100%%">%s Send a test notification</a></div>' % (
            ic("globe", 20),
            li("Push notifications", "", tg(True)), li("Critical alerts", "Critical incidents break through Do Not Disturb", tg(True)), li("Mute this Core", "Pause everything from Production", ch("Off")),
            li("Quiet hours", "Silence push at night", tg(True)), li("Schedule", "", ch("22:00 to 07:00")), li("Critical still sounds", "", tg(True)), ic("send", 16))) + tabs("More"))


def core_switcher(d):
    row = lambda sh, n, s, on, bad="": ('<div class="row g12" style="padding:12px;border:1px solid %s;border-radius:8px;background:var(--sf)"><span class="s %s"></span><div class="col g4" style="flex:1"><b class="med">%s</b><span class="xs mu">%s</span></div>%s</div>') % ("var(--ac)" if on else "var(--bd)", sh, n, s, ic("check", 18) if on else bad)
    return (hd() + '<h1 class="ttl">Notifications</h1>' + sc('<div class="st4" style="margin:0 16px"><div><span>Incidents</span><b>2</b></div><div><span>Servers</span><b>5</b> <small>of 6</small></div><div><span>Monitors</span><b>21</b> <small>of 24</small></div></div>') + tabs("Notifications") +
            sheet("Your Cores", '<div class="col g8">%s%s</div><a href="#" class="btn">%s Add a Core</a>' % (
                row("dg", "Production", "orion.example.com &middot; 2 issues", True),
                row("up", "Home lab", "nas.local:8080 &middot; all healthy &middot; 2 unread", False, '<span class="b un">Muted</span>'),
                ic("plus", 18))))


# ---------- states ----------
def revoked(d):
    return ('<div class="sc ctr" style="padding:150px 28px 0;gap:14px"><div style="color:var(--dnt);background:var(--dnb);width:64px;height:64px;display:flex;align-items:center;justify-content:center">%s</div>'
            '<h1 class="ttl" style="padding:8px 0 0">This device was removed</h1><p class="mu" style="margin:0">An admin removed this phone from Production in Console. It no longer receives alerts. Pair it again to keep getting them.</p></div>'
            '<div class="col g8" style="padding:0 24px 40px"><a href="#" class="btn p">%s Scan a new QR code</a><a href="#" class="btn g">Forget Production</a></div>') % (ic("shield", 30), ic("qr", 20))


def session_expired(d):
    return (hd(right=()) + '<h1 class="ttl">Notifications</h1>' + sc('<div class="pad col g8"><div class="card cp" style="height:96px;opacity:.5"></div><div class="card cp" style="height:96px;opacity:.5"></div></div>') + tabs("Notifications") +
            sheet("Sign in again", '<p class="mu sm" style="margin:-6px 0 0">Your session on Production expired. Your cached data stays on this phone.</p>'
                  '<div><label class="lbl">Username</label><div class="fld"><span class="v">admin</span><span class="fa">%s</span></div></div>'
                  '<div><label class="lbl">Password</label><div class="fld f"><span class="v">&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;</span></div></div>'
                  '<a href="#" class="btn p">Sign in</a>' % ic("shield", 16)))
