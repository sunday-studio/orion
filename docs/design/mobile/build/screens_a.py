"""Mobile screens, part A: connect, notifications, incidents."""
from common import ic
from parts import hd, dhd, tabs, sc, sheet, title, tracker, expected_actual, box, bars, stats, tl2, logo, si, rw, gh, pill












# ---------- connect ----------


def scan(d):
    corners = "".join('<i style="%s"></i>' % s for s in [
        "left:0;top:0;border-right:0;border-bottom:0", "right:0;top:0;border-left:0;border-bottom:0",
        "left:0;bottom:0;border-right:0;border-top:0", "right:0;bottom:0;border-left:0;border-top:0"])
    return ('<div class="cam" style="position:absolute;inset:0"></div>'
            '<div class="hd" style="position:relative;color:#fafaf9"><a href="#" class="core" style="color:#fafaf9" aria-label="Close">%s</a>'
            '<a href="#" class="ibtn" style="color:#fafaf9" aria-label="Torch">%s</a></div>'
            '<div class="sc ctr" style="position:relative;justify-content:center;gap:28px;color:#fafaf9;padding-bottom:80px">'
            '<div class="qr">%s</div><div style="max-width:270px"><div style="font-size:18px;font-weight:500">Scan the pairing code</div>'
            '<p style="margin:6px 0 0;color:#d6d3d1;font-size:14px">Open Console, go to Settings, Mobile, then choose Pair a device. The code works once and expires in 10 minutes.</p></div></div>'
            '<div class="col" style="position:relative;padding:0 24px 40px"><a href="#" class="btn" style="background:rgba(250,250,249,.12);color:#fafaf9;border-color:rgba(250,250,249,.25)">Enter server address instead</a></div>') % (
        ic("x", 24), ic("flash", 22), corners)


def manual(d, insecure=False):
    url = "http://192.168.1.20:8080" if insecure else "https://orion.example.com"
    status = ('<div class="call dg">%s<div><b>This address is not encrypted</b><br>Anyone on this network can read what the app sends, including your sign-in. Use it only on a network you trust.</div></div>' % ic("alert", 20)
              if insecure else
              '<div class="call up">%s<div><b>Reached Production</b><br>Orion Core 0.14 &middot; HTTPS &middot; certificate valid</div></div>' % ic("check", 20))
    btn = '<a href="#" class="btn p">Continue anyway</a>' if insecure else '<a href="#" class="btn p">Sign in</a>'
    creds = '' if insecure else (
        '<div><label class="lbl">Username</label><div class="fld"><span class="v">admin</span></div></div>'
        '<div><label class="lbl">Password</label><div class="fld f"><span class="v">&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;</span></div></div>')
    return ('<div class="hd"><a href="#" class="core" aria-label="Back">%s<span>Connect</span></a></div>'
            '<div class="sc g16" style="padding:8px 16px;gap:16px"><h1 class="ttl" style="padding:0">Enter server address</h1>'
            '<div><label class="lbl">Address</label><div class="fld"><span class="v">%s</span><span class="fa">%s</span></div>'
            '<div class="hint">The URL you use to open Console.</div></div>%s%s</div>'
            '<div class="col g8" style="padding:0 16px 40px">%s<a href="#" class="btn g">Scan QR code instead</a></div>') % (
        ic("back", 22), url, ic("check" if not insecure else "alert", 18), status, creds, btn)


def paired(d):
    def row(i, t, s, on, last=False):
        return ('<div class="row" style="align-items:flex-start;gap:14px;padding:14px 0;%s"><div style="color:var(--ac);padding-top:2px">%s</div>'
                '<div class="col g4" style="flex:1"><b class="med">%s</b><span class="sm mu">%s</span></div><span class="tg%s"></span></div>') % (
            "" if last else "border-bottom:1px solid var(--bd)", ic(i, 22), t, s, " on" if on else "")
    return ('<div class="sc" style="padding:96px 16px 0;gap:6px"><div class="b up" style="align-self:flex-start">%s Paired</div>'
            '<h1 class="ttl" style="padding:8px 0 0">Connected to Production</h1>'
            '<p class="mu" style="margin:0 0 18px">This phone is now a notification channel. Choose how it reaches you.</p>'
            '<div class="card" style="padding:0 14px">%s%s%s</div>'
            '<p class="hint">Which alerts reach this phone is set by routes in Console. You can change these later in Settings.</p></div>'
            '<div class="col g8" style="padding:0 16px 40px"><a href="#" class="btn p">Allow notifications</a><a href="#" class="btn g">Not now</a></div>') % (
        ic("check", 14, 2.2),
        row("bell", "Notifications", "Incidents, silent servers and failing monitors.", True),
        row("flash", "Critical alerts", "Let Critical incidents break through Do Not Disturb.", False),
        row("face", "Lock with Face ID", "Ask for Face ID when the app opens.", True, True))


