import { Events, Window } from "@wailsio/runtime";
import { t, setLocale, type MessageKey } from "./i18n";
import { Service, type AppInfo, type Settings } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/gui";
import type { Activity, Stats, Status } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/session";
import type { Recent } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/config";
import type { Device } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/bluetooth";

// ---------------------------------------------------------------- state

type View = "main" | "settings";
type Transport = "wifi" | "bt";

let view: View = "main";
let status: Status = { state: "disconnected", addr: "", paused: false, connected_at: "", retrying: false, error: "", detail: "" };
let stats: Stats = { keys: 0, texts: 0, last_frame_ms: -1 };
let activities: Activity[] = [];
let recent: Recent[] = [];
let settings: Settings | null = null;
let info: AppInfo = { version: "", elevated: false, elevation_needed: false };
let localIP = "";
let draftAddr = "";
let transport: Transport = "wifi";
/** Bluetooth devices paired with this PC; null until first loaded */
let devices: Device[] | null = null;
/** Why the paired list could not be read, e.g. Bluetooth is off */
let btUnavailable = false;
let loadingDevices = false;
let focusAddr = true;
/** IDs of text entries the user expanded to read in full */
const expanded = new Set<number>();
/** Text longer than this is likely cut off in one line and gets a toggle. */
const EXPAND_CHARS = 16;

const app = document.getElementById("app")!;
const DEFAULT_PORT = ":8088";

// ---------------------------------------------------------------- helpers

type Child = Node | string | null | false | undefined;

/** h builds an element; attributes starting with "on" become listeners. */
function h(tag: string, attrs: Record<string, unknown> = {}, ...children: Child[]): HTMLElement {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === false || v === undefined || v === null) continue;
    if (k.startsWith("on") && typeof v === "function") el.addEventListener(k.slice(2), v as EventListener);
    else if (k === "class") el.className = String(v);
    else if (k === "html") el.innerHTML = String(v); // only for our own SVG constants
    else el.setAttribute(k, v === true ? "" : String(v));
  }
  for (const c of children) if (c) el.append(c);
  return el;
}

const icon = {
  settings: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>`,
  chevron: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M6 9l6 6 6-6"/></svg>`,
  back: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M15 18l-6-6 6-6"/></svg>`,
  power: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M12 3v9"/><path d="M6.3 7.3a8 8 0 1 0 11.4 0"/></svg>`,
  phone: (size: number) => `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/></svg>`,
  pause: `<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>`,
  pauseBig: `<svg width="26" height="26" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>`,
  play: `<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M7 5l12 7-12 7z"/></svg>`,
  alert: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg>`,
  bt: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 7l10 10-5 5V2l5 5L7 17"/></svg>`,
  infoI: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>`,
};

/** host drops the default port for display. */
function host(addr: string): string {
  return addr.endsWith(DEFAULT_PORT) ? addr.slice(0, -DEFAULT_PORT.length) : addr;
}

const isBT = (addr: string) => addr.toLowerCase().startsWith("bt:");

/** label shows a paired phone's name instead of its Bluetooth address. */
function label(addr: string): string {
  if (!isBT(addr)) return host(addr);
  return devices?.find((d) => d.addr === addr)?.name || addr.slice(3);
}

function clock(iso: string): string {
  const d = new Date(iso);
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map((n) => String(n).padStart(2, "0")).join(":");
}

function duration(seconds: number): string {
  const h = Math.floor(seconds / 3600), m = Math.floor((seconds % 3600) / 60), s = seconds % 60;
  if (h > 0) return t("hoursMinutes", { hours: h, minutes: m });
  if (m > 0) return t("minutesSeconds", { minutes: m, seconds: String(s).padStart(2, "0") });
  return t("seconds", { seconds: s });
}

function when(iso: string): string {
  const d = new Date(iso), now = new Date();
  const hm = `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  const days = Math.round((new Date(now.toDateString()).getTime() - new Date(d.toDateString()).getTime()) / 86400000);
  if (days === 0) return t("today", { time: hm });
  if (days === 1) return t("yesterday", { time: hm });
  return t("date", { month: d.getMonth() + 1, day: d.getDate(), time: hm });
}

