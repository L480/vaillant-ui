import { icon } from './icons.js';

const UNLOCK_CODE = '17';
const DAYS = [['Monday', 'Mo', 'Montag'], ['Tuesday', 'Di', 'Dienstag'], ['Wednesday', 'Mi', 'Mittwoch'], ['Thursday', 'Do', 'Donnerstag'], ['Friday', 'Fr', 'Freitag'], ['Saturday', 'Sa', 'Samstag'], ['Sunday', 'So', 'Sonntag']];
const S = { meta: null, cat: null, items: {}, unlocked: false };
try { S.unlocked = sessionStorage.getItem('fh') === '1'; } catch {}

const $ = (s) => document.querySelector(s);
const eq = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const clone = (o) => JSON.parse(JSON.stringify(o));

function h(tag, props, ...kids) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') e.className = v;
    else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
    else if (v === true) e.setAttribute(k, '');
    else e.setAttribute(k, v);
  }
  add(e, kids);
  return e;
}
function add(e, kids) {
  for (const k of kids.flat(Infinity)) {
    if (k == null || k === false) continue;
    e.append(k instanceof Node ? k : document.createTextNode(String(k)));
  }
  return e;
}
const clear = (e) => { e.replaceChildren(); return e; };

async function api(path, opts = {}) {
  const c = new AbortController();
  const t = setTimeout(() => c.abort(), 20000);
  try {
    const r = await fetch(path, { ...opts, signal: c.signal, headers: { 'Content-Type': 'application/json' } });
    let j = null;
    try { j = await r.json(); } catch {}
    if (!r.ok) throw new Error((j && j.error) || `HTTP ${r.status}`);
    return j;
  } catch (e) {
    if (e.name === 'AbortError') throw new Error('Zeitüberschreitung – keine Antwort vom Server');
    if (e instanceof TypeError) throw new Error('Server nicht erreichbar');
    throw e;
  } finally { clearTimeout(t); }
}
const readValues = async (ids, fresh) => (await api('/api/values?ids=' + ids.map(encodeURIComponent).join(',') + (fresh ? '&fresh=1' : ''))).values || {};

function toast(msg, kind = '') {
  const t = h('div', { class: 'toast ' + kind, 'data-testid': 'toast', role: 'status' }, msg);
  $('#toasts').append(t);
  setTimeout(() => t.remove(), kind === 'err' ? 6000 : 2500);
}

function modal(build) {
  return new Promise((resolve) => {
    const bg = h('div', { class: 'modalbg' });
    const close = (v) => { bg.remove(); document.removeEventListener('keydown', onKey); resolve(v); };
    const onKey = (e) => { if (e.key === 'Escape') close(null); };
    const box = h('div', { class: 'modal', role: 'dialog', 'aria-modal': 'true' });
    bg.append(box);
    bg.addEventListener('click', (e) => { if (e.target === bg) close(null); });
    document.addEventListener('keydown', onKey);
    build(box, close);
    document.body.append(bg);
    const f = box.querySelector('input,button.primary,button.danger');
    if (f) f.focus();
  });
}
const confirmBox = (title, text, okLabel = 'Fortfahren', danger = false) =>
  modal((box, close) => add(box, [
    h('h2', {}, icon('warning'), title), h('p', {}, text),
    h('div', { class: 'btns' },
      h('button', { class: 'btn', onclick: () => close(false) }, 'Abbrechen'),
      h('button', { class: 'btn ' + (danger ? 'danger' : 'primary'), 'data-testid': 'confirm-ok', onclick: () => close(true) }, okLabel)),
  ])).then(Boolean);

const itemOf = (id) => S.items[id];
const num = (v) => typeof v === 'number' && isFinite(v);
const decOf = (it) => it.decimals != null ? it.decimals : (it.step && String(it.step).includes('.') ? String(it.step).split('.')[1].length : 0);
const fmtNum = (v, d) => v.toLocaleString('de-DE', { minimumFractionDigits: d, maximumFractionDigits: d });
const fmtDate = (s) => (typeof s === 'string' && /^\d{4}-\d\d-\d\d$/.test(s)) ? s.split('-').reverse().join('.') : s;
const winText = (w) => w.map(([a, b]) => `${a}–${b}`).join(', ') || 'aus';

function fmtValue(it, res) {
  if (!res) return '…';
  if (res.error) return res.error;
  const v = res.value;
  if (v == null) return '—';
  if (it.format === 'statecode') return res.text || String(v);
  if (it.format === 'flame') return res.text || '—';
  if (it.format === 'counter' && num(v)) return fmtNum(v, 0) + (it.unit ? ' ' + it.unit : '');
  if (it.format === 'pressure' && num(v)) return fmtNum(v, 2) + ' bar';
  switch (it.kind) {
    case 'number': return num(v) ? fmtNum(v, decOf(it)) + (it.unit ? ' ' + it.unit : '') : String(v);
    case 'enum': { const o = (it.options || []).find((o) => o.value === v); return o ? o.label : String(v); }
    case 'bool': return v ? 'An' : 'Aus';
    case 'date': return fmtDate(v);
    case 'timer': return DAYS.map(([k, s]) => `${s} ${winText(v[k] || [])}`).join(' · ');
    default: return String(v) + (it.unit && num(v) ? ' ' + it.unit : '');
  }
}

function skeleton(w) { return h('span', { class: 'skel', style: w ? `width:${w}px` : null }); }

