"""Shared mobile building blocks: header, tray nav, sheet and the Orion signature components
(lifecycle tracker, expected vs actual, pale-header box, uptime bars, halo timeline)."""
from common import ic

MARK = ('<svg width="%d" height="%d" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" '
        'stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="9" stroke-dasharray="2 4"/></svg>')


def logo(size=26):
    return '<span class="mk" style="width:%dpx;height:%dpx">%s</span>' % (size, size, MARK % (size * 5 // 9, size * 5 // 9))


def hd(core="Production", dot="dg", issues="2 issues", back=None, right=("search",), pill=True):
    if back:
        left = '<a href="#" class="core" aria-label="Back">%s<span>%s</span></a>' % (ic("back", 22), back)
    else:
        left = '<a href="#" class="core" aria-label="Switch Core">%s<span>%s</span><span class="cv">%s</span></a>' % (logo(26), core, ic("down", 16))
    r = "".join('<a href="#" class="ibtn sq" aria-label="%s">%s</a>' % (n.title(), ic(n, 18)) for n in right)
    if pill and issues and not back:
        r += '<span class="hp"><i class="s %s"></i>%s</span>' % (dot, issues)
    return '<div class="hd">%s<div class="row" style="gap:0">%s</div></div>' % (left, r)


def tabs(active, badge=3):
    items = [("Notifications", "bell"), ("Incidents", "alert"), ("Servers", "server"), ("Monitors", "pulse"), ("More", "more")]
    out = ""
    for n, i in items:
        b = '<span class="bg2">%d</span>' % badge if (n == "Notifications" and badge) else ""
        out += '<a href="#"%s>%s%s<span>%s</span></a>' % (' class="on"' if n == active else "", ic(i, 20), b, n)
    return '<nav class="nav" aria-label="Main"><div class="tray">%s</div></nav>' % out


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