let toastTimer = 0;
function toast(msg: string) {
  document.querySelector(".toast")?.remove();
  const el = h("div", { class: "toast", role: "status" }, msg);
  document.body.append(el);
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => el.remove(), 2600);
}

// ---------------------------------------------------------------- errors

const errors: Record<string, { title: MessageKey; tips?: MessageKey[] }> = {
  refused: { title: "refused", tips: ["checkPhoneListening", "checkSameWifi", "checkMatchingPort"] },
  timeout: { title: "timeout", tips: ["checkSameWifi", "checkIsolation", "checkMatchingAddress"] },
  unreachable: { title: "unreachable", tips: ["checkNetwork", "checkMatchingAddress"] },
  bad_address: { title: "badAddress", tips: ["addressFormatHint"] },
  lost: { title: "lost", tips: ["lostHint"] },
  other: { title: "connectionFailed" },
};

const btErrors: Record<string, { title: MessageKey; tips?: MessageKey[] }> = {
  refused: { title: "bluetoothRefused", tips: ["checkBluetoothListening", "checkPaired"] },
  timeout: { title: "timeout", tips: ["checkBluetoothEnabled", "checkRangeMeters"] },
  unreachable: { title: "bluetoothUnreachable", tips: ["checkBluetoothEnabled", "checkRange"] },
  no_bluetooth: { title: "noBluetooth", tips: ["enableBluetooth"] },
  lost: { title: "bluetoothLost", tips: ["bluetoothLostHint"] },
};

function errorBanner(): HTMLElement | null {
  if (!status.error || status.state === "connected") return null;
  if (status.error === "phone_closed") {
    return h("div", { class: "banner info", role: "status" }, h("span", { html: icon.infoI }), h("div", {}, t("phoneDisconnected")));
  }
  const e = (isBT(status.addr) ? btErrors[status.error] : undefined) ?? errors[status.error] ?? errors.other;
  return h("div", { class: "banner err", role: "alert" },
    h("span", { html: icon.alert }),
    h("div", {},
      h("b", {}, t(e.title)),
      e.tips ? h("ol", {}, ...e.tips.map((tip) => h("li", {}, t(tip)))) : h("div", { class: "caption" }, status.detail),
    ));
}

// ---------------------------------------------------------------- actions

async function connect(addr: string) {
  addr = addr.trim();
  if (!addr) {
    toast(t("enterAddress"));
    return;
  }
  draftAddr = addr;
  try {
    await Service.Connect(addr);
  } catch {
    // reported through the status event
  }
  recent = (await Service.Recent()) ?? [];
  render();
}

async function loadDevices() {
  loadingDevices = true;
  render();
  try {
    devices = (await Service.PairedDevices()) ?? [];
    btUnavailable = false;
  } catch {
    devices = [];
    btUnavailable = true;
  }
  loadingDevices = false;
  render();
}

function setTransport(t: Transport) {
  transport = t;
  focusAddr = t === "wifi";
  if (t === "bt" && devices === null) loadDevices();
  else render();
}

async function refreshRecent() {
  recent = (await Service.Recent()) ?? [];
  render();
}

let savingSettings = false;
async function saveSettings(patch: Partial<Settings>) {
  if (!settings) return;
  const next = { ...settings, ...patch };
  // Each control submits a full settings snapshot. Wait for the active save.
  if (savingSettings) return;
  savingSettings = true;
  render();
  try {
    await Service.SaveSettings(next);
    settings = next;
  } catch (err) {
    toast(t("saveFailed", { error: String(err) }));
    settings = await Service.Settings();
  } finally {
    savingSettings = false;
    render();
  }
}

// ---------------------------------------------------------------- views

