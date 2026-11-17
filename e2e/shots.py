"""README screenshots against a running vaillant-ui backed by cmd/fakeebusd.

python3 e2e/shots.py http://127.0.0.1:8098 docs/screenshots
"""
import sys

from playwright.sync_api import sync_playwright

base, out = sys.argv[1], sys.argv[2]
D, M = {"width": 1280, "height": 820}, {"width": 390, "height": 844}
shots = [
    ("uebersicht-desktop", "uebersicht", D, False),
    ("uebersicht-mobil", "uebersicht", M, False),
    ("zeitprogramm-mobil", "zeit", M, False),
    ("menue-mobil", "menu", M, False),
    ("heizkreis-desktop", "g/hc1", D, True),
]
with sync_playwright() as p:
    # Full Chromium honours --lang, so time inputs render as 24 h.
    b = p.chromium.launch(channel="chromium", args=["--lang=de-DE"])
    for name, route, vp, unlock in shots:
        ctx = b.new_context(viewport=vp, color_scheme="light", device_scale_factor=2, locale="de-DE", timezone_id="Europe/Berlin")
        if unlock:
            ctx.add_init_script("sessionStorage.setItem('fh','1')")
        pg = ctx.new_page()
        pg.goto(f"{base}/#/{route}")
        pg.wait_for_timeout(3500)
        pg.screenshot(path=f"{out}/{name}.png")
        ctx.close()
    b.close()
