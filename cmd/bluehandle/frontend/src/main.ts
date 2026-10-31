import { Events, Window } from "@wailsio/runtime";
import { Service, type AppInfo, type Settings } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/gui";
import type { Activity, Stats, Status } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/session";
import type { Recent } from "../bindings/github.com/bluehandle-open/bluehandle-go/internal/config";

// ---------------------------------------------------------------- state

type View = "main" | "settings";

let view: View = "main";
let status: Status = { state: "disconnected", addr: "", paused: false, connected_at: "", retrying: false, error: "", detail: "" };
let stats: Stats = { keys: 0, texts: 0, last_frame_ms: -1 };
let activities: Activity[] = [];
let recent: Recent[] = [];
let settings: Settings | null = null;
let info: AppInfo = { version: "", elevated: false, elevation_needed: false };
let localIP = "";
let draftAddr = "";
let focusAddr = true;

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
  back: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M15 18l-6-6 6-6"/></svg>`,
  power: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M12 3v9"/><path d="M6.3 7.3a8 8 0 1 0 11.4 0"/></svg>`,
  phone: (size: number) => `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/></svg>`,
  pause: `<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>`,
  pauseBig: `<svg width="26" height="26" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>`,
  play: `<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M7 5l12 7-12 7z"/></svg>`,
  alert: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg>`,
  infoI: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>`,
};

/** host drops the default port for display. */
function host(addr: string): string {
  return addr.endsWith(DEFAULT_PORT) ? addr.slice(0, -DEFAULT_PORT.length) : addr;
}

function clock(iso: string): string {
  const d = new Date(iso);
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map((n) => String(n).padStart(2, "0")).join(":");
}

function duration(seconds: number): string {
  const h = Math.floor(seconds / 3600), m = Math.floor((seconds % 3600) / 60), s = seconds % 60;
  if (h > 0) return `${h} 小时 ${m} 分`;
  if (m > 0) return `${m} 分 ${String(s).padStart(2, "0")} 秒`;
  return `${s} 秒`;
}