function render() {
  setLocale(settings?.language);
  document.title = t("appName");
  document.getElementById("app-title")!.textContent = document.title;
  document.getElementById("btn-min")!.setAttribute("aria-label", t("minimize"));
  document.getElementById("btn-close")!.setAttribute("aria-label", t("close"));
  const active = document.activeElement as HTMLElement | null;
  const keepFocus = active?.id;
  const scroll = document.querySelector(".log .list")?.scrollTop ?? 0;
  app.replaceChildren(...(view === "settings" ? settingsView() : status.state === "connected" ? connectedView() : connectView()));
  const list = document.querySelector(".log .list");
  if (list) list.scrollTop = scroll;
  if (keepFocus) document.getElementById(keepFocus)?.focus();
  else if (focusAddr && view === "main" && transport === "wifi" && status.state !== "connected") document.getElementById("addr")?.focus();
  focusAddr = false;
}

function header(title: string): HTMLElement {
  return h("div", { class: "row" },
    h("h1", { class: "h1" }, title), h("div", { class: "grow" }),
    h("button", { class: "iconbtn", "aria-label": t("settings"), title: t("settings"), html: icon.settings, onclick: () => openSettings() }));
}

function connectView(): Node[] {
  const busy = status.state === "connecting" || status.retrying;
  const input = h("input", {
    id: "addr", value: draftAddr, placeholder: t("exampleAddress"), inputmode: "decimal",
    "aria-label": t("phoneAddress"), autocomplete: "off", spellcheck: "false", disabled: busy,
    oninput: (e: Event) => { draftAddr = (e.target as HTMLInputElement).value; },
    onkeydown: (e: KeyboardEvent) => { if (e.key === "Enter") connect(draftAddr); },
  }) as HTMLInputElement;

  const statusLine = busy
    ? h("span", { class: "status", role: "status" }, h("span", { class: "dot wait pulse" }),
        status.retrying ? t("waiting", { phone: label(status.addr) })
          : t("connecting", { phone: label(status.addr), seconds: isBT(status.addr) ? 20 : 12 }))
    : h("span", { class: "status" }, h("span", { class: "dot" }), t("disconnected"));

  const tabs = h("div", { class: "seg", role: "tablist", "aria-label": t("transport") },
    ...([["wifi", "Wi‑Fi"], ["bt", t("bluetooth")]] as const).map(([k, text]) =>
      h("button", { role: "tab", class: transport === k ? "on" : "", "aria-selected": String(transport === k), disabled: busy, onclick: () => setTransport(k) }, text)));

  const card = transport === "bt" ? btCard(busy, statusLine) : h("div", { class: "card stack" },
    statusLine,
    h("label", { class: "caption", for: "addr" }, t("wifiAddressHint")),
    h("div", { class: "input" }, input, settings && settings.port !== 8088 ? h("span", { class: "suffix" }, `:${settings.port}`) : h("span", { class: "suffix" }, DEFAULT_PORT)),
    errorBanner(),
    busy
      ? h("button", { class: "btn ghost", onclick: () => Service.Cancel() }, t("cancel"))
      : h("button", { class: "btn primary", html: icon.power + (status.error ? t("retry") : t("connect")), onclick: () => connect(draftAddr) }),
    !status.error && !busy && h("div", { class: "caption" }, t("wifiInstructions")),
  );

  const list = recent.length > 0 && h("div", { class: "card", style: "padding:10px 14px" },
    h("div", { class: "caption", style: "margin:4px 0 4px 4px" }, t("recent")),
    h("div", {}, ...recent.map((r) => h("div", { class: "item" },
      h("div", { class: "avatar", html: isBT(r.addr) ? icon.bt : icon.phone(18) }),
      h("div", { class: "grow" },
        h("div", { class: "name" }, label(r.addr)),
        h("div", { class: "sub" }, when(r.last_at) + (r.duration_seconds > 0 ? t("previousDuration", { duration: duration(r.duration_seconds) }) : ""))),
      h("button", { class: "forget", "aria-label": t("removePhone", { phone: label(r.addr) }), title: t("removeRecent"), onclick: async () => { await Service.ForgetRecent(r.addr); refreshRecent(); } }, "✕"),
      h("button", { class: "btn ghost xs", disabled: busy, onclick: () => connect(r.addr) }, t("connect")),
    ))));

  return [
    header(t("connectPhone")),
    tabs,
    card,
    list || null,
    h("div", { class: "grow" }),
    h("div", { class: "caption center" }, transport === "bt" ? t("bluetoothRange")
      : localIP ? t("localAddress", { ip: localIP }) : t("noLan")),
  ].filter(Boolean) as Node[];
}