# ---------- notifications ----------






def lockscreen(d):
    def n(app, t, body, extra=""):
        return ('<div class="ntf"><div class="row sp xs" style="color:#d6d3d1"><span class="row g8">%s Orion &middot; Production</span><span>now</span></div>'
                '<b style="font-size:15px">%s</b><span style="font-size:14px;color:#e7e5e4">%s</span>%s</div>') % (ic("pulse", 14), t, body, extra)
    acts = '<div class="row g8" style="margin-top:8px"><span class="btn sm f" style="background:rgba(250,250,249,.16);color:#fafaf9;border:0">Acknowledge</span><span class="btn sm f" style="background:rgba(250,250,249,.16);color:#fafaf9;border:0">Open</span></div>'
    return ('<div class="lock" style="position:absolute;inset:0"></div><div class="sc ctr" style="position:relative;padding:96px 16px 0;gap:6px;color:#fafaf9">'
            '<div style="font-size:15px;color:#d6d3d1">Thursday, October 16</div><div style="font-size:84px;line-height:1;font-weight:300;margin-bottom:36px">14:32</div>'
            '<div class="col g8" style="width:100%%;text-align:left">%s%s%s</div></div>') % (
        n("", "Critical: api.example.com is down", "INC-0231 &middot; api-https failing for 30 min. No one has acknowledged.", acts),
        n("", "edge-01 stopped reporting", "No report for 31 min. 2 monitors are Unknown."),
        n("", "Resolved: nightly-backup", "INC-0227 closed after 1h 12m."))


def widgets(d):
    small = ('<div class="card cp col sp" style="width:158px;height:158px"><div class="row g8 xs mu">%s Production</div>'
             '<div><div style="font-size:34px;font-weight:500;line-height:1">2</div><div class="sm mu">open incidents</div></div>'
             '<div class="row g8 sm"><span class="s dn"></span><b class="med">api-https</b></div></div>') % ic("pulse", 14)
    medium = ('<div class="card cp col g8" style="width:338px;height:158px"><div class="row sp xs mu"><span class="row g8">%s Production</span><span>14:32</span></div>'
              '<div class="row g8"><span class="s dn"></span><b class="med" style="flex:1">api.example.com is down</b><span class="b cr">Critical</span></div>'
              '<div class="row g8"><span class="s dg"></span><b class="med" style="flex:1">db-02 disk above 90%%</b><span class="b dg">Ack</span></div>'
              '<div class="xs mu" style="margin-top:auto">Servers 5 of 6 &middot; Monitors 21 of 24</div></div>') % ic("pulse", 14)
    live = ('<div class="card cp col g12" style="width:358px"><div class="row sp xs mu"><span class="row g8">%s INC-0231 &middot; Production</span><span>Live Activity</span></div>'
            '<div class="row sp"><b style="font-size:17px">api.example.com is down</b><span class="b dn">Open</span></div>'
            '<div class="row g8 xs mu"><span class="b ok" style="padding:0">Detected</span>&rarr;<span>Notified</span>&rarr;<b style="color:var(--tx)">Acknowledge</b>&rarr;<span>Resolved</span></div>'
            '<div class="row sp"><span class="mono">Open 30:12</span><span class="btn sm p">Acknowledge</span></div></div>') % ic("alert", 14)
    island = '<div style="background:#000;color:#fafaf9;height:37px;width:180px;border-radius:19px;display:flex;align-items:center;justify-content:space-between;padding:0 14px;font-size:13px"><span style="color:#e5484d">&#9632;</span><span class="mono" style="color:#fafaf9">30:12</span></div>'
    return ('<div class="hd"></div><div class="sc" style="padding:30px 16px;gap:22px;align-items:center">'
            '<div class="xs fa" style="align-self:flex-start">Home screen widgets</div><div class="row g12">%s%s</div>%s'
            '<div class="xs fa" style="align-self:flex-start;margin-top:8px">Lock screen Live Activity</div>%s'
            '<div class="xs fa" style="align-self:flex-start;margin-top:8px">Dynamic Island, compact</div>%s</div>') % (
        small, '<div class="card cp col sp" style="width:158px;height:158px"><div class="row g8 xs mu">%s Home lab</div><div><div style="font-size:34px;font-weight:500;line-height:1">0</div><div class="sm mu">open incidents</div></div><div class="row g8 sm"><span class="s up"></span><b class="med">All healthy</b></div></div>' % ic("pulse", 14),
        medium, live, island)