function when(iso: string): string {
  const d = new Date(iso), now = new Date();
  const hm = `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  const days = Math.round((new Date(now.toDateString()).getTime() - new Date(d.toDateString()).getTime()) / 86400000);
  if (days === 0) return `今天 ${hm}`;
  if (days === 1) return `昨天 ${hm}`;
  return `${d.getMonth() + 1} 月 ${d.getDate()} 日 ${hm}`;
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

const errors: Record<string, { title: string; tips?: string[] }> = {
  refused: { title: "手机拒绝了连接", tips: ["手机上是否已点「开始连接」并显示“等待电脑接入”", "手机与电脑是否连着同一个 Wi‑Fi", "手机设置里的端口是否和这里一致"] },
  timeout: { title: "连接超时", tips: ["手机与电脑是否连着同一个 Wi‑Fi", "路由器是否开启了“访客网络”或“AP 隔离”", "地址是否和手机首页显示的一致"] },
  unreachable: { title: "找不到这个地址", tips: ["电脑是否已连上网络", "地址是否和手机首页显示的一致"] },
  bad_address: { title: "地址格式不对", tips: ["请填写手机首页显示的地址，例如 192.168.1.5"] },
  lost: { title: "与手机的连接中断了", tips: ["手机可能锁屏、切到后台或离开了 Wi‑Fi"] },
  other: { title: "连接失败" },
};

function errorBanner(): HTMLElement | null {
  if (!status.error || status.state === "connected") return null;
  if (status.error === "phone_closed") {
    return h("div", { class: "banner info", role: "status" }, h("span", { html: icon.infoI }), h("div", {}, "手机已断开连接"));
  }
  const e = errors[status.error] ?? errors.other;
  return h("div", { class: "banner err", role: "alert" },
    h("span", { html: icon.alert }),
    h("div", {},
      h("b", {}, e.title),
      e.tips ? h("ol", {}, ...e.tips.map((t) => h("li", {}, t))) : h("div", { class: "caption" }, status.detail),
    ));
}

// ---------------------------------------------------------------- actions

async function connect(addr: string) {
  addr = addr.trim();
  if (!addr) {
    toast("请填写手机地址");
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

async function refreshRecent() {
  recent = (await Service.Recent()) ?? [];
  render();
}

async function saveSettings(patch: Partial<Settings>) {
  if (!settings) return;
  const next = { ...settings, ...patch };
  try {
    await Service.SaveSettings(next);
    settings = next;
  } catch (err) {
    toast(`保存失败：${err}`);
    settings = await Service.Settings();
  }
  render();
}

// ---------------------------------------------------------------- views

function render() {
  const active = document.activeElement as HTMLElement | null;
  const keepFocus = active?.id;
  app.replaceChildren(...(view === "settings" ? settingsView() : status.state === "connected" ? connectedView() : connectView()));
  if (keepFocus) document.getElementById(keepFocus)?.focus();
  else if (focusAddr && view === "main" && status.state !== "connected") document.getElementById("addr")?.focus();
  focusAddr = false;
}

function header(title: string): HTMLElement {
  return h("div", { class: "row" },
    h("h1", { class: "h1" }, title), h("div", { class: "grow" }),
    h("button", { class: "iconbtn", "aria-label": "设置", title: "设置", html: icon.settings, onclick: () => openSettings() }));
}

function connectView(): Node[] {
  const busy = status.state === "connecting" || status.retrying;
  const input = h("input", {
    id: "addr", value: draftAddr, placeholder: "例如 192.168.1.5", inputmode: "decimal",
    "aria-label": "手机地址", autocomplete: "off", spellcheck: "false", disabled: busy,
    oninput: (e: Event) => { draftAddr = (e.target as HTMLInputElement).value; },
    onkeydown: (e: KeyboardEvent) => { if (e.key === "Enter") connect(draftAddr); },
  }) as HTMLInputElement;

  const statusLine = busy
    ? h("span", { class: "status", role: "status" }, h("span", { class: "dot wait pulse" }),
        status.retrying ? `正在等待手机 ${host(status.addr)}，每 3 秒重试` : `正在连接 ${host(status.addr)}…`)
    : h("span", { class: "status" }, h("span", { class: "dot" }), "未连接");

  const card = h("div", { class: "card stack" },
    statusLine,
    h("label", { class: "caption", for: "addr" }, "手机地址（手机首页「本机地址」显示的 IP）"),
    h("div", { class: "input" }, input, settings && settings.port !== 8088 ? h("span", { class: "suffix" }, `:${settings.port}`) : h("span", { class: "suffix" }, DEFAULT_PORT)),
    errorBanner(),
    busy
      ? h("button", { class: "btn ghost", onclick: () => Service.Cancel() }, "取消")
      : h("button", { class: "btn primary", html: icon.power + (status.error ? "重试" : "连接"), onclick: () => connect(draftAddr) }),
    !status.error && !busy && h("div", { class: "caption" }, "先在手机上打开蓝色手柄，选 Wi‑Fi 并点「开始连接」，再点这里的「连接」。"),
  );

  const list = recent.length > 0 && h("div", { class: "card", style: "padding:10px 14px" },
    h("div", { class: "caption", style: "margin:4px 0 4px 4px" }, "最近连接"),
    h("div", {}, ...recent.map((r) => h("div", { class: "item" },
      h("div", { class: "avatar", html: icon.phone(18) }),
      h("div", { class: "grow" },
        h("div", { class: "name" }, host(r.addr)),
        h("div", { class: "sub" }, when(r.last_at) + (r.duration_seconds > 0 ? ` · 连接了 ${duration(r.duration_seconds)}` : ""))),
      h("button", { class: "forget", "aria-label": `删除 ${host(r.addr)}`, title: "从列表删除", onclick: async () => { await Service.ForgetRecent(r.addr); refreshRecent(); } }, "✕"),
      h("button", { class: "btn ghost xs", disabled: busy, onclick: () => connect(r.addr) }, "连接"),
    ))));

  return [
    header("连接手机"),
    card,
    list || null,
    h("div", { class: "grow" }),
    h("div", { class: "caption center" }, localIP ? `本机 ${localIP} · 需与手机在同一 Wi‑Fi` : "本机没有连接局域网"),
  ].filter(Boolean) as Node[];
}

function connectedView(): Node[] {
  const paused = status.paused;
  const elapsed = status.connected_at ? Math.max(0, Math.floor((Date.now() - new Date(status.connected_at).getTime()) / 1000)) : 0;
  const heartbeatOk = stats.last_frame_ms >= 0 && stats.last_frame_ms < 3000;

  const hero = h("div", { class: "hero" },
    h("div", { class: paused ? "phone paused" : "phone", html: paused ? icon.pauseBig : icon.phone(26) }),
    h("div", { class: "grow" },
      paused
        ? h("span", { class: "status", style: "color:var(--action)" }, h("span", { class: "dot wait" }), "已暂停接收")
        : h("span", { class: "status", style: "color:#7DD3AC" }, h("span", { class: "dot ok" }), "已连接"),
      h("div", { class: "addr" }, host(status.addr)),
      h("div", { class: "sub", id: "elapsed" }, paused ? "连接保持中，手机上的操作不会生效" : `已连接 ${duration(elapsed)}`)));

  const card = h("div", { class: "card" },
    hero,
    !paused && h("div", { class: "stats" },
      h("div", { class: "stat" }, h("b", {}, String(stats.keys)), h("span", { class: "caption" }, "按键")),
      h("div", { class: "stat" }, h("b", {}, String(stats.texts)), h("span", { class: "caption" }, "段文字")),
      h("div", { class: heartbeatOk ? "stat" : "stat bad" },
        h("b", {}, heartbeatOk ? "正常" : `${Math.round(stats.last_frame_ms / 1000)}s`),
        h("span", { class: "caption" }, "心跳"))),
    h("div", { class: "row", style: "margin-top:14px;gap:8px" },
      paused
        ? h("button", { class: "btn primary sm", html: icon.play + "继续接收", onclick: () => Service.SetPaused(false) })
        : h("button", { class: "btn warn sm", html: icon.pause + "暂停接收", onclick: () => Service.SetPaused(true) }),
      h("button", { class: "btn ghost sm", onclick: () => Service.Disconnect() }, "断开")));

  const items = [...activities].reverse();
  const log = h("div", { class: "card log" },
    h("div", { class: "head" },
      h("span", { class: "caption" }, "最近收到"),
      items.length > 0 && h("button", { class: "linkbtn", onclick: async () => { await Service.ClearActivities(); activities = []; render(); } }, "清空")),
    h("div", { class: "list", role: "log", "aria-live": "polite" },
      ...(items.length ? items.map(activityRow) : [h("div", { class: "empty" }, "在手机上按键或输入文字，这里会显示收到的内容")])));

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
  const t = h("time", {}, clock(a.at));
  let body: Node[];
  switch (a.kind) {
    case "combo":
      body = [h("span", { class: "tag key" }, "组合"), ...keys(a.keys)];
      break;
    case "hold":
      body = [h("span", { class: "tag key" }, "按住"), ...keys(a.keys), h("span", { class: "caption" }, `${((a.hold_ms ?? 0) / 1000).toFixed(1)} 秒`)];
      break;
    case "text":
      body = [h("span", { class: "tag txt" }, "文字"),
        h("span", { class: "txt" }, a.text ? a.text : a.chars ? `${a.chars} 个字` : "（只按回车）"),
        a.enter ? h("span", { class: "kbd" }, "↵") : null].filter(Boolean) as Node[];
      break;
    case "lock":
      body = [h("span", { class: "tag sys" }, "锁屏"), h("span", { class: "txt" }, "锁定电脑")];
      break;
    case "pause":
      body = [h("span", { class: "tag sys" }, "暂停"), h("span", { class: "txt" }, "在电脑上暂停接收")];
      break;
    case "resume":
      body = [h("span", { class: "tag sys" }, "继续"), h("span", { class: "txt" }, "恢复接收")];
      break;
    default:
      body = [h("span", { class: "tag key" }, "按键"), ...keys(a.keys)];
  }
  if (!a.applied) body.push(h("span", { class: "caption" }, "已忽略"));
  return h("div", { class: a.applied ? "ev" : "ev dropped" }, t, ...body);
}

function toggle(id: string, label: string, sub: string | null, checked: boolean, key: keyof Settings): HTMLElement {
  return h("div", { class: "set" },
    h("label", { for: id }, label, sub && h("div", { class: "sub" }, sub)),
    h("input", { type: "checkbox", role: "switch", class: "sw", id, checked, onchange: (e: Event) => saveSettings({ [key]: (e.target as HTMLInputElement).checked }) }));
}

function settingsView(): Node[] {
  if (!settings) return [];
  const s = settings;
  return [
    h("div", { class: "row" },
      h("button", { class: "iconbtn", "aria-label": "返回", html: icon.back, onclick: () => { view = "main"; render(); } }),
      h("h1", { class: "h1" }, "设置")),
    h("div", { class: "section" }, "连接"),
    h("div", { class: "card", style: "padding:6px 16px" },
      h("div", { class: "set" },
        h("label", { for: "port" }, "端口", h("div", { class: "sub" }, "与手机设置里的端口一致，默认 8088")),
        h("input", {
          class: "port", id: "port", type: "number", min: "1", max: "65535", value: String(s.port),
          onchange: (e: Event) => {
            const v = Number((e.target as HTMLInputElement).value);
            if (Number.isInteger(v) && v > 0 && v <= 65535) saveSettings({ port: v });
            else { toast("端口需在 1–65535 之间"); render(); }
          },
        })),
      toggle("auto-connect", "启动时自动连接上次的手机", "手机未在等待时每 3 秒重试", s.auto_connect, "auto_connect"),
      toggle("auto-reconnect", "意外断开后自动重连", null, s.auto_reconnect, "auto_reconnect")),
    h("div", { class: "section" }, "窗口"),
    h("div", { class: "card", style: "padding:6px 16px" },
      toggle("start-on-boot", "开机自动启动", "启动后只在托盘显示", s.start_on_boot, "start_on_boot"),
      toggle("close-to-tray", "关闭窗口时缩到托盘", null, s.close_to_tray, "close_to_tray"),
      toggle("show-text", "最近收到里显示文字内容", "关闭后只显示字数", s.show_text, "show_text")),
    info.elevation_needed && !info.elevated && h("div", { class: "banner info" },
      h("span", { html: icon.infoI }),
      h("div", {}, "要控制以管理员身份运行的程序（部分游戏、任务管理器），蓝色手柄也需要以管理员身份运行。",
        h("a", { role: "button", tabindex: "0", onclick: restartAsAdmin, onkeydown: (e: KeyboardEvent) => { if (e.key === "Enter") restartAsAdmin(); } }, "以管理员身份重启"))),
    h("div", { class: "grow" }),
    h("div", { class: "caption center" }, `版本 ${info.version}${info.elevated ? " · 管理员" : ""}`),
  ].filter(Boolean) as Node[];
}

async function restartAsAdmin() {
  try {
    await Service.RestartAsAdmin();
  } catch {
    toast("没有以管理员身份重启");
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
  draftAddr = host(await Service.LastAddr());
  focusAddr = true;
  render();
}

init();