function makeRow(item, o = {}) {
  if (item.kind === 'timer') return makeTimer(item, o);
  const ro = !item.writable;
  const disabled = S.meta.readonly;
  let res = null, cur, draft, dirty = false, saving = false;
  const ctl = h('div', { class: 'ctl' });
  const actions = h('div', { class: 'actions', hidden: true });
  const help = item.help ? h('div', { class: 'help', hidden: true }, item.help) : null;
  const infoBtn = item.help ? h('button', { class: 'infobtn', type: 'button', 'aria-label': 'Hilfe', onclick: () => { help.hidden = !help.hidden; } }, icon('info', 16)) : null;
  const el = h('div', { class: 'row', 'data-testid': 'row-' + item.id }, h('div', { class: 'lbl' }, h('span', { class: 'name' }, o.label || item.label || item.id), infoBtn), ctl, help, actions);

  const gridBase = item.min ?? 0, gridStep = item.step || 1;
  const onGrid = (v) => { const k = (v - gridBase) / gridStep; return Math.abs(k - Math.round(k)) < 1e-6; };
  const valid = () => {
    if (item.kind === 'number') return num(draft) && onGrid(draft) && (item.min == null || draft >= item.min) && (item.max == null || draft <= item.max);
    if (item.kind === 'date' || item.kind === 'time') return !!draft;
    return draft != null;
  };
  function paintActions() {
    el.classList.toggle('dirty', dirty);
    actions.hidden = !dirty;
    clear(actions);
    if (!dirty) return;
    add(actions, [
      h('button', { class: 'btn small', 'data-testid': 'cancel-' + item.id, disabled: saving, onclick: cancel }, 'Abbrechen'),
      h('button', { class: 'btn small primary', 'data-testid': 'save-' + item.id, disabled: saving || !valid(), onclick: save }, saving ? 'Speichern …' : 'Speichern'),
    ]);
  }
  function setDraft(v, rerender = true) {
    draft = v;
    dirty = !eq(v, cur);
    if (o.instant && dirty && (item.kind === 'enum' || item.kind === 'bool')) return save();
    if (rerender) paintCtl();
    paintActions();
  }
  function cancel() { dirty = false; draft = cur; paintCtl(); paintActions(); }
  async function save() {
    if (item.level === 'installer' && !(await confirmBox('Fachhandwerker-Einstellung', `Achtung: „${item.label || item.id}“ ist eine Fachhandwerker-Einstellung. Falsche Werte können die Anlage beschädigen oder den Betrieb stören. Wirklich ändern?`, 'Ja, ändern', true))) {
      if (o.instant) cancel();
      return;
    }
    saving = true; paintCtl(); paintActions();
    try {
      const r = await api('/api/values/' + encodeURIComponent(item.id), { method: 'PUT', body: JSON.stringify({ value: draft }) });
      res = { value: r.value, text: r.text }; cur = r.value; draft = cur; dirty = false;
      toast('Gespeichert', 'ok');
      if (o.onSaved) o.onSaved(item, r);
    } catch (e) {
      toast(e.message || 'Fehler beim Speichern', 'err');
      if (o.instant) { dirty = false; draft = cur; }
    }
    saving = false; paintCtl(); paintActions();
  }
  function paintCtl() {
    clear(ctl);
    el.classList.remove('stack');
    if (!res) return ctl.append(skeleton());
    if (res.error) return ctl.append(h('span', { class: 'err' }, res.error));
    if (ro) return ctl.append(h('span', { class: 'val', 'data-testid': 'val-' + item.id }, fmtValue(item, res)));
    const val = dirty ? draft : cur;
    const dis = disabled || saving;
    switch (item.kind) {
      case 'number': {
        el.classList.add('stack');
        const step = item.step || 1, d = decOf(item);
        const bump = (dir) => {
          const base = item.min ?? 0;
          let n;
          if (!num(val)) n = base;
          else { const k = (val - base) / step, kr = Math.round(k); n = base + (Math.abs(k - kr) < 1e-6 ? kr + dir : dir > 0 ? Math.ceil(k) : Math.floor(k)) * step; }
          n = Math.round(n * 1e6) / 1e6;
          if (item.min != null) n = Math.max(item.min, n);
          if (item.max != null) n = Math.min(item.max, n);
          setDraft(n);
        };
        ctl.append(h('div', { class: 'stepper' + (o.big ? ' big' : '') },
          h('button', { type: 'button', 'data-testid': 'dec-' + item.id, 'aria-label': 'Weniger', disabled: dis || (num(val) && item.min != null && val <= item.min), onclick: () => bump(-1) }, '−'),
          h('output', { 'data-testid': 'val-' + item.id }, num(val) ? fmtNum(val, d) + (item.unit ? ' ' + item.unit : '') : '—'),
          h('button', { type: 'button', 'data-testid': 'inc-' + item.id, 'aria-label': 'Mehr', disabled: dis || (num(val) && item.max != null && val >= item.max), onclick: () => bump(1) }, '+')));
        break;
      }
      case 'enum': {
        const opts = [...(item.options || [])];
        if (val != null && !opts.some((x) => x.value === val)) opts.push({ value: val, label: String(val) });
        if (opts.length <= 4 && !o.select) {
          ctl.append(h('div', { class: 'seg', role: 'radiogroup' }, opts.map((x) => h('button', { type: 'button', 'data-testid': `opt-${item.id}-${x.value}`, role: 'radio', 'aria-checked': String(x.value === val), disabled: dis, onclick: () => setDraft(x.value) }, x.label))));
          if (opts.length > 2) el.classList.add('stack');
        } else {
          ctl.append(add(h('select', { disabled: dis, onchange: (e) => setDraft(opts[e.target.selectedIndex].value, false) }), opts.map((x) => { const op = h('option', {}, x.label); op.selected = x.value === val; return op; })));
        }
        break;
      }
      case 'bool':
        ctl.append(h('button', { class: 'switch', type: 'button', role: 'switch', 'aria-checked': String(!!val), 'aria-label': item.label, disabled: dis, onclick: () => setDraft(!val) }));
        break;
      case 'date': case 'time':
        ctl.append(h('input', { type: item.kind, value: val || '', disabled: dis, oninput: (e) => setDraft(e.target.value || null, false) }));
        break;
      default:
        ctl.append(h('input', { type: 'text', value: val ?? '', maxlength: item.maxlen > 0 ? item.maxlen : null, disabled: dis, oninput: (e) => setDraft(e.target.value, false) }));
        el.classList.add('stack');
    }
  }
  return {
    el, item,
    set(r) {
      res = r;
      if (r && !r.error) { cur = r.value; if (!dirty) draft = cur; }
      paintCtl(); paintActions();
    },
    get dirty() { return dirty; },
  };
}