/** The Bluetooth tab: pick a paired phone instead of typing an address. */
function btCard(busy: boolean, statusLine: Node): HTMLElement {
  let list: Node | null;
  if (devices === null || loadingDevices) list = h("div", { class: "empty" }, t("loadingDevices"));
  else if (btUnavailable) list = null;
  else if (devices.length === 0) list = h("div", { class: "empty" }, t("noPairedDevices"));
  else list = h("div", {}, ...devices.map((d) => h("div", { class: "item" },
    h("div", { class: "avatar", html: d.phone ? icon.phone(18) : icon.bt }),
    h("div", { class: "grow" },
      h("div", { class: "name" }, d.name || d.addr.slice(3)),
      h("div", { class: "sub" }, (d.phone ? t("phonePrefix") : "") + d.addr.slice(3))),
    h("button", { class: "btn ghost xs", disabled: busy, onclick: () => connect(d.addr) }, t("connect")))));

  const unavailable = btUnavailable && !status.error
    ? h("div", { class: "banner err", role: "alert" }, h("span", { html: icon.alert }), h("div", {}, t("bluetoothUnavailable")))
    : null;

  return h("div", { class: "card stack" },
    statusLine,
    h("div", { class: "row" },
      h("span", { class: "caption grow" }, t("pairedDevices")),
      h("button", { class: "linkbtn", disabled: loadingDevices || busy, onclick: loadDevices }, t("refresh"))),
    list,
    errorBanner() ?? unavailable,
    busy
      ? h("button", { class: "btn ghost", onclick: () => Service.Cancel() }, t("cancel"))
      : h("div", { class: "caption" }, t("bluetoothInstructions")),
    !busy && h("button", { class: "btn ghost sm", onclick: () => Service.OpenBluetoothSettings() }, t("openBluetoothSettings")));
}

function connectedView(): Node[] {
  const paused = status.paused;
  const elapsed = status.connected_at ? Math.max(0, Math.floor((Date.now() - new Date(status.connected_at).getTime()) / 1000)) : 0;
  const heartbeatOk = stats.last_frame_ms >= 0 && stats.last_frame_ms < 3000;

  const hero = h("div", { class: "hero" },
    h("div", { class: paused ? "phone paused" : "phone", html: paused ? icon.pauseBig : icon.phone(26) }),
    h("div", { class: "grow" },
      paused
        ? h("span", { class: "status", style: "color:var(--action)" }, h("span", { class: "dot wait" }), t("paused"))
        : h("span", { class: "status", style: "color:#7DD3AC" }, h("span", { class: "dot ok" }), t("connected")),
      h("div", { class: "addr" }, label(status.addr)),
      h("div", { class: "sub", id: "elapsed" }, paused ? t("pausedHint") : t("connectedDuration", { duration: duration(elapsed) }))));

  const card = h("div", { class: "card" },
    hero,
    !paused && h("div", { class: "stats" },
      h("div", { class: "stat" }, h("b", {}, String(stats.keys)), h("span", { class: "caption" }, t("keys"))),
      h("div", { class: "stat" }, h("b", {}, String(stats.texts)), h("span", { class: "caption" }, t("texts"))),
      h("div", { class: heartbeatOk ? "stat" : "stat bad" },
        h("b", {}, heartbeatOk ? t("heartbeatOk") : `${Math.round(stats.last_frame_ms / 1000)}s`),
        h("span", { class: "caption" }, t("heartbeat")))),
    h("div", { class: "row", style: "margin-top:14px;gap:8px" },
      paused
        ? h("button", { class: "btn primary sm", html: icon.play + t("resumeInput"), onclick: () => Service.SetPaused(false) })
        : h("button", { class: "btn warn sm", html: icon.pause + t("pauseInput"), onclick: () => Service.SetPaused(true) }),
      h("button", { class: "btn ghost sm", onclick: () => Service.Disconnect() }, t("disconnect"))));

  const items = [...activities].reverse();
  const log = h("div", { class: "card log" },
    h("div", { class: "head" },
      h("span", { class: "caption" }, t("activity")),
      items.length > 0 && h("button", { class: "linkbtn", onclick: async () => { await Service.ClearActivities(); activities = []; render(); } }, t("clear"))),
    h("div", { class: "list", role: "log", "aria-live": "polite" },
      ...(items.length ? items.map(activityRow) : [h("div", { class: "empty" }, t("emptyActivity"))])));

  return [card, log];
}

