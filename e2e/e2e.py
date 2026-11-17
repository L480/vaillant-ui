"""Browser E2E against a running vaillant-ui (writable runs must use cmd/fakeebusd, never a real bus).

python3 e2e/e2e.py http://127.0.0.1:8098 [--readonly URL] [--shots DIR]
"""
import json
import sys
import urllib.request

from playwright.sync_api import expect, sync_playwright

args = sys.argv[1:]
BASE = args[0]
RO = args[args.index("--readonly") + 1] if "--readonly" in args else None
SHOTS = args[args.index("--shots") + 1] if "--shots" in args else None
failures = []


def api(base, ids):
    with urllib.request.urlopen(f"{base}/api/values?fresh=1&ids={ids}") as r:
        return {k: v.get("value") for k, v in json.load(r)["values"].items()}


def check(name, cond, detail=""):
    print(("PASS " if cond else "FAIL ") + name + (f" — {detail}" if detail and not cond else ""))
    if not cond:
        failures.append(name)


def tid(page, t):
    return page.locator(f'[data-testid="{t}"]')


def save_if_needed(page, item):
    page.wait_for_timeout(400)
    btn = tid(page, f"save-{item}").first
    try:
        btn.click(timeout=1500)
    except Exception:
        pass  # instant-save control


def confirm_if_shown(page):
    ok = tid(page, "confirm-ok")
    try:
        ok.wait_for(state="visible", timeout=1500)
        ok.click()
    except Exception:
        pass


def wait_toast(page, text="Gespeichert"):
    expect(tid(page, "toast").filter(has_text=text).first).to_be_visible(timeout=10000)


