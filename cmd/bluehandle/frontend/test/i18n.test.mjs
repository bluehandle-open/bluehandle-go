import assert from "node:assert/strict";
import { beforeEach, test } from "node:test";
import { keyLabel, setLocale } from "../src/i18n.ts";

globalThis.document = { documentElement: { lang: "" } };

beforeEach(() => setLocale("zh-CN"));

test("English activity logs translate special keys, media keys and mouse buttons", () => {
  setLocale("en");
  const labels = ["退格", "空格", "菜单键", "静音", "音量-", "音量+",
    "下一曲", "上一曲", "停止", "播放/暂停", "鼠标左键", "鼠标右键", "鼠标中键"];
  assert.deepEqual(labels.map(keyLabel), ["Backspace", "Space", "Menu", "Mute", "Volume −", "Volume +",
    "Next track", "Previous track", "Stop", "Play / Pause", "Left mouse button", "Right mouse button", "Middle mouse button"]);
});

test("all ten numpad digits are localized", () => {
  setLocale("en");
  for (let digit = 0; digit <= 9; digit++) {
    assert.equal(keyLabel(`小键盘${digit}`), `Numpad ${digit}`);
  }
});

test("changing languages updates existing records without modifying their keys", () => {
  const existing = { keys: ["Ctrl", "退格", "小键盘1"] };
  assert.deepEqual(existing.keys.map(keyLabel), ["Ctrl", "退格", "小键盘1"]);
  setLocale("en");
  assert.deepEqual(existing.keys.map(keyLabel), ["Ctrl", "Backspace", "Numpad 1"]);
  const incoming = { keys: ["空格", "鼠标左键"] };
  assert.deepEqual(incoming.keys.map(keyLabel), ["Space", "Left mouse button"]);
  setLocale("zh-CN");
  assert.deepEqual(existing.keys.map(keyLabel), ["Ctrl", "退格", "小键盘1"]);
  assert.deepEqual(incoming.keys.map(keyLabel), ["空格", "鼠标左键"]);
});

test("unsupported or missing languages fall back to Chinese", () => {
  for (const language of [undefined, "", "fr"]) {
    setLocale("en");
    setLocale(language);
    assert.equal(keyLabel("退格"), "退格");
  }
});

test("universal key names and unknown labels pass through unchanged", () => {
  setLocale("en");
  for (const label of ["A", "1", "F12", "Ctrl", "←", "+", "VK_0xFF", "constructor", "小键盘10", ""]) {
    assert.equal(keyLabel(label), label);
  }
});