function keys(list: string[] | null | undefined): Node[] {
  const out: Node[] = [];
  (list ?? []).forEach((k, i) => {
    if (i > 0) out.push(document.createTextNode("+"));
    out.push(h("span", { class: "kbd" }, k));
  });
  return out;
}

function activityRow(a: Activity): HTMLElement {
  const timestamp = h("time", {}, clock(a.at));
  let body: Node[];
  switch (a.kind) {
    case "combo":
      body = [h("span", { class: "tag key" }, t("combo")), ...keys(a.keys)];
      break;
    case "hold":
      body = [h("span", { class: "tag key" }, t("hold")), ...keys(a.keys), h("span", { class: "caption" }, t("holdDuration", { seconds: ((a.hold_ms ?? 0) / 1000).toFixed(1) }))];
      break;
    case "text":
      body = [h("span", { class: "tag text" }, t("text")),
        h("span", { class: "txt" }, a.text ? a.text : a.chars ? t("characters", { count: a.chars }) : t("enterOnly")),
        a.enter ? h("span", { class: "kbd" }, "↵") : null].filter(Boolean) as Node[];
      if (a.text && (a.chars ?? 0) > EXPAND_CHARS) return expandableRow(a, timestamp, body);
      break;
    case "lock":
      body = [h("span", { class: "tag sys" }, t("lock")), h("span", { class: "txt" }, t("lockPc"))];
      break;
    case "pause":
      body = [h("span", { class: "tag sys" }, t("pause")), h("span", { class: "txt" }, t("pausedOnPc"))];
      break;
    case "resume":
      body = [h("span", { class: "tag sys" }, t("resume")), h("span", { class: "txt" }, t("resumedOnPc"))];
      break;
    default:
      body = [h("span", { class: "tag key" }, t("keys")), ...keys(a.keys)];
  }
  if (!a.applied) body.push(h("span", { class: "caption" }, t("ignored")));
  return h("div", { class: a.applied ? "ev" : "ev dropped" }, timestamp, ...body);
}

/** A text entry that opens to show the whole text; click again to fold. */
function expandableRow(a: Activity, timestamp: Node, body: Node[]): HTMLElement {
  const open = expanded.has(a.id);
  const flip = () => {
    if (window.getSelection()?.toString()) return; // the user is selecting text to copy
    if (open) expanded.delete(a.id);
    else expanded.add(a.id);
    render();
  };
  return h("div", {
    class: `ev expandable${open ? " open" : ""}${a.applied ? "" : " dropped"}`,
    role: "button", tabindex: "0", "aria-expanded": String(open),
    title: open ? t("collapse") : t("expand"),
    onclick: flip,
    onkeydown: (e: KeyboardEvent) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); flip(); } },
  }, timestamp, ...body, h("span", { class: "chev", html: icon.chevron }));
}

function toggle(id: string, label: string, sub: string | null, checked: boolean, key: keyof Settings): HTMLElement {
  return h("div", { class: "set" },
    h("label", { for: id }, label, sub && h("div", { class: "sub" }, sub)),
    h("input", { type: "checkbox", disabled: savingSettings, role: "switch", class: "sw", id, checked, onchange: (e: Event) => saveSettings({ [key]: (e.target as HTMLInputElement).checked }) }));
}