const toMin = (t) => { const m = /^(\d\d):(\d\d)$/.exec(t || ''); return m ? +m[1] * 60 + +m[2] : null; };
const toTime = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
const toInp = (t, end) => (end && t === '24:00') ? '00:00' : t;
const fromInp = (v, end) => (end && v === '00:00') ? '24:00' : v;

function validateDay(ws) {
  if (ws.length > 3) return 'Maximal 3 Zeitfenster pro Tag';
  let prev = 0;
  for (const [f, t] of ws) {
    const a = toMin(f), b = toMin(t);
    if (a == null || b == null) return 'Bitte Beginn und Ende angeben';
    if (a % 10 || b % 10) return 'Nur 10-Minuten-Schritte erlaubt';
    if (a >= b) return 'Beginn muss vor dem Ende liegen';
    if (a < prev) return 'Zeitfenster dürfen sich nicht überlappen und müssen aufsteigend sein';
    prev = b;
  }
  return null;
}

function makeTimer(item, o = {}) {
  let res = null, cur = null, draft = null, saving = false;
  const disabled = S.meta.readonly;
  const body = h('div', {});
  const card = h('div', { class: 'card', 'data-testid': 'timer-' + item.id },
    h('div', { class: 'cardtitle' }, icon('clock'), o.label || item.label || item.id),
    item.help ? h('div', { class: 'sub' }, item.help) : null, body);

  const changed = () => DAYS.map(([k]) => k).filter((k) => !eq(cur[k], draft[k]));
  const errors = () => Object.fromEntries(DAYS.map(([k]) => [k, validateDay(draft[k])]));

  function paint() {
    clear(body);
    if (!res) return body.append(h('div', { class: 'loading' }, skeleton(200)));
    if (res.error) return body.append(h('div', { class: 'err sub' }, res.error));
    if (!item.writable) return body.append(...DAYS.map(([k, s]) => h('div', { class: 'row' }, h('span', {}, s), h('span', { class: 'val' }, winText(cur[k])))));
    const errs = errors();
    for (const [k, s, full] of DAYS) {
      const ws = draft[k];
      const lock = disabled || saving;
      const setWin = (i, j, v) => { ws[i][j] = fromInp(v, j === 1); paint(); };
      const last = ws[ws.length - 1];
      const start = last ? toMin(last.length && last[1]) : 360;
      const canAdd = ws.length < 3 && start != null && start < 1440;
      body.append(h('div', { class: 'day' },
        h('div', { class: 'dayhead' }, h('b', { title: full }, s),
          h('div', { class: 'bar' }, ws.map(([f, t]) => { const a = toMin(f), b = toMin(t); return a != null && b != null && b > a ? h('i', { style: `left:${a / 14.4}%;width:${(b - a) / 14.4}%` }) : null; }))),
        ws.map(([f, t], i) => h('div', { class: 'win' },
          h('input', { type: 'time', step: '600', value: toInp(f, false), disabled: lock, 'aria-label': `${full} Beginn`, onchange: (e) => setWin(i, 0, e.target.value) }), '–',
          h('input', { type: 'time', step: '600', value: toInp(t, true), disabled: lock, 'aria-label': `${full} Ende`, onchange: (e) => setWin(i, 1, e.target.value) }),
          h('button', { class: 'x', type: 'button', disabled: lock, 'aria-label': 'Zeitfenster entfernen', onclick: () => { ws.splice(i, 1); paint(); } }, '✕'))),
        errs[k] ? h('div', { class: 'dayerr' }, errs[k]) : null,
        h('div', { class: 'dayact' },
          h('button', { class: 'linkbtn', type: 'button', 'data-testid': `add-${item.id}-${k}`, disabled: lock || !canAdd, onclick: () => { ws.push([toTime(start), toTime(Math.min(start + 60, 1440))]); paint(); } }, '+ Zeitfenster'),
          h('details', { class: 'copy' }, h('summary', {}, 'Kopieren'),
            h('div', { class: 'pop' },
              h('button', { type: 'button', disabled: lock, onclick: () => copyTo(k, DAYS.slice(0, 5).map((d) => d[0])) }, 'Auf alle Werktage'),
              h('button', { type: 'button', disabled: lock, onclick: () => copyTo(k, DAYS.map((d) => d[0])) }, 'Auf alle Tage'))))));
    }
    const ch = changed(), bad = Object.values(errs).some(Boolean);
    body.append(h('div', { class: 'timerbar' },
      h('button', { class: 'btn', disabled: !ch.length || saving, onclick: () => { draft = clone(cur); paint(); } }, 'Abbrechen'),
      h('button', { class: 'btn primary', 'data-testid': 'timer-save-' + item.id, disabled: !ch.length || bad || saving || disabled, onclick: save }, saving ? 'Speichern …' : 'Speichern')));
  }
  function copyTo(from, keys) { for (const k of keys) draft[k] = clone(draft[from]); paint(); }
  async function save() {
    const ch = changed();
    if (item.level === 'installer' && !(await confirmBox('Fachhandwerker-Einstellung', `Achtung: „${item.label || item.id}“ ist eine Fachhandwerker-Einstellung. Wirklich ändern?`, 'Ja, ändern', true))) return;
    saving = true; paint();
    try {
      const r = await api('/api/values/' + encodeURIComponent(item.id), { method: 'PUT', body: JSON.stringify({ value: Object.fromEntries(ch.map((k) => [k, draft[k]])) }) });
      cur = normTimer({ ...cur, ...(r.value || {}) }); draft = clone(cur);
      toast('Gespeichert', 'ok');
    } catch (e) { toast(e.message || 'Fehler beim Speichern', 'err'); }
    saving = false; paint();
  }
  return {
    el: card, item,
    set(r) {
      res = r;
      if (r && !r.error) { const dirty = cur && !eq(cur, draft); cur = normTimer(r.value); if (!dirty) draft = clone(cur); }
      paint();
    },
    get dirty() { return !!cur && !eq(cur, draft); },
  };
}
const normTimer = (v) => Object.fromEntries(DAYS.map(([k]) => [k, ((v && v[k]) || []).filter((w) => w && w[0] && w[1] && w[0] !== '-:-').map((w) => [w[0], w[1]])]));

