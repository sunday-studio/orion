"""Mobile screens, part A: connect, notifications, incidents."""
from common import ic
from parts import hd, tabs, sc, sheet, title, tracker, expected_actual, box, bars, stats, tl2, logo












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






def incident_ack(d):
    return incident_body(d) + sheet("Acknowledge INC-0231",
        '<p class="mu sm" style="margin:-6px 0 0">Tells everyone you are on it. The incident stays open until it recovers or you resolve it.</p>'
        '<div><label class="lbl">Note (optional)</label><div class="fld f" style="align-items:flex-start;min-height:80px;padding-top:12px"><span class="v mu">Looking at edge-01 now</span></div></div>'
        '<div class="row sp"><span class="med">Take responder role</span><span class="tg on"></span></div>'
        '<div class="row g8"><a href="#" class="btn f">Cancel</a><a href="#" class="btn p f">Acknowledge</a></div>')


def incident_resolve(d):
    return incident_body(d) + sheet("Resolve INC-0231",
        '<p class="mu sm" style="margin:-6px 0 0">Closes the incident and keeps the timeline. If api-https fails again, Orion opens a new one.</p>'
        '<div><label class="lbl">What fixed it?</label><div class="fld f" style="align-items:flex-start;min-height:96px;padding-top:12px"><span class="v mu">Restarted nginx on edge-01</span></div></div>'
        '<div class="row g8"><a href="#" class="btn f">Cancel</a><a href="#" class="btn p f">Resolve incident</a></div>')


def incident_handoff(d):
    opt = lambda on, t, s: ('<div class="row g12" style="padding:12px;border:1px solid %s;border-radius:8px;background:var(--sf)"><span class="s %s" style="width:16px;height:16px"></span><div class="col g4" style="flex:1"><b class="med">%s</b><span class="xs mu">%s</span></div></div>') % ("var(--ac)" if on else "var(--bd)", "up" if on else "un", t, s)
    return incident_body(d) + sheet("Who is on this?",
        '<div class="col g8">%s%s%s</div><div class="row g8"><a href="#" class="btn f">Cancel</a><a href="#" class="btn p f">Confirm</a></div>' % (
            opt(True, "I&rsquo;ll take it", "You become the responder. Others see your name."),
            opt(False, "Cover for Ama", "Ama is off shift. You act on her behalf until she is back."),
            opt(False, "Hand to someone else", "Pick a teammate. They get a push.")))


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


def feed(d, offline=False):
    ban = ('<div class="ban un">%s<span>Can&rsquo;t reach Production. Showing data from 4 min ago.</span><a href="#" class="med" style="margin-left:auto;color:var(--tx)">Retry</a></div>' % ic("off", 18)) if offline else ""
    dim = ' style="opacity:.55"' if offline else ""
    need = ('<div class="fieldbox"%s><div class="fh"><b>Needs you now</b><span>Open for 30m 12s</span></div>'
            '<div class="card cp col g12"><div class="row sp"><div class="row g8"><span class="s dn"></span><b class="med" style="font-size:15.5px">api.example.com is down</b></div><span class="b cr">Critical</span></div>'
            '%s<div class="xs mu" style="line-height:1.5">Latest: check failed, timeout after 10s.<br>Sent to this iPhone and ops-team. oncall webhook failed.</div>'
            '<div class="row g8" style="justify-content:flex-end"><a href="#" class="btn sm">Details</a>%s</div></div></div>') % (
        dim, tracker([("Detected", "14:02", "done"), ("Notified", "14:02", "done"), ("Acknowledge", "waiting", "now"), ("Resolved", "-", "todo")]),
        '' if offline else '<a href="#" class="btn sm p">Acknowledge</a>')
    st = stats([("Monitors up", "21", "of 24"), ("Servers", "5", "of 6"), ("Uptime, 30d", "99.94%", "")])
    row = lambda sh, t, sub, when, bold=False: ('<a href="#" class="br"><span class="s %s"></span><div class="t"><b style="font-weight:%d">%s</b><small>%s</small></div><span class="xs fa">%s</span>%s</a>' % (sh, 500, t, sub, when, '<span class="unr"></span>' if bold else ''))
    earlier = box("Earlier", "4 notifications",
                  row("dg", "db-02 disk above 90%", "INC-0230 &middot; acknowledged by you", "16h", True) +
                  row("up", "edge-02 is reporting again", "Stopped reporting for 8 min", "Sun", True) +
                  row("up", "nightly-backup heartbeat resumed", "INC-0227 resolved after 1h 12m", "Sat"))
    return (hd() + ban + '<h1 class="ttl" style="padding-bottom:12px">How is it going, Ama?</h1><div class="sub">Thursday, Oct 16 &middot; 14:32 UTC</div>' +
            sc(need + st + earlier) + tabs("Notifications"))