# ---------- incidents ----------
def inc_row(shape, title, idn, sev, st, when, mon):
    return ('<a href="#" class="li" style="align-items:flex-start;padding:14px 16px"><span class="s %s" style="margin-top:6px"></span>'
            '<div class="t"><b style="white-space:normal">%s</b><small>%s &middot; %s</small><div class="row g8" style="margin-top:8px">%s%s</div></div>'
            '<span class="xs mu">%s</span></a>') % (shape, title, idn, mon, sev, st, when)




def incident_review(d):
    def c(t, why, sig):
        return ('<div class="card cp col g12"><div class="row g8"><span class="s un"></span><b class="med" style="font-size:15px">%s</b></div>'
                '<p class="sm mu" style="margin:0">%s</p><div class="row g8 xs"><span class="b un">%s</span><span class="mu">Seen 6 min ago</span></div>'
                '<div class="row g8"><a href="#" class="btn p f">Open incident</a><a href="#" class="btn f">Dismiss</a></div></div>') % (t, why, sig)
    return (hd(right=("search", "filter")) + '<h1 class="ttl">Incidents</h1><div class="seg"><a href="#">Active 3</a><a href="#" class="on">Needs review 1</a><a href="#">Resolved 21</a></div>' +
            sc('<div class="pad col g12"><p class="sm mu" style="margin:0">Signals Orion noticed that did not meet an incident rule. Open one or dismiss it. Signals that recover clear themselves.</p>%s</div>' %
               c("edge-02 missed 2 reports", "edge-02 skipped its last two 60 second reports, then resumed. No monitor is failing yet.", "Server signal")) +
            tabs("Incidents"))








def incident_resolve(d):
    return incident_body(d) + sheet("Resolve INC-0231",
        '<p class="mu sm" style="margin:-6px 0 0">Closes the incident and keeps the timeline. If api-https fails again, Orion opens a new one.</p>'
        '<div><label class="lbl">What fixed it?</label><div class="fld f" style="align-items:flex-start;min-height:96px;padding-top:12px"><span class="v mu">Restarted nginx on edge-01</span></div></div>'
        '<div class="row g8"><a href="#" class="btn f">Cancel</a><a href="#" class="btn p f">Resolve incident</a></div>')




def incident_public(d):
    return incident_body(d) + sheet("Post a status page update",
        '<div class="call dg">%s<div>This is public. Subscribers get an email and the page changes right away.</div></div>'
        '<div class="card cp col g8"><div class="row sp"><b class="med">Investigating</b><span class="xs mu">Public page: Orion Status</span></div><p class="sm" style="margin:0">We are investigating errors reaching the API. Next update in 30 minutes.</p><div class="row g8 xs"><span class="b un">Component: API</span><span class="b dn">Major outage</span></div></div>'
        '<div class="row g8 sm">%s<span>42 confirmed subscribers will be emailed.</span></div>'
        '<div class="row g8"><a href="#" class="btn f">Cancel</a><a href="#" class="btn p f">Post update</a></div>' % (ic("alert", 20), ic("mail", 16)))