let routeSeq = 0;
const SRC = { bai: 'Therme', '700': 'Regler' };
const unreadableWarn = (e) => e && e.unreadable && e.unreadable.length
  ? h('div', { class: 'banner subtle' }, icon('warning', 18), `Fehlerstatus konnte nicht gelesen werden (${e.unreadable.map((x) => SRC[x] || x).join(', ')})`) : null;

function loadRows(rows, host) {
  const seq = routeSeq;
  const ids = [...rows.keys()];
  const go = async () => {
    host.querySelector(':scope > .errbox')?.remove();
    for (const r of rows.values()) r.set(null);
    try {
      const vals = await readValues(ids);
      if (seq !== routeSeq) return;
      for (const [id, r] of rows) r.set(vals[id] || { error: 'Kein Wert geliefert' });
    } catch (e) {
      if (seq !== routeSeq) return;
      for (const r of rows.values()) r.set({ error: 'Nicht geladen' });
      host.prepend(h('div', { class: 'errbox' }, h('span', {}, 'Werte konnten nicht gelesen werden: ' + e.message), h('button', { class: 'btn', onclick: go }, 'Erneut versuchen')));
    }
  };
  return go();
}

let cleanup = null;

function features() { return S.meta.features || {}; }
function groupAvailable(g) {
  if (!g.requires) return true;
  const m = /^zone(\d)$/.exec(g.requires);
  if (m) return (features().zones || [1]).includes(+m[1]);
  return !!features()[g.requires];
}
const hol = /^(z\d)?(Bank)?Holiday(StartPeriod|EndPeriod|Temp)$/;
const holName = (id) => id.split('.').slice(1).join('.');
const isHoliday = (id) => hol.test(holName(id));
function groupKind(g) {
  const its = g.items.map(itemOf).filter(Boolean);
  if (its.length && its.every((i) => i.kind === 'timer')) return 'zeit';
  if (its.length && g.items.every(isHoliday)) return 'urlaub';
  return null;
}
function visibleItems(g) { return g.items.map(itemOf).filter((i) => i && (S.unlocked || i.level !== 'installer')); }
function visibleSections() {
  return S.cat.sections.filter((s) => s.level !== 'installer' || S.unlocked)
    .map((s) => ({ ...s, groups: s.groups.filter(groupAvailable).map((g) => ({ ...g, kind: groupKind(g) })) }));
}
function allGroups() { return visibleSections().flatMap((s) => s.groups.map((g) => ({ ...g, section: s }))); }

function navModel() {
  const top = [
    { href: '#/uebersicht', label: 'Übersicht', icon: 'home', key: 'uebersicht' },
    { href: '#/zeit', label: 'Zeitprogramme', icon: 'clock', key: 'zeit' },
    { href: '#/urlaub', label: 'Urlaub', icon: 'suitcase', key: 'urlaub' },
    { href: '#/fehler', label: 'Fehler', icon: 'warning', key: 'fehler' },
  ];
  return top;
}
function routeKey() { const p = (location.hash || '#/uebersicht').slice(2).split('/'); return p[0] === 'g' ? 'g/' + p[1] : p[0] || 'uebersicht'; }