function settingsView(): Node[] {
  if (!settings) return [];
  const s = settings;
  return [
    h("div", { class: "row" },
      h("button", { class: "iconbtn", "aria-label": t("back"), html: icon.back, onclick: () => { view = "main"; render(); } }),
      h("h1", { class: "h1" }, t("settings"))),
    h("div", { class: "section" }, t("connectionSection")),
    h("div", { class: "card", style: "padding:6px 16px" },
      h("div", { class: "set" },
        h("label", { for: "port" }, t("port"), h("div", { class: "sub" }, t("portHint"))),
        h("input", {
          class: "port", id: "port", disabled: savingSettings, type: "number", min: "1", max: "65535", value: String(s.port),
          onchange: (e: Event) => {
            const v = Number((e.target as HTMLInputElement).value);
            if (Number.isInteger(v) && v > 0 && v <= 65535) saveSettings({ port: v });
            else { toast(t("invalidPort")); render(); }
          },
        })),
      toggle("auto-connect", t("autoConnect"), t("autoConnectHint"), s.auto_connect, "auto_connect"),
      toggle("auto-reconnect", t("autoReconnect"), null, s.auto_reconnect, "auto_reconnect")),
    h("div", { class: "section" }, t("window")),
    h("div", { class: "card", style: "padding:6px 16px" },
      toggle("start-on-boot", t("launchAtStartup"), t("launchAtStartupHint"), s.start_on_boot, "start_on_boot"),
      toggle("close-to-tray", t("closeToTray"), null, s.close_to_tray, "close_to_tray"),
      toggle("show-text", t("showText"), t("showTextHint"), s.show_text, "show_text"),
      h("div", { class: "set" },
        h("label", { for: "language" }, t("language")),
        h("select", {
          id: "language", disabled: savingSettings,
          onchange: (e: Event) => saveSettings({ language: (e.target as HTMLSelectElement).value }),
        },
        h("option", { value: "zh-CN", selected: s.language !== "en" }, "简体中文"),
        h("option", { value: "en", selected: s.language === "en" }, "English")))),
    info.elevation_needed && !info.elevated && h("div", { class: "banner info" },
      h("span", { html: icon.infoI }),
      h("div", {}, t("adminHint") + " ",
        h("a", { role: "button", tabindex: "0", onclick: restartAsAdmin, onkeydown: (e: KeyboardEvent) => { if (e.key === "Enter") restartAsAdmin(); } }, t("restartAsAdmin")))),
    h("div", { class: "grow" }),
    h("div", { class: "caption center" }, t("version", { version: info.version }) + (info.elevated ? t("administratorSuffix") : "")),
  ].filter(Boolean) as Node[];
}

async function restartAsAdmin() {
  try {
    await Service.RestartAsAdmin();
  } catch {
    toast(t("restartFailed"));
  }
}

async function openSettings() {
  settings = await Service.Settings();
  view = "settings";
  render();
}

// ---------------------------------------------------------------- wiring

document.getElementById("btn-min")!.addEventListener("click", () => Window.Minimise());
document.getElementById("btn-close")!.addEventListener("click", () => Window.Close());

Events.On("status", (e) => {
  const prev = status;
  status = e.data as Status;
  if (prev.state !== status.state) {
    if (status.state === "connected") stats = { keys: 0, texts: 0, last_frame_ms: 0 };
    refreshRecent();
    Service.LocalIP().then((ip) => { localIP = ip; });
  }
  render();
});
Events.On("stats", (e) => {
  stats = e.data as Stats;
  if (view === "main" && status.state === "connected") render();
});
Events.On("activity", (e) => {
  const a = e.data as Activity;
  const i = activities.findIndex((x) => x.id === a.id);
  if (i >= 0) activities[i] = a;
  else activities.push(a);
  if (activities.length > 200) activities.splice(0, activities.length - 200);
  if (view === "main" && status.state === "connected") render();
});
Events.On("navigate", (e) => {
  if (e.data === "settings") openSettings();
});

async function init() {
  [status, stats, activities, recent, settings, info, localIP] = await Promise.all([
    Service.Status(), Service.Stats(), Service.Activities().then((a) => a ?? []),
    Service.Recent().then((r) => r ?? []), Service.Settings(), Service.Info(), Service.LocalIP(),
  ]);
  const last = await Service.LastAddr();
  if (isBT(last)) transport = "bt";
  else draftAddr = host(last);
  focusAddr = transport === "wifi";
  render();
  // names for Bluetooth entries in the recent list, and the list for the tab
  if (transport === "bt" || recent.some((r) => isBT(r.addr))) loadDevices();
}

init();
