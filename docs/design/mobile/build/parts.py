"""Shared mobile building blocks: header, tray nav, sheet and the Orion signature components
(lifecycle tracker, expected vs actual, pale-header box, uptime bars, halo timeline)."""
from common import ic

MARK = ('<svg width="%d" height="%d" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" '
        'stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="9" stroke-dasharray="2 4"/></svg>')


def logo(size=26):
    return '<span class="mk" style="width:%dpx;height:%dpx">%s</span>' % (size, size, MARK % (size * 5 // 9, size * 5 // 9))


SI = {
 "dn": '<circle cx="10" cy="10" r="8" style="stroke:var(--dnx)" fill="none" stroke-width="1.6"/><path d="M10 5.8v4.8" style="stroke:var(--dnx)" stroke-width="1.8" stroke-linecap="round"/><circle cx="10" cy="13.7" r="1" style="fill:var(--dnx)"/>',
 "dg": '<circle cx="10" cy="10" r="8" style="stroke:var(--ddg)" fill="none" stroke-width="1.6"/><path d="M10 4.5a5.5 5.5 0 0 1 0 11z" style="fill:var(--ddg)"/>',
 "cv": '<circle cx="10" cy="10" r="8" style="stroke:var(--cvt)" fill="none" stroke-width="1.6"/><path d="M10 4.5a5.5 5.5 0 0 1 5.5 5.5H10z" style="fill:var(--cvt)"/>',
 "up": '<circle cx="10" cy="10" r="9" style="fill:var(--dup)"/><path d="M6 10.4l2.8 2.8L14 7.6" stroke="#fff" fill="none" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>',
 "un": '<circle cx="10" cy="10" r="8" style="stroke:#a1a1aa" fill="none" stroke-width="1.6" stroke-dasharray="2.6 2.6"/>',
 "mt": '<circle cx="10" cy="10" r="8" style="stroke:#8b5cf6" fill="none" stroke-width="1.6"/><path d="M8 7.5v5M12 7.5v5" style="stroke:#8b5cf6" stroke-width="1.6" stroke-linecap="round"/>',
}


def si(kind, size=20):
    return '<svg class="si" width="%d" height="%d" viewBox="0 0 20 20" aria-hidden="true">%s</svg>' % (size, size, SI[kind])


def hd(core="Production", dot="dg", back=None, right=("search", "sliders")):
    if back:
        left = '<a href="#" class="core" aria-label="Back">%s<span>%s</span></a>' % (ic("back", 22), back)
    else:
        left = '<a href="#" class="core" aria-label="Switch Core">%s<span>%s</span><i class="s %s"></i><span class="cv">%s</span></a>' % (logo(24), core, dot, ic("down", 16))
    r = "".join('<a href="#" class="ibtn" aria-label="%s">%s</a>' % (n.title(), ic(n, 21)) for n in right)
    return '<div class="hd" style="padding-bottom:2px">%s<div class="row" style="gap:0">%s</div></div>' % (left, r)


def dhd(mid, right=("bell", "more")):
    r = "".join('<a href="#" class="ibtn" aria-label="%s">%s</a>' % (n.title(), ic(n, 21)) for n in right)
    return ('<div class="dh"><a href="#" class="core" aria-label="Back">%s</a><span class="mono" style="font-size:13px">%s</span><div class="r">%s</div></div>') % (ic("back", 24), mid, r)


def tabs(active, badge=3):
    items = [("Notifications", "bell"), ("Incidents", "alert"), ("Servers", "server"), ("Monitors", "pulse"), ("More", "more")]
    out = ""
    for n, i in items:
        b = '<span class="bg2">%d</span>' % badge if (n == "Notifications" and badge) else ""
        out += '<a href="#"%s>%s%s<span>%s</span></a>' % (' class="on"' if n == active else "", ic(i, 24), b, n)
    return '<nav class="bar2" aria-label="Main">%s</nav>' % out


def rw(kind, t, sub, when, unread=False, extra=""):
    return ('<a href="#" class="rw"%s>%s<div class="t"><b class="%s">%s</b><small>%s</small></div><div class="r"><span>%s</span>%s</div></a>') % (
        extra, si(kind), "u" if unread else "", t, sub, when, '<i class="unr"></i>' if unread else "")


def gh(kind, t, n):
    return '<div class="gh">%s%s<span>%s</span></div>' % (si(kind, 16) if kind else "", t, n)


def pill(icon_html, t, q=False):
    return '<span class="pl%s">%s%s</span>' % (" q" if q else "", icon_html, t)


def sc(inner, gap=12):
    return '<div class="sc" style="gap:%dpx">%s</div>' % (gap, inner)


def sheet(title, inner):
    return '<div class="scrim"></div><div class="sheet"><div class="grab"></div><div class="sh1">%s</div>%s</div>' % (title, inner)


def title(t, sub=None):
    return '<h1 class="ttl">%s</h1>%s' % (t, ('<div class="sub">%s</div>' % sub) if sub else "")


def tracker(steps):
    """steps: [(label, small, state)] with state done | now | nowa | todo."""
    pos = 0
    for i, (_, _, st) in enumerate(steps):
        if st in ("done", "now", "nowa"):
            pos = i
    cells = "".join('<div class="%s"><i>%s</i><b>%s</b><small>%s</small></div>' % (st, ic("check", 12, 3) if st == "done" else "", l, sm) for l, sm, st in steps)
    return '<div class="trk"><span class="ln"></span><span class="fl" style="width:%.1f%%"></span>%s</div>' % (75.0 * pos / (len(steps) - 1), cells)


def expected_actual(ev, es, av, asub, tone="dnt"):
    return ('<div class="ea"><div class="x"><span>Expected</span><b>%s</b><small>%s</small></div>'
            '<div class="y"><span>Actual, 14:32</span><b style="color:var(--%s)">%s</b><small>%s</small></div></div>') % (ev, es, tone, av, asub)


def box(t, meta, rows):
    return '<div class="box"><div class="bh"><b>%s</b><span>%s</span></div>%s</div>' % (t, meta, rows)


def bars(n=30, bad=(), warn=()):
    return '<div class="ub">%s</div>' % "".join('<i class="%s"></i>' % ("d" if i in bad else "w" if i in warn else "") for i in range(n))


def stats(items):
    return '<div class="stats">%s</div>' % "".join(
        '<div class="card sc3"><span>%s</span><b>%s%s</b></div>' % (l, v, (' <small>%s</small>' % o) if o else "") for l, v, o in items)


def tl2(items):
    return '<div class="tl2">%s</div>' % "".join(
        '<div><span class="td %s"><i></i></span><p>%s</p><time>%s</time></div>' % (tone, txt, t) for tone, txt, t in items)