function buildNav() {
  const key = routeKey();
  const link = (n) => h('a', { class: 'navitem' + (n.key === key ? ' active' : ''), href: n.href }, icon(n.icon), n.label);
  const nav = clear($('#nav'));
  const list = h('div', { class: 'navlist' }, navModel().map(link));
  for (const s of visibleSections()) {
    const gs = s.groups.filter((g) => !g.kind);
    if (!gs.length) continue;
    list.append(h('div', { class: 'navsec' }, s.title));
    for (const g of gs) list.append(link({ href: '#/g/' + encodeURIComponent(g.id), label: g.title, icon: g.icon || 'home', key: 'g/' + encodeURIComponent(g.id) }));
  }
  list.append(h('div', { class: 'navsec' }, 'System'));
  if (!S.unlocked) list.append(h('button', { class: 'navitem', onclick: unlockDialog }, icon('lock'), 'Fachhandwerkerebene'));
  list.append(link({ href: '#/info', label: 'Info', icon: 'info', key: 'info' }));
  nav.append(list);
  const tabs = clear($('#tabbar'));
  const tab = (n) => h('a', { href: n.href, class: n.key === key ? 'active' : '' }, icon(n.icon), h('span', {}, n.label));
  add(tabs, [...navModel().slice(0, 4).map((n) => tab({ ...n, label: n.key === 'zeit' ? 'Zeiten' : n.label })),
    tab({ href: '#/menu', label: 'Mehr', icon: 'menu', key: 'menu' })]);
  const badges = [];
  if (S.meta.readonly) badges.push(h('span', { class: 'badge' }, 'Nur-Lese-Modus'));
  clear($('#badges')).append(...badges);
}

function renderBanners() {
  const b = clear($('#banners'));
  if (S.unlocked) b.append(h('div', { class: 'banner subtle' }, icon('wrench', 18), h('span', { class: 'grow' }, 'Fachhandwerkermodus aktiv'), h('button', { class: 'linkbtn', onclick: lock }, 'sperren')));
  if (S.meta.readonly) b.append(h('div', { class: 'banner subtle' }, icon('lock', 18), h('span', { class: 'grow' }, 'Nur-Lese-Modus: Änderungen sind deaktiviert.')));
}
function lock() {
  S.unlocked = false;
  try { sessionStorage.removeItem('fh'); } catch {}
  renderBanners(); buildNav(); route();
}
async function unlockDialog() {
  const ok = await modal((box, close) => {
    const inp = h('input', { type: 'password', inputmode: 'numeric', autocomplete: 'off', placeholder: 'Code', 'data-testid': 'unlock-input' });
    const err = h('div', { class: 'err', hidden: true }, 'Falscher Code');
    const go = () => { if (inp.value.trim() === UNLOCK_CODE) close(true); else { err.hidden = false; inp.select(); } };
    inp.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
    add(box, [h('h2', {}, icon('wrench'), 'Fachhandwerkerebene'), h('p', {}, 'Bitte Code eingeben. Änderungen in dieser Ebene können die Anlage beschädigen.'), inp, err,
      h('div', { class: 'btns' }, h('button', { class: 'btn', onclick: () => close(false) }, 'Abbrechen'), h('button', { class: 'btn primary', onclick: go }, 'Entsperren'))]);
  });
  if (!ok) return;
  S.unlocked = true;
  try { sessionStorage.setItem('fh', '1'); } catch {}
  renderBanners(); buildNav(); route();
}

const fatal = (view, msg, retry) => clear(view).append(h('div', { class: 'errbox' }, h('span', {}, msg), h('button', { class: 'btn', onclick: retry }, 'Erneut versuchen')));
const pageTitle = (t, sub) => [h('h1', {}, t), sub ? h('p', { class: 'sub' }, sub) : null];

function renderGroup(view, id) {
  const g = allGroups().find((x) => x.id === id);
  if (!g) {
    const inst = S.cat.sections.some((s) => s.groups.some((x) => x.id === id));
    return add(view, [h('h1', {}, 'Nicht verfügbar'), inst ? h('div', { class: 'card' }, h('p', {}, 'Diese Seite gehört zur Fachhandwerkerebene.'), h('button', { class: 'btn primary', onclick: unlockDialog }, 'Entsperren')) : h('p', {}, 'Diese Seite existiert nicht.')]);
  }
  const rows = new Map();
  const list = h('div', { class: 'card flush' });
  const timers = h('div', {});
  for (const it of visibleItems(g)) {
    const r = makeRow(it);
    rows.set(it.id, r);
    (it.kind === 'timer' ? timers : list).append(r.el);
  }
  add(view, [pageTitle(g.title), list.children.length ? list : null, timers]);
  loadRows(rows, view);
}

function renderZeit(view) {
  const rows = new Map();
  for (const g of allGroups()) for (const it of visibleItems(g)) if (it.kind === 'timer' && !rows.has(it.id)) rows.set(it.id, makeRow(it));
  add(view, pageTitle('Zeitprogramme', 'Heizzeiten je Wochentag, bis zu 3 Zeitfenster, in 10-Minuten-Schritten.'));
  if (!rows.size) return view.append(h('p', {}, 'Keine Zeitprogramme verfügbar.'));
  for (const r of rows.values()) view.append(r.el);
  loadRows(rows, view);
}