def welcome(d):
    return ('<div class="deep" style="padding:104px 24px 40px;display:flex;flex-direction:column;gap:22px;flex:1">%s'
            '<div class="hero" style="font-size:46px;color:#fff">Know when it breaks.<br><span style="color:#80ffaa">Fix it from here.</span></div>'
            '<p style="margin:0;font-size:16px;line-height:1.5;color:#b9d6c6">Orion Mobile connects straight to the Orion Core you run. No Orion account, no cloud copy of your data.</p></div>'
            '<div class="col g12" style="padding:24px 24px 40px;background:var(--gr)"><a href="#" class="btn p">%s Scan QR code</a>'
            '<a href="#" class="btn">Enter server address</a>'
            '<p class="xs fa ctr" style="margin:6px 0 0">Find the QR code in Console, Settings, Mobile.</p></div>') % (
        logo(52), ic("qr", 20))


def _inbox(offline=False, swipe=False):
    first = rw("dn", "api.example.com is down", "Critical &middot; No one has acknowledged yet", "30m", True)
    if swipe:
        first = ('<div class="swp">%s<div class="acts"><span style="background:var(--ac)">%s Acknowledge</span><span style="background:#71717a">%s Snooze</span></div></div>') % (
            first, ic("check", 20, 2.2), ic("clock", 20))
    rows = (gh(None, "Today", "3") + first +
            rw("un", "edge-01 stopped reporting", "No report for 31 min &middot; 2 monitors Unknown", "31m", True) +
            rw("dn", "Delivery to oncall webhook failed", "502 &middot; 3 attempts &middot; INC-0231", "14:04") +
            gh(None, "Earlier", "3") +
            rw("dg", "db-02 disk above 90%", "You acknowledged this &middot; INC-0230", "16h") +
            rw("up", "edge-02 is reporting again", "Stopped reporting for 8 min", "Sun") +
            rw("up", "nightly-backup heartbeat resumed", "INC-0227 resolved after 1h 12m", "Sat"))
    return '<div style="opacity:.55">%s</div>' % rows if offline else rows


def feed(d, offline=False, swipe=False):
    ban = ('<div class="ban un">%s<span>Can&rsquo;t reach Production. Showing data from 4 min ago.</span><a href="#" class="med" style="margin-left:auto;color:var(--tx)">Retry</a></div>' % ic("off", 18)) if offline else ""
    fl = '<div class="chips" style="padding-bottom:10px"><span class="fp on">All</span><span class="fp">Unread 2</span><span class="fp">Incidents</span><span class="fp">Servers</span><span class="fp">Monitors</span></div>'
    return hd() + ban + '<h1 class="ttl2">Notifications</h1>' + fl + sc(_inbox(offline, swipe), 0) + tabs("Notifications")


def feed_empty(d):
    st = stats([("Monitors up", "24", "of 24"), ("Servers", "6", "of 6"), ("Uptime, 30d", "99.98%", "")])
    return (hd(dot="up") + '<h1 class="ttl2">Notifications</h1>' +
            sc('<div class="ctr" style="padding:56px 32px 28px;gap:10px"><span class="td up" style="width:44px;height:44px">%s</span><b style="font-size:17px;font-weight:600">All quiet, Ama</b>'
               '<span class="sm mu" style="max-width:270px">When an alert fires it lands here and on your lock screen. Nothing needs you right now.</span>'
               '<a href="#" class="btn sm" style="margin-top:8px">Send a test notification</a></div>%s' % (ic("check", 24, 2.4), st)) + tabs("Notifications", 0))


def incidents_list(d):
    rows = (gh("dn", "Open", "1") + rw("dn", "api.example.com unreachable", "INC-0231 &middot; Critical &middot; api-https", "30m", True) +
            gh("dg", "Acknowledged", "1") + rw("dg", "db-02 disk above 90%", "INC-0230 &middot; High &middot; disk-usage", "16h") +
            gh("cv", "Covered", "1") + rw("cv", "Load balancer degraded", "INC-0229 &middot; Medium &middot; lb-https", "2d"))
    fl = '<div class="chips" style="padding-bottom:10px"><span class="fp on">Active 3</span><span class="fp">Needs review 1</span><span class="fp">Resolved 21</span></div>'
    return (hd() + '<h1 class="ttl2">Incidents</h1>' + fl + sc(rows, 0) +
            '<a href="#" class="fab" aria-label="Declare incident">%s</a>' % ic("plus", 26, 2.2) + tabs("Incidents"))