def feed_empty(d):
    st = stats([("Monitors up", "24", "of 24"), ("Servers", "6", "of 6"), ("Uptime, 30d", "99.98%", "")])
    return (hd(dot="up", issues="All good") + '<h1 class="ttl" style="padding-bottom:12px">All quiet, Ama.</h1><div class="sub">Thursday, Oct 16 &middot; 14:32 UTC</div>' +
            sc('<div class="fieldbox"><div class="fh"><b>Needs you now</b><span>Nothing</span></div><div class="card cp ctr g8" style="padding:26px 20px"><span class="td up" style="width:36px;height:36px">%s</span>'
               '<b style="font-size:16px;font-weight:500">Nothing needs you</b><span class="sm mu" style="max-width:260px">When an alert fires it lands here, on your lock screen and in the Console inbox.</span>'
               '<a href="#" class="btn sm" style="margin-top:6px">Send a test notification</a></div></div>%s' % (ic("check", 20, 2.4), st)) + tabs("Notifications", 0))


def incidents_list(d):
    def r(sh, t, idn, mon, badges, when):
        return ('<a href="#" class="br" style="align-items:flex-start;padding:14px"><span class="s %s" style="margin-top:6px"></span><div class="t"><b style="white-space:normal">%s</b>'
                '<small style="margin-top:2px">%s &middot; %s</small><div class="row g8" style="margin-top:8px">%s</div></div><span class="xs fa">%s</span></a>') % (sh, t, idn, mon, badges, when)
    rows = (r("dn", "api.example.com unreachable", "INC-0231", "api-https", '<span class="b cr">Critical</span><span class="b dn">Open</span>', "30m") +
            r("dg", "db-02 disk above 90%", "INC-0230", "disk-usage", '<span class="b dn">High</span><span class="b dg">Acknowledged</span>', "16h") +
            r("dg", "Load balancer degraded", "INC-0229", "lb-https", '<span class="b dg">Medium</span><span class="b cv">Covered</span>', "2d"))
    return (hd(right=("search", "filter")) + title("Incidents", "Everything Orion has opened, newest first.") +
            '<div class="seg"><a href="#" class="on">Active 3</a><a href="#">Needs review 1</a><a href="#">Resolved 21</a></div>' +
            sc(box("Active", "Severity High and Critical shown first", rows)) + tabs("Incidents"))


def incident_body(d):
    prose = ('<p class="prose">The health check on <b>api-https</b> has timed out since 14:02. ops-team was notified; the on-call webhook returned 502, so nobody was paged.</p>')
    trk = '<div class="fieldbox"><div class="card cp" style="padding:14px 8px">%s</div></div>' % tracker([("Detected", "14:02", "done"), ("Notified", "14:02", "done"), ("Acknowledged", "waiting", "now"), ("Resolved", "-", "todo")])
    tl = tl2([("dg", "Delivery to <b>oncall webhook</b> failed after 3 attempts &middot; <u>Retry</u>", "14:04"),
              ("up", "Notified <b>this iPhone</b> and <b>ops-team</b> (email)", "14:02"),
              ("dn", "Incident opened: <b>api-https</b> failed 3 checks in a row", "14:02")])
    return ('<div class="hd" style="padding-bottom:4px"><a href="#" class="core" aria-label="Back">%s<span class="crumb">Incidents <span>/</span> <span style="color:var(--tx)">INC-0231</span></span></a><a href="#" class="ibtn" aria-label="More">%s</a></div>'
            '<div class="sc" style="gap:12px"><div class="pad col g8"><h1 class="h2t">api.example.com unreachable</h1>'
            '<div class="row g8"><span class="b dn">Open</span><span class="b cr">Critical</span><span class="xs fa">Open for 30m 12s</span></div></div>'
            '<div class="pad">%s</div>%s%s'
            '<div class="ut" style="margin:0 16px"><a href="#" class="on">Timeline</a><a href="#">Details</a><a href="#">Notifications 3</a><a href="#">Logs 14</a></div>'
            '<div class="pad">%s</div></div>') % (ic("back", 22), ic("more", 22), prose, trk, expected_actual("200 OK", "under 1.5s, valid TLS", "Timeout", "no response in 10s, 3 failures in a row"), tl)


def incident_detail(d):
    return incident_body(d) + '<div class="bar"><a href="#" class="btn">Cover</a><a href="#" class="btn f">Resolve</a><a href="#" class="btn p f">Acknowledge</a></div>'