const todayIso = () => new Date().toLocaleDateString('sv-SE');
function holidayStatus(s, e) {
  const t = todayIso();
  if (!s || !e) return ['Nicht geplant', 'muted'];
  if (e < t) return ['Vergangen', 'muted'];
  if (s > t) return ['Geplant', 'warn'];
  return ['Aktiv', ''];
}
function renderUrlaub(view) {
  const gs = allGroups().filter((g) => g.kind === 'urlaub');
  add(view, pageTitle('Urlaub', 'Während des Urlaubs gilt die Abwesenheitstemperatur.'));
  if (!gs.length) return view.append(h('p', {}, 'Keine Urlaubseinstellungen verfügbar.'));
  const all = new Map();
  for (const g of gs) {
    const status = h('span', { class: 'badge muted' }, '…');
    const rows = new Map();
    const card = h('div', { class: 'card' }, h('div', { class: 'cardtitle center' }, h('span', {}, g.title), status));
    const its = visibleItems(g);
    const upd = () => {
      const s = its.find((i) => /^(z\d)?HolidayStartPeriod$/.test(i.name)), e = its.find((i) => /^(z\d)?HolidayEndPeriod$/.test(i.name));
      const sv = rows.get(s?.id)?.cur, ev = rows.get(e?.id)?.cur;
      if (sv === undefined || ev === undefined) return;
      const [t, k] = holidayStatus(sv, ev);
      status.textContent = t; status.className = 'badge ' + k;
    };
    for (const it of its) {
      const r = makeRow(it, { onSaved: (i, r) => { rows.get(i.id).cur = r.value; upd(); } });
      const set = r.set; r.set = (v) => { r.cur = v && !v.error ? v.value : undefined; set(v); upd(); };
      rows.set(it.id, r); all.set(it.id, r); card.append(r.el);
    }
    view.append(card);
  }
  loadRows(all, view);
}

function renderErrors(view) {
  add(view, pageTitle('Fehler'));
  const host = h('div', {});
  view.append(host);
  const seq = routeSeq;
  const go = async () => {
    clear(host).append(h('div', { class: 'loading' }, skeleton(200)));
    try {
      const e = await api('/api/errors');
      if (seq !== routeSeq) return;
      clear(host);
      add(host, [unreadableWarn(e)]);
      host.append(h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('warning'), 'Aktuell'),
        e.current && e.current.length
          ? e.current.map((x) => h('div', { class: 'banner danger' }, icon('warning'), h('span', { class: 'grow' }, x.text || `Fehler ${x.code}`), x.source ? h('span', { class: 'sub' }, x.source) : null))
          : h('p', { class: 'sub' }, 'Keine aktiven Fehler.')));
      host.append(h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('clock'), 'Verlauf'),
        e.history && e.history.length
          ? h('table', { class: 'table' }, h('tr', {}, ['Datum', 'Zeit', 'Fehler'].map((t) => h('th', {}, t))),
            e.history.map((x) => h('tr', {}, h('td', {}, fmtDate(x.date) || '—'), h('td', {}, x.time || '—'), h('td', {}, x.text || `Fehler ${x.code}`))))
          : h('p', { class: 'sub' }, 'Kein Fehlerverlauf.')));
    } catch (e) { if (seq === routeSeq) fatal(host, 'Fehler konnten nicht gelesen werden: ' + e.message, go); }
  };
  go();
}

function renderInfo(view) {
  const m = S.meta;
  add(view, [pageTitle('Info'),
    h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('info'), 'System'),
      infoRow('Version', m.version), infoRow('Modus', m.readonly ? 'Nur-Lese-Modus' : 'Lesen und Schreiben'),
      m.ebusd ? infoRow('ebusd', `${m.ebusd.version || '—'} · Signal ${m.ebusd.signal ? 'ok' : 'fehlt'}`) : null,
      infoRow('Zonen', (m.features?.zones || []).join(', ') || '—'),
      infoRow('Warmwasser', m.features?.hwc ? 'ja' : 'nein')),
    h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('wrench'), 'Geräte'),
      (m.devices || []).length ? h('table', { class: 'table' }, h('tr', {}, ['Kreis', 'Adresse', 'ID', 'Software', 'Hardware'].map((t) => h('th', {}, t))),
        m.devices.map((d) => h('tr', {}, [d.circuit, d.address, d.id, d.sw, d.hw].map((x) => h('td', {}, x ?? '—'))))) : h('p', { class: 'sub' }, 'Keine Geräte gemeldet.')),
    dumpCard()]);
}
function dumpCard() {
  const btn = h('button', { class: 'btn primary', 'data-testid': 'dump' }, 'Konfiguration herunterladen');
  const note = h('p', { class: 'sub' }, 'Alle aktuellen Einstellungen und Messwerte von Regler und Therme als JSON. Das Auslesen dauert bis zu einer Minute.');
  btn.addEventListener('click', async () => {
    btn.disabled = true; btn.textContent = 'Lese alle Werte …';
    try {
      const ctl = new AbortController(); const t = setTimeout(() => ctl.abort(), 180000);
      const r = await fetch('api/dump', { signal: ctl.signal }); clearTimeout(t);
      if (!r.ok) throw new Error('HTTP ' + r.status);
      const name = (/filename="([^"]+)"/.exec(r.headers.get('Content-Disposition') || '') || [])[1] || 'vaillant-config.json';
      const url = URL.createObjectURL(await r.blob());
      const a = h('a', { href: url, download: name }); document.body.append(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
      toast('Konfiguration gespeichert', 'ok');
    } catch (e) { toast('Export fehlgeschlagen: ' + (e.message || e), 'err'); }
    btn.disabled = false; btn.textContent = 'Konfiguration herunterladen';
  });
  return h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('chart'), 'Konfigurations-Export'), note, btn);
}
const infoRow = (k, v) => h('div', { class: 'row' }, h('span', {}, k), h('span', { class: 'val' }, v ?? '—'));

