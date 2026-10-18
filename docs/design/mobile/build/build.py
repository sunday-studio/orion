"""Generate the Orion mobile artboards and merge them into canvas.json.

Usage: python3 build.py <canvas.json in> <out dir>
Writes <out>/project/*.dc.html, <out>/project/canvas.json and <out>/preview/*.html.
"""
import json
import os
import re
import sys

from common import page
import screens_a as A
import screens_b as B
import pages

SECTIONS = [
    ("Connect", [("welcome", "Welcome", A.welcome), ("scan", "Scan QR code", A.scan), ("manual", "Enter server address", A.manual),
                 ("manual-insecure", "Insecure address", lambda d: A.manual(d, True)), ("paired", "Paired, permissions", A.paired)]),
    ("Notifications", [("feed", "Notifications feed", A.feed), ("feed-empty", "All quiet", A.feed_empty), ("lockscreen", "Push on lock screen", A.lockscreen), ("widgets", "Widgets and Live Activity", A.widgets)]),
    ("Incidents", [("incidents-list", "Incidents", A.incidents_list), ("incident-review", "Needs review", A.incident_review), ("incident-detail", "Incident detail", A.incident_detail),
                   ("incident-ack", "Acknowledge sheet", A.incident_ack), ("incident-resolve", "Resolve sheet", A.incident_resolve),
                   ("incident-handoff", "Take or cover sheet", A.incident_handoff), ("incident-public", "Status page update sheet", A.incident_public)]),
    ("Servers and monitors", [("servers-list", "Servers", B.servers_list), ("server-detail", "Server detail", B.server_detail), ("monitors-list", "Monitors", B.monitors_list),
                              ("monitor-detail", "Monitor detail", B.monitor_detail), ("monitor-test", "Test run sheet", B.monitor_test)]),
    ("Logs and deliveries", [("logs", "Logs", B.logs), ("logs-live", "Live tail", B.logs_live), ("log-detail", "Log detail", B.log_detail),
                             ("deliveries", "Alert deliveries", B.deliveries), ("delivery-detail", "Delivery detail", B.delivery_detail)]),
    ("More and settings", [("more", "More", B.more), ("search", "Search", B.search), ("settings", "Settings", B.settings),
                           ("settings-notifications", "Notification settings", B.settings_notifications), ("core-switcher", "Core switcher", B.core_switcher)]),
    ("States", [("offline", "Core unreachable", lambda d: A.feed(d, True)), ("session-expired", "Session expired", B.session_expired), ("revoked", "Device removed", B.revoked)]),
]


def main(canvas_in, out):
    proj = os.path.join(out, "project")
    prev = os.path.join(out, "preview")
    os.makedirs(proj, exist_ok=True)
    os.makedirs(prev, exist_ok=True)
    c = json.load(open(canvas_in))
    boards, order, notes = c["boards"], c["order"], c["notes"]
    for pid, name in (("mia", "Mobile IA"), ("mfl", "Mobile flows"), ("mui", "Mobile screens")):
        if not any(p["id"] == pid for p in c["pages"]):
            c["pages"].append({"id": pid, "name": name})

    def emit(fname, html, x, y, w, h, title, pg):
        open(os.path.join(proj, fname), "w").write(html)
        pv = html.replace('<script src="./support.js"></script>', "").replace("<x-dc><helmet>", "").replace("</helmet>", "").replace("</x-dc>", "")
        pv = re.sub(r'<script type="text/x-dc".*?</script>', "", pv, flags=re.S)
        open(os.path.join(prev, fname.replace(".dc.html", ".html")), "w").write(pv)
        boards[fname] = {"x": x, "y": y, "w": w, "h": h, "title": title, "page": pg}
        if fname not in order:
            order.append(fname)

    emit("mobile-ia.dc.html", pages.ia(), 0, 120, 1280, 2340, "Mobile IA", "mia")
    fhtml, fh = pages.flows()
    emit("mobile-flows.dc.html", fhtml, 0, 120, 1280, fh, "Mobile flows", "mfl")

    y = 120
    for sname, items in SECTIONS:
        rowW = len(items) * 390 + (len(items) - 1) * 80
        notes["mob-" + sname.lower().replace(" ", "-")] = {"x": 0, "y": y, "text": sname, "kind": "title1", "maxW": max(rowW, 900), "page": "mui"}
        ly = y + 260
        dy = ly + 844 + 120
        for i, (key, title, fn) in enumerate(items):
            x = i * 470
            emit("mobile-%s.dc.html" % key, page(title, fn(False), False), x, ly, 390, 844, title, "mui")
            emit("mobile-%s-dark.dc.html" % key, page(title + ", dark", fn(True), True), x, dy, 390, 844, title + ", dark", "mui")
        y = dy + 844 + 200

    json.dump(c, open(os.path.join(proj, "canvas.json"), "w"), indent=1, ensure_ascii=False)
    print("boards added:", len([b for b in boards if b.startswith("mobile-")]))


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