def incident_body(d):
    pills = '<div class="pls">%s</div>' % (pill(si("dn", 16), "Open") + pill(ic("sev", 16, 2, 'style="color:var(--crit)"'), "Critical") +
                                          pill(ic("user", 16), "Unassigned", True) + pill(ic("pulse", 16), "api-https") + pill(ic("server", 16), "edge-01"))
    prose = '<p class="prose">The health check on <b>api-https</b> has timed out since 14:02. ops-team was notified; the on-call webhook returned 502, so nobody was paged.</p>'
    trk = '<div class="card" style="margin:0 16px;padding:14px 8px">%s</div>' % tracker([("Detected", "14:02", "done"), ("Notified", "14:02", "done"), ("Acknowledged", "waiting", "now"), ("Resolved", "-", "todo")])
    tl = tl2([("dg", "Delivery to <b>oncall webhook</b> failed after 3 attempts &middot; <u>Retry</u>", "14:04"),
              ("up", "Notified <b>this iPhone</b> and <b>ops-team</b> (email)", "14:02"),
              ("dn", "Incident opened: <b>api-https</b> failed 3 checks in a row", "14:02")])
    return (dhd("INC-0231") + '<div class="sc" style="gap:14px"><div class="pad col g12"><h1 class="h2t">api.example.com unreachable</h1>%s%s</div>%s%s'
            '<div class="pad"><div class="hl">Activity</div>%s</div></div>') % (prose, pills, trk, expected_actual("200 OK", "under 1.5s, valid TLS", "Timeout", "no response in 10s, 3 failures in a row"), tl)


def incident_detail(d):
    return incident_body(d) + '<div class="cmp"><span class="in">Leave a note&hellip;</span><a href="#" class="btn p">Acknowledge</a></div>'


def _opt(kind, t, sub="", check=False, hot=False):
    return ('<div class="opt"%s>%s<div class="col"><b style="font-weight:500">%s</b>%s</div>%s</div>') % (
        ' style="background:var(--seg);margin:0 -16px;padding:0 16px"' if hot else "", si(kind), t,
        ('<span class="xs fa">%s</span>' % sub) if sub else "", ('<span class="ck">%s</span>' % ic("check", 20, 2.2)) if check else "")


def incident_ack(d):
    return incident_body(d) + sheet("Status", '<div class="col" style="margin-top:-6px">%s%s%s%s</div><p class="xs fa" style="margin:0">Acknowledging also makes you the responder. Others get a push.</p>' % (
        _opt("dn", "Open", "", True), _opt("dg", "Acknowledged", "You are on it", False, True), _opt("cv", "Covered", "Someone is covering for you"), _opt("up", "Resolved", "Closes the timeline")))


def incident_handoff(d):
    av = lambda t, bg: '<span style="width:28px;height:28px;border-radius:50%%;background:%s;color:#fff;font-size:11px;font-weight:600;display:flex;align-items:center;justify-content:center;flex:none">%s</span>' % (bg, t)
    pr = lambda a, n, s, ck=False: '<div class="opt">%s<div class="col"><b style="font-weight:500">%s</b><span class="xs fa">%s</span></div>%s</div>' % (a, n, s, ('<span class="ck">%s</span>' % ic("check", 20, 2.2)) if ck else "")
    return incident_body(d) + sheet("Responder",
        '<div class="fld"><span class="fa">%s</span><span class="v fa">Assign to&hellip;</span></div><div class="col" style="margin-top:-6px">%s%s%s%s</div>' % (
            ic("search", 18), pr('<span class="avt"></span>', "Take it", "Ama Mensah (you)"), pr(av("KB", "#3b82f6"), "Kofi Boateng", "On call this week"),
            pr(av("EO", "#8b5cf6"), "Esi Owusu", "Off shift &middot; cover for her"), pr(ic("user", 24), "Unassigned", "", True)))