function renderMenu(view) {
  add(view, pageTitle('Mehr'));
  const list = h('div', { class: 'card' }, h('div', { class: 'navlist' }));
  const l = list.firstChild;
  for (const s of visibleSections()) {
    const gs = s.groups.filter((g) => !g.kind);
    if (!gs.length) continue;
    l.append(h('div', { class: 'navsec' }, s.title));
    for (const g of gs) l.append(h('a', { class: 'navitem', href: '#/g/' + encodeURIComponent(g.id) }, icon(g.icon || 'home'), g.title));
  }
  l.append(h('div', { class: 'navsec' }, 'System'));
  if (!S.unlocked) l.append(h('button', { class: 'navitem', onclick: unlockDialog }, icon('lock'), 'Fachhandwerkerebene'));
  l.append(h('a', { class: 'navitem', href: '#/info' }, icon('info'), 'Info'));
  view.append(list);
}

const TILES = [
  { k: 'room', label: 'Raumsoll', id: '700.z1ActualRoomTempDesired', icon: 'home' },
  { k: 'flow', label: 'Vorlauf', re: /^bai\.FlowTemp$/, icon: 'thermometer' },
  { k: 'ret', label: 'Rücklauf', re: /ReturnTemp/, icon: 'thermometer' },
  { k: 'press', label: 'Wasserdruck', re: /WaterPressure/, icon: 'drop' },
  { k: 'flame', label: 'Brenner', re: /^bai\.Flame$/, icon: 'flame' },
  { k: 'pump', label: 'Pumpe', re: /(^bai\.WP|Hc1PumpStatus)$/, icon: 'fan' },
  { k: 'state', label: 'Status', re: /Statenumber$/, icon: 'info' },
  { k: 'mod', label: 'Modulation', re: /Modulation(?!TempDesired)/, icon: 'flame' },
  { k: 'gas', label: 'Gas diesen Monat', re: /FuelSum.*ThisMonth/, icon: 'flame' },
];