def run(p, vp, label):
    b = p.chromium.launch()
    ctx = b.new_context(viewport=vp, locale="de-DE", timezone_id="Europe/Berlin")
    page = ctx.new_page()
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.on("console", lambda m: m.type == "error" and errors.append(m.text))
    L = lambda s: f"[{label}] {s}"

    page.goto(f"{BASE}/#/uebersicht")
    expect(page.get_by_text("Außentemperatur").first).to_be_visible(timeout=15000)
    expect(tid(page, "val-700.z1DayTemp").first).to_be_visible(timeout=15000)
    check(L("overview renders"), True)
    if SHOTS:
        page.screenshot(path=f"{SHOTS}/e2e-{label}-overview.png", full_page=True)

    tid(page, "opt-700.OpMode-night").first.click()
    save_if_needed(page, "700.OpMode")
    wait_toast(page)
    check(L("OpMode -> night"), api(BASE, "700.OpMode")["700.OpMode"] == "night")
    tid(page, "opt-700.OpMode-auto").first.click()
    save_if_needed(page, "700.OpMode")
    wait_toast(page)
    check(L("OpMode -> auto"), api(BASE, "700.OpMode")["700.OpMode"] == "auto")

    before = api(BASE, "700.z1DayTemp")["700.z1DayTemp"]
    tid(page, "inc-700.z1DayTemp").first.click()
    save_if_needed(page, "700.z1DayTemp")
    wait_toast(page)
    after = api(BASE, "700.z1DayTemp")["700.z1DayTemp"]
    check(L("DayTemp +0.5"), after == before + 0.5, f"{before} -> {after}")
    tid(page, "dec-700.z1DayTemp").first.click()
    save_if_needed(page, "700.z1DayTemp")
    wait_toast(page)

    tid(page, "sf-party").first.click()
    confirm_if_shown(page)
    page.wait_for_timeout(1500)
    check(L("Schnellwahl party"), api(BASE, "700.SFMode")["700.SFMode"] == "party")
    tid(page, "sf-auto").first.click()
    confirm_if_shown(page)
    page.wait_for_timeout(1500)
    check(L("Schnellwahl beenden"), api(BASE, "700.SFMode")["700.SFMode"] == "auto")

    page.goto(f"{BASE}/#/zeit")
    timer = tid(page, "timer-700.z1Timer")
    expect(timer).to_be_visible(timeout=15000)
    tid(page, "add-700.z1Timer-Wednesday").click()
    page.wait_for_timeout(300)
    if SHOTS:
        page.screenshot(path=f"{SHOTS}/e2e-{label}-timer.png", full_page=True)
    wed = api(BASE, "700.z1Timer")["700.z1Timer"]["Wednesday"]
    tid(page, "timer-save-700.z1Timer").click()
    confirm_if_shown(page)
    page.wait_for_timeout(1500)
    wed2 = api(BASE, "700.z1Timer")["700.z1Timer"]["Wednesday"]
    check(L("timer add window saved"), len(wed2) == len(wed) + 1, f"{wed} -> {wed2}")

    page.goto(f"{BASE}/#/urlaub")
    expect(tid(page, "row-700.HolidayStartPeriod")).to_be_visible(timeout=15000)
    day = "2026-12-24" if label == "mobile" else "2026-12-27"
    tid(page, "row-700.HolidayStartPeriod").locator("input[type=date]").fill(day)
    save_if_needed(page, "700.HolidayStartPeriod")
    wait_toast(page)
    check(L("holiday start date"), api(BASE, "700.HolidayStartPeriod")["700.HolidayStartPeriod"] == day)
    if SHOTS:
        page.screenshot(path=f"{SHOTS}/e2e-{label}-urlaub.png", full_page=True)

    page.goto(f"{BASE}/#/g/hc1")
    page.wait_for_timeout(1000)
    check(L("installer group hidden while locked"), tid(page, "row-700.Hc1HeatCurve").count() == 0)
    page.get_by_role("button", name="Entsperren").click()
    tid(page, "unlock-input").fill("17")
    tid(page, "unlock-input").press("Enter")
    page.goto(f"{BASE}/#/g/hc1")
    expect(tid(page, "row-700.Hc1HeatCurve")).to_be_visible(timeout=15000)
    hc = api(BASE, "700.Hc1HeatCurve")["700.Hc1HeatCurve"]
    tid(page, "inc-700.Hc1HeatCurve").click()
    save_if_needed(page, "700.Hc1HeatCurve")
    confirm_if_shown(page)
    wait_toast(page)
    hc2 = api(BASE, "700.Hc1HeatCurve")["700.Hc1HeatCurve"]
    check(L("heat curve +0.05 after installer confirm"), abs(hc2 - hc - 0.05) < 1e-6, f"{hc} -> {hc2}")
    if SHOTS:
        page.screenshot(path=f"{SHOTS}/e2e-{label}-hc1.png", full_page=True)

    page.goto(f"{BASE}/#/g/thermeset")
    expect(tid(page, "row-bai.PartloadHcKW")).to_be_visible(timeout=20000)
    check(L("non-writable d-code has no stepper"), tid(page, "inc-bai.PartloadHcKW").count() == 0)

    page.goto(f"{BASE}/#/info")
    with page.expect_download(timeout=120000) as dl:
        tid(page, "dump").click()
    path = dl.value.path()
    dump = json.load(open(path))
    check(L("config dump download"), dl.value.suggested_filename.startswith("vaillant-config-") and len(dump["raw"]) > 300 and "700.z1DayTemp" in dump["items"], dl.value.suggested_filename)
    for r in ["fehler", "info", "g/status", "g/verbrauch", "g/thermelive", "g/thermestat", "g/system"]:
        page.goto(f"{BASE}/#/{r}")
        page.wait_for_timeout(2500)
        check(L(f"view {r} has no error box"), page.locator(".errbox").count() == 0)
    check(L("no JS errors"), not errors, "; ".join(errors[:5]))
    b.close()


def run_readonly(p):
    b = p.chromium.launch()
    page = b.new_page(locale="de-DE")
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.goto(f"{RO}/#/uebersicht")
    expect(page.get_by_text("Änderungen sind deaktiviert").first).to_be_visible(timeout=20000)
    page.wait_for_timeout(3000)
    check("[ro] no steppers", tid(page, "inc-700.z1DayTemp").count() == 0 or tid(page, "inc-700.z1DayTemp").first.is_disabled())
    check("[ro] no JS errors", not errors, "; ".join(errors))
    if SHOTS:
        page.screenshot(path=f"{SHOTS}/e2e-readonly.png", full_page=True)
    b.close()


with sync_playwright() as p:
    run(p, {"width": 390, "height": 844}, "mobile")
    run(p, {"width": 1280, "height": 900}, "desktop")
    if RO:
        run_readonly(p)

print(f"\n{len(failures)} failure(s)" + (": " + ", ".join(failures) if failures else ""))
sys.exit(1 if failures else 0)