function renderOverview(view) {
  const dash = (S.cat.dashboard || []).filter((id) => itemOf(id));
  const used = new Set();
  const outId = dash.find((id) => /DisplayedOutsideTemp$/.test(id)) || dash.find((id) => /OutsideTemp/.test(id));
  if (outId) used.add(outId);
  const tiles = [];
  for (const t of TILES) {
    const id = t.id ? (itemOf(t.id) ? t.id : null) : dash.find((x) => t.re.test(x) && !used.has(x));
    if (id) { used.add(id); tiles.push({ ...t, id }); }
  }
  const ctlRe = /(\.OpMode|\.SFMode|DayTemp|NightTemp|QuickVetoTemp|Holiday\w*)$/;
  for (const id of dash) if (!used.has(id) && !ctlRe.test(id)) { used.add(id); tiles.push({ k: id, label: itemOf(id).label || id, id, icon: 'thermometer' }); }

  const pick = (...c) => c.find((x) => itemOf(x));
  const opId = pick('700.OpMode', '700.z1OpMode'), sfId = pick('700.SFMode', '700.z1SFMode');
  const dayId = pick('700.z1DayTemp'), nightId = pick('700.z1NightTemp'), vetoId = pick('700.z1QuickVetoTemp');

  const heroVal = h('div', { class: 'big' }, '—'), heroChip = h('div', { class: 'chip' });
  const hero = h('div', { class: 'hero' }, h('div', {}, h('div', { class: 'cap' }, 'Außentemperatur'), heroVal), heroChip);
  const tileEls = new Map(tiles.map((t) => [t.id, h('div', { class: 'tile' }, h('div', { class: 'cap' }, icon(t.icon, 16), t.label), h('div', { class: 'v' }, skeleton(60)))]));
  const banner = h('div', {});
  const status = h('div', { class: 'status' });
  const seq = routeSeq;
  const refreshAfter = () => setTimeout(() => { if (seq === routeSeq) refresh(true); }, 1500);

  const rows = new Map();
  const mk = (id, o) => { if (!id) return null; const r = makeRow(itemOf(id), { onSaved: refreshAfter, ...o }); rows.set(id, r); return r; };
  const opRow = mk(opId, { label: 'Betriebsart', instant: true });
  const dayRow = mk(dayId, { label: 'Wunschtemperatur Tag', big: true });
  const nightRow = mk(nightId, { label: 'Absenktemperatur', big: true });
  const vetoRow = mk(vetoId, { label: 'Temperatur bei „Manuell“' });
  let sf = null, sfBox = null;
  if (sfId) {
    const it = itemOf(sfId);
    sfBox = h('div', { class: 'quick' });
    sf = { cur: undefined };
    const sfOff = S.meta.readonly || !it.writable;
    const send = async (val) => {
      if (sfOff) return;
      try { await api('/api/values/' + encodeURIComponent(sfId), { method: 'PUT', body: JSON.stringify({ value: val }) }); toast('Gespeichert', 'ok'); sf.cur = val; paintSf(); refreshAfter(); }
      catch (e) { toast(e.message, 'err'); }
    };
    var paintSf = () => {
      clear(sfBox);
      if (sf.cur === undefined) return sfBox.append(skeleton(160));
      const opts = (it.options || []).filter((o) => o.value !== 'auto' && (o.value !== 'load' || features().hwc));
      for (const o of opts) sfBox.append(h('button', { class: 'btn' + (sf.cur === o.value ? ' active' : ''), 'data-testid': 'sf-' + o.value, disabled: sfOff, onclick: () => send(o.value) }, o.label));
      if (sf.cur !== 'auto' && sf.cur != null) sfBox.append(h('button', { class: 'btn primary', 'data-testid': 'sf-auto', style: 'grid-column:1/-1', disabled: sfOff, onclick: () => send('auto') }, 'Beenden'));
    };
    paintSf();
  }

  add(view, [banner, hero, h('div', { class: 'tiles' }, [...tileEls.values()]),
    h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('flame'), 'Heizen'), opRow && opRow.el,
      dayRow && dayRow.el, nightRow && nightRow.el),
    sfBox ? h('div', { class: 'card' }, h('div', { class: 'cardtitle' }, icon('clock'), 'Schnellwahl'), sfBox, vetoRow && vetoRow.el) : null,
    status]);

  const ids = [...new Set([...(outId ? [outId] : []), ...tiles.map((t) => t.id), ...rows.keys(), ...(sfId ? [sfId] : [])])];
  let busy = false;
  async function refresh(fresh) {
    if (busy) return;
    busy = true;
    const [v, e] = await Promise.allSettled([readValues(ids, fresh === true), api('/api/errors')]);
    busy = false;
    if (seq !== routeSeq) return;
    if (v.status === 'fulfilled') {
      const vals = v.value;
      const R = (id) => vals[id] || { error: 'Kein Wert' };
      if (outId) { const r = R(outId); heroVal.textContent = r.error ? '—' : fmtValue(itemOf(outId), r).replace(' ', ' '); }
      let flameOn = null;
      for (const t of tiles) {
        const it = itemOf(t.id), r = R(t.id), box = tileEls.get(t.id);
        const vEl = box.querySelector('.v');
        box.className = 'tile';
        box.hidden = r.value == null && !r.error;
        let txt = r.error ? r.error : fmtValue(it, r);
        if (r.error) box.classList.add('warn');
        if (t.k === 'press' && num(r.value)) box.classList.add(r.value < 0.8 ? 'bad' : r.value < 1.0 ? 'warn' : 'x');
        if (t.k === 'flame' && !r.error) { flameOn = r.text === 'An' ? true : r.text === 'Aus' ? false : null; if (flameOn) box.classList.add('on'); }
        if (t.k === 'pump' && !r.error) { const on = r.value === true || r.value === 1 || r.value === 'on'; txt = it.kind === 'enum' ? txt : on ? 'An' : 'Aus'; if (r.value === true || r.value === 1 || r.value === 'on') box.classList.add('on'); }
        vEl.className = 'v' + (t.k === 'state' || r.error ? ' small' : '');
        vEl.textContent = txt;
        vEl.title = r.error || '';
      }
      clear(heroChip).append(icon('flame', 18), flameOn == null ? 'Brenner —' : flameOn ? 'Brenner an' : 'Brenner aus');
      for (const [id, r] of rows) r.set(R(id));
      if (sf) { sf.cur = R(sfId).error ? undefined : R(sfId).value; paintSf(); }
      clear(status).append('Aktualisiert ' + new Date().toLocaleTimeString('de-DE') + ' ', h('button', { class: 'linkbtn', onclick: () => refresh(true) }, 'Neu lesen'));
    } else {
      clear(status).append('Lesefehler: ' + v.reason.message + ' ', h('button', { class: 'linkbtn', onclick: () => refresh(true) }, 'Erneut versuchen'));
    }
    clear(banner);
    if (e.status === 'rejected') banner.append(h('div', { class: 'banner subtle' }, icon('warning', 18), 'Fehlerstatus konnte nicht gelesen werden'));
    else add(banner, [unreadableWarn(e.value)]);
    if (e.status === 'fulfilled' && (e.value.current && e.value.current.length))
      banner.append(h('a', { class: 'banner danger', href: '#/fehler', style: 'text-decoration:none;color:inherit' }, icon('warning'),
        h('span', { class: 'grow' }, e.value.current.map((x) => x.text || `Fehler ${x.code}`).join(' · ')), 'Details'));
  }
  const timer = setInterval(() => { if (!document.hidden && ![...rows.values()].some((r) => r.dirty)) refresh(); }, 15000);
  const vis = () => { if (!document.hidden) refresh(); };
  document.addEventListener('visibilitychange', vis);
  cleanup = () => { clearInterval(timer); document.removeEventListener('visibilitychange', vis); };
  refresh();
}

function route() {
  if (cleanup) { cleanup(); cleanup = null; }
  routeSeq++;
  const view = clear($('#view'));
  buildNav();
  window.scrollTo(0, 0);
  const parts = (location.hash || '#/uebersicht').slice(2).split('/');
  try {
    switch (parts[0]) {
      case 'g': renderGroup(view, decodeURIComponent(parts[1] || '')); break;
      case 'zeit': renderZeit(view); break;
      case 'urlaub': renderUrlaub(view); break;
      case 'fehler': renderErrors(view); break;
      case 'info': renderInfo(view); break;
      case 'menu': renderMenu(view); break;
      default: renderOverview(view);
    }
  } catch (e) { fatal(view, 'Seite konnte nicht angezeigt werden: ' + e.message, route); }
}

async function boot() {
  const view = $('#view');
  for (const s of ['#brandTop', '#brandSide']) {
    clear($(s)).append(h('img', { class: 'logo', src: 'logo.svg', alt: 'Vaillant' }), h('span', { class: 'brandtxt' }, 'Heizungssteuerung'));
  }
  clear(view).append(h('div', { class: 'loading' }, 'Lade …'));
  try {
    const [meta, cat] = await Promise.all([api('/api/meta'), api('/api/catalog')]);
    S.meta = meta; S.cat = cat; S.items = cat.items || {};
  } catch (e) { return fatal(view, 'Verbindung zur Heizung fehlgeschlagen: ' + e.message, boot); }
  renderBanners();
  window.addEventListener('hashchange', route);
  route();
}
boot();
