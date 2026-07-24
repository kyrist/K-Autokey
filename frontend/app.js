/**
 * K-Autokey 前端：通过 Wails Bind 调用 Go App，EventsOn("running") 同步启停状态。
 *
 * 能力概要：
 * - 可视化键盘多选绑定；间隔数字输入
 * - 热键捕获（组合键）；开启连发时锁定热键按钮
 * - 进程绑定弹窗；主界面只显示绑定摘要
 * - 「后台」→ HideToTray
 *
 * 协议细节见 docs/PROTOCOL.md。
 */

/**
 * u: 键宽（以 1u 标准键为单位）
 * { gap: n }: 空隙（n 为 u）
 */
const KEYBOARD_ROWS = [
  [
    { label: "Esc", text: "Esc", u: 1, cls: "mod" },
    { gap: 0.7 },
    { label: "F1", u: 1, cls: "f-key" },
    { label: "F2", u: 1, cls: "f-key" },
    { label: "F3", u: 1, cls: "f-key" },
    { label: "F4", u: 1, cls: "f-key" },
    { gap: 0.7 },
    { label: "F5", u: 1, cls: "f-key" },
    { label: "F6", u: 1, cls: "f-key" },
    { label: "F7", u: 1, cls: "f-key" },
    { label: "F8", u: 1, cls: "f-key" },
    { gap: 0.7 },
    { label: "F9", u: 1, cls: "f-key" },
    { label: "F10", u: 1, cls: "f-key" },
    { label: "F11", u: 1, cls: "f-key" },
    { label: "F12", u: 1, cls: "f-key" },
  ],
  [
    { label: "`", text: "`~", u: 1 },
    { label: "1", u: 1 },
    { label: "2", u: 1 },
    { label: "3", u: 1 },
    { label: "4", u: 1 },
    { label: "5", u: 1 },
    { label: "6", u: 1 },
    { label: "7", u: 1 },
    { label: "8", u: 1 },
    { label: "9", u: 1 },
    { label: "0", u: 1 },
    { label: "-", text: "-_", u: 1 },
    { label: "=", text: "=+", u: 1 },
    { label: "Back", text: "Backspace", u: 2, cls: "mod" },
  ],
  [
    { label: "Tab", text: "Tab", u: 1.5, cls: "mod" },
    { label: "Q", u: 1 },
    { label: "W", u: 1 },
    { label: "E", u: 1 },
    { label: "R", u: 1 },
    { label: "T", u: 1 },
    { label: "Y", u: 1 },
    { label: "U", u: 1 },
    { label: "I", u: 1 },
    { label: "O", u: 1 },
    { label: "P", u: 1 },
    { label: "[", text: "[{", u: 1 },
    { label: "]", text: "]}", u: 1 },
    { label: "\\", text: "\\|", u: 1.5, cls: "mod" },
  ],
  [
    { label: "Caps", text: "Caps", u: 1.75, cls: "mod" },
    { label: "A", u: 1 },
    { label: "S", u: 1 },
    { label: "D", u: 1 },
    { label: "F", u: 1 },
    { label: "G", u: 1 },
    { label: "H", u: 1 },
    { label: "J", u: 1 },
    { label: "K", u: 1 },
    { label: "L", u: 1 },
    { label: ";", text: ";:", u: 1 },
    { label: "'", text: "'\"", u: 1 },
    { label: "Enter", text: "Enter", u: 2.25, cls: "mod" },
  ],
  [
    { label: "LShift", text: "L-Shift", u: 2.25, cls: "mod" },
    { label: "Z", u: 1 },
    { label: "X", u: 1 },
    { label: "C", u: 1 },
    { label: "V", u: 1 },
    { label: "B", u: 1 },
    { label: "N", u: 1 },
    { label: "M", u: 1 },
    { label: ",", text: ",<", u: 1 },
    { label: ".", text: ".>", u: 1 },
    { label: "/", text: "/?", u: 1 },
    { label: "RShift", text: "R-Shift", u: 2.75, cls: "mod" },
  ],
  [
    { label: "LCtrl", text: "L-Ctrl", u: 1.5, cls: "mod" },
    { label: "LAlt", text: "L-Alt", u: 1.5, cls: "mod" },
    { label: "Space", text: "Space", u: 9 },
    { label: "RAlt", text: "R-Alt", u: 1.5, cls: "mod" },
    { label: "RCtrl", text: "R-Ctrl", u: 1.5, cls: "mod" },
  ],
];

const CODE_TO_KEY = {
  Escape: "esc",
  Space: "space",
  Backspace: "back",
  Tab: "tab",
  Enter: "enter",
  CapsLock: "caps",
  Backquote: "`",
  Minus: "-",
  Equal: "=",
  BracketLeft: "[",
  BracketRight: "]",
  Backslash: "\\",
  Semicolon: ";",
  Quote: "'",
  Comma: ",",
  Period: ".",
  Slash: "/",
  IntlBackslash: "\\",
};

const MOD_CODES = new Set([
  "ControlLeft",
  "ControlRight",
  "ShiftLeft",
  "ShiftRight",
  "AltLeft",
  "AltRight",
  "MetaLeft",
  "MetaRight",
  "OSLeft",
  "OSRight",
]);

const state = {
  keyLabels: ["Space"],
  intervalMs: 50,
  enableHotkey: "f6",
  emergencyHotkey: "f8",
  boundProcess: "",
  keyChoices: [],
  processes: [],
  enabled: false,
  capturing: null, // "enable" | "emergency" | null
  captureMods: new Set(),
  captureBusy: false,
};

let configQueue = Promise.resolve();
let processListSeq = 0;

function $(id) {
  return document.getElementById(id);
}

function setStatus(text, enabled = false) {
  const el = $("status");
  el.textContent = text;
  el.classList.toggle("running", !!enabled);
}

function waitApi(timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const timer = setInterval(() => {
      const api = window.go && window.go.main && window.go.main.App;
      if (api) {
        clearInterval(timer);
        resolve(api);
        return;
      }
      if (Date.now() - start > timeoutMs) {
        clearInterval(timer);
        reject(new Error("Go API 未就绪"));
      }
    }, 40);
  });
}

function codeToKeyToken(code) {
  if (CODE_TO_KEY[code]) return CODE_TO_KEY[code];
  if (/^F([1-9]|1[0-2])$/.test(code)) return code.toLowerCase();
  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase();
  if (/^Digit[0-9]$/.test(code)) return code.slice(5);
  if (/^Numpad[0-9]$/.test(code)) return code.slice(6);
  return "";
}

function pickMod(mods, left, right, either) {
  const hasL = mods.has(left);
  const hasR = mods.has(right);
  if (hasL && !hasR) {
    if (left === "ControlLeft") return "lctrl";
    if (left === "ShiftLeft") return "lshift";
    if (left === "AltLeft") return "lalt";
  }
  if (hasR && !hasL) {
    if (right === "ControlRight") return "rctrl";
    if (right === "ShiftRight") return "rshift";
    if (right === "AltRight") return "ralt";
  }
  if (hasL || hasR) return either;
  return "";
}

function modsFromEvent(e, tracked) {
  const mods = new Set(tracked);
  if (e.ctrlKey) {
    if (!mods.has("ControlLeft") && !mods.has("ControlRight")) mods.add("ControlLeft");
  }
  if (e.shiftKey) {
    if (!mods.has("ShiftLeft") && !mods.has("ShiftRight")) mods.add("ShiftLeft");
  }
  if (e.altKey) {
    if (!mods.has("AltLeft") && !mods.has("AltRight")) mods.add("AltLeft");
  }
  if (e.metaKey) {
    if (!mods.has("MetaLeft") && !mods.has("MetaRight") && !mods.has("OSLeft") && !mods.has("OSRight")) {
      mods.add("MetaLeft");
    }
  }
  return mods;
}

function eventToHotkey(e, trackedMods) {
  const token = codeToKeyToken(e.code);
  if (!token || MOD_CODES.has(e.code)) return "";

  const mods = modsFromEvent(e, trackedMods);
  const parts = [];
  const ctrl = pickMod(mods, "ControlLeft", "ControlRight", "ctrl");
  const shift = pickMod(mods, "ShiftLeft", "ShiftRight", "shift");
  const alt = pickMod(mods, "AltLeft", "AltRight", "alt");

  let win = "";
  const hasLWin = mods.has("MetaLeft") || mods.has("OSLeft");
  const hasRWin = mods.has("MetaRight") || mods.has("OSRight");
  if (hasLWin && !hasRWin) win = "lwin";
  else if (hasRWin && !hasLWin) win = "rwin";
  else if (hasLWin || hasRWin) win = "win";

  if (ctrl) parts.push(ctrl);
  if (shift) parts.push(shift);
  if (alt) parts.push(alt);
  if (win) parts.push(win);
  parts.push(token);
  return parts.join("+");
}

function formatHotkeyDisplay(raw) {
  if (!raw) return "";
  return String(raw)
    .split("+")
    .map((p) => {
      const x = p.trim().toLowerCase();
      const map = {
        ctrl: "Ctrl",
        lctrl: "LCtrl",
        rctrl: "RCtrl",
        shift: "Shift",
        lshift: "LShift",
        rshift: "RShift",
        alt: "Alt",
        lalt: "LAlt",
        ralt: "RAlt",
        win: "Win",
        lwin: "LWin",
        rwin: "RWin",
        esc: "Esc",
        space: "Space",
        back: "Backspace",
        tab: "Tab",
        enter: "Enter",
        caps: "Caps",
      };
      if (map[x]) return map[x];
      if (/^f([1-9]|1[0-2])$/.test(x)) return x.toUpperCase();
      if (/^[a-z]$/.test(x)) return x.toUpperCase();
      return p;
    })
    .join("+");
}

function syncHotkeyButtons() {
  const en = $("enable-hotkey");
  const em = $("emergency-hotkey");
  const locked = !!state.enabled;
  const enText = formatHotkeyDisplay(state.enableHotkey) || "F6";
  const emText = formatHotkeyDisplay(state.emergencyHotkey) || "F8";

  if (state.capturing === "enable") {
    en.textContent = "按下组合键…";
    en.classList.add("listening");
    en.title = "Esc 取消";
  } else {
    en.textContent = enText;
    en.classList.remove("listening");
    en.title = locked ? "开启连发时不可修改热键" : enText;
  }
  if (state.capturing === "emergency") {
    em.textContent = "按下组合键…";
    em.classList.add("listening");
    em.title = "Esc 取消";
  } else {
    em.textContent = emText;
    em.classList.remove("listening");
    em.title = locked ? "开启连发时不可修改热键" : emText;
  }

  en.disabled = locked;
  em.disabled = locked;
}

async function setCapturing(which) {
  if (state.enabled && which) {
    setStatus("开启连发时不可修改热键", true);
    return;
  }
  if (state.captureBusy) return;

  const next = which && state.capturing === which ? null : which;
  const prev = state.capturing;
  state.capturing = next;
  state.captureMods = new Set();
  syncHotkeyButtons();

  try {
    const api = await waitApi();
    const result = await api.SetHotkeyListening(!!next);
    if (result && result.ok === false) {
      state.capturing = prev;
      state.captureMods = new Set();
      syncHotkeyButtons();
      setStatus(result.message || "无法捕获热键", state.enabled);
    }
  } catch (err) {
    state.capturing = prev;
    state.captureMods = new Set();
    syncHotkeyButtons();
    setStatus("无法捕获热键 — " + err, state.enabled);
  }
}

async function finishCapture(hotkey) {
  const which = state.capturing;
  if (!which || state.captureBusy) return;

  // 立刻退出捕获态，避免按键重复触发重入
  state.captureBusy = true;
  state.capturing = null;
  state.captureMods = new Set();
  syncHotkeyButtons();

  try {
    if (!hotkey) {
      try {
        const api = await waitApi();
        await api.SetHotkeyListening(false);
      } catch (_) {
        /* ignore */
      }
      return;
    }
    const other = which === "enable" ? state.emergencyHotkey : state.enableHotkey;
    if (hotkey.toLowerCase() === String(other).toLowerCase()) {
      setStatus("两个热键不能相同", state.enabled);
      try {
        const api = await waitApi();
        await api.SetHotkeyListening(false);
      } catch (_) {
        /* ignore */
      }
      return;
    }
    if (which === "enable") state.enableHotkey = hotkey;
    else state.emergencyHotkey = hotkey;
    // 先写入配置（后端 listening 仍为 true），再结束监听并同步边沿
    await pushConfig();
    try {
      const api = await waitApi();
      await api.SetHotkeyListening(false);
    } catch (_) {
      /* ignore */
    }
    syncHotkeyButtons();
    if (!state.enabled) setStatus(idleStatus(), false);
  } finally {
    state.captureBusy = false;
  }
}

function allowedSet() {
  return new Set(state.keyChoices.length ? state.keyChoices : flattenLabels());
}

function flattenLabels() {
  const out = [];
  const seen = new Set();
  KEYBOARD_ROWS.forEach((row) => {
    row.forEach((item) => {
      if (item && item.label && !seen.has(item.label)) {
        seen.add(item.label);
        out.push(item.label);
      }
    });
  });
  return out;
}

function uClass(u) {
  const map = {
    1: "u1",
    1.5: "u15",
    1.75: "u175",
    2: "u2",
    2.25: "u225",
    2.75: "u275",
    9: "u9",
  };
  return map[u] || "u1";
}

function gapClass(u) {
  const map = {
    0.7: "g07",
    2.75: "g275",
  };
  return map[u] || "g1";
}

function renderKeys() {
  const root = $("keyboard");
  const allowed = allowedSet();
  root.innerHTML = KEYBOARD_ROWS.map((row) => {
    const keys = row
      .map((item) => {
        if (!item) return "";
        if (item.gap != null) {
          return `<span class="kb-gap ${gapClass(item.gap)}" aria-hidden="true"></span>`;
        }
        if (!allowed.has(item.label)) return "";
        const active = state.keyLabels.includes(item.label) ? "active" : "";
        const cls = ["kb-key", uClass(item.u || 1), item.cls || "", active].filter(Boolean).join(" ");
        const text = item.text || item.label;
        return `<button type="button" class="${cls}" data-key="${item.label}">${text}</button>`;
      })
      .join("");
    return `<div class="kb-row">${keys}</div>`;
  }).join("");

  $("selected-keys").textContent =
    "已选：" + (state.keyLabels.length ? state.keyLabels.join("、") : "无");
}

function collectConfig() {
  return {
    key_labels: state.keyLabels.length ? state.keyLabels : ["Space"],
    interval_ms: Number(state.intervalMs) || 50,
    enable_hotkey: state.enableHotkey || "f6",
    emergency_hotkey: state.emergencyHotkey || "f8",
    bound_process: state.boundProcess || "",
  };
}

function pushConfig() {
  const payload = collectConfig();
  configQueue = configQueue
    .catch(() => {})
    .then(async () => {
      const api = await waitApi();
      const result = await api.Configure(payload);
      if (result && typeof result.enabled === "boolean" && result.enabled !== state.enabled) {
        onEnabledChanged(result.enabled);
      }
      return result;
    });
  return configQueue;
}

function normalizeKeyLabel(label) {
  switch (label) {
    case "Shift":
      return "LShift";
    case "Ctrl":
      return "LCtrl";
    case "Alt":
      return "LAlt";
    default:
      return label;
  }
}

function normalizeKeyLabels(labels) {
  const out = [];
  const seen = new Set();
  (labels || []).forEach((raw) => {
    const label = normalizeKeyLabel(String(raw || "").trim());
    if (!label || seen.has(label)) return;
    seen.add(label);
    out.push(label);
  });
  return out.length ? out : ["Space"];
}

function idleStatus() {
  if (state.boundProcess) {
    return `已绑定 ${state.boundProcess} — 切到该窗口自动连发`;
  }
  return `未开启 — 点「开启」或按 ${formatHotkeyDisplay(state.enableHotkey)}`;
}

function runningStatus() {
  if (state.boundProcess) {
    return `连发中（${state.boundProcess} 前台）`;
  }
  return "已开启 — 按住已选键即可连发";
}

function syncProcessInfo() {
  const el = $("process-info");
  if (!el) return;
  if (state.boundProcess) {
    const text = `进程：${state.boundProcess}（前台自动连发）`;
    el.textContent = text;
    el.title = text;
    el.classList.add("is-bound");
  } else {
    el.textContent = "进程：未绑定";
    el.title = "未绑定进程时，请用按钮或热键手动开启连发";
    el.classList.remove("is-bound");
  }
}

function escapeAttr(s) {
  return String(s).replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function renderProcessList(list) {
  const root = $("process-list");
  const items = Array.isArray(list) ? list : [];
  state.processes = items;
  const cur = (state.boundProcess || "").toLowerCase();

  if (!items.length) {
    root.innerHTML = `<div class="process-empty">暂无可用进程，请点击「刷新列表」</div>`;
    return;
  }

  root.innerHTML = items
    .map((p) => {
      const name = p.name || "";
      const title = p.title || "";
      const pid = p.pid != null ? p.pid : "";
      const active = name.toLowerCase() === cur ? "active" : "";
      return `<button type="button" class="process-item ${active}" data-process="${escapeAttr(name)}">
        <span class="process-item-meta">
          <div class="process-item-name">${escapeHtml(name)}</div>
          <div class="process-item-title">${escapeHtml(title)}</div>
        </span>
        <span class="process-item-pid">PID ${escapeHtml(String(pid))}</span>
      </button>`;
    })
    .join("");
}

async function refreshProcesses() {
  const seq = ++processListSeq;
  const api = await waitApi();
  const list = await api.ListProcesses();
  if (seq !== processListSeq) return;
  renderProcessList(list);
}

async function setBoundProcess(name) {
  state.boundProcess = (name || "").toLowerCase();
  syncProcessInfo();
  if (!state.enabled) setStatus(idleStatus(), false);
  const result = await pushConfig();
  if (result && typeof result.enabled === "boolean") {
    onEnabledChanged(result.enabled);
  }
}

function openProcessModal() {
  const modal = $("process-modal");
  modal.hidden = false;
  refreshProcesses().catch((err) => {
    $("process-list").innerHTML = `<div class="process-empty">加载失败：${escapeHtml(String(err))}</div>`;
  });
}

function closeProcessModal() {
  $("process-modal").hidden = true;
}

function applyBootstrap(data) {
  const cfg = data.config || {};
  state.keyChoices = data.key_choices || [];
  state.keyLabels = normalizeKeyLabels(cfg.key_labels);
  state.intervalMs = cfg.interval_ms || 50;
  state.enableHotkey = cfg.enable_hotkey || cfg.hold_hotkey || "f6";
  state.emergencyHotkey = cfg.emergency_hotkey || "f8";
  state.boundProcess = (cfg.bound_process || "").toLowerCase();
  state.enabled = !!data.enabled;

  $("interval-input").value = String(state.intervalMs);
  state.processes = data.processes || [];
  syncProcessInfo();
  syncHotkeyButtons();
  renderKeys();
  syncStartButton();
  setStatus(data.status || (state.enabled ? runningStatus() : idleStatus()), state.enabled);
}

function syncStartButton() {
  $("btn-start").disabled = state.enabled;
  $("btn-start").textContent = state.enabled ? "已开" : "开启";
}

async function onEnabledChanged(enabled) {
  state.enabled = !!enabled;
  if (state.enabled && state.capturing) {
    await setCapturing(null);
  } else {
    syncHotkeyButtons();
  }
  syncStartButton();
  setStatus(state.enabled ? runningStatus() : idleStatus(), state.enabled);
}

function onCaptureKeyDown(e) {
  if (!state.capturing || state.captureBusy) return;
  e.preventDefault();
  e.stopPropagation();

  if (e.code === "Escape") {
    finishCapture("");
    return;
  }
  if (MOD_CODES.has(e.code)) {
    state.captureMods.add(e.code);
    return;
  }

  const hotkey = eventToHotkey(e, state.captureMods);
  if (!hotkey) return;
  finishCapture(hotkey);
}

function onCaptureKeyUp(e) {
  if (!state.capturing) return;
  if (MOD_CODES.has(e.code)) {
    state.captureMods.delete(e.code);
  }
}

function bindEvents() {
  $("keyboard").addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-key]");
    if (!btn) return;
    const label = btn.dataset.key;
    if (state.keyLabels.includes(label)) {
      if (state.keyLabels.length <= 1) {
        setStatus("至少保留一个连发按键", state.enabled);
        return;
      }
      state.keyLabels = state.keyLabels.filter((x) => x !== label);
    } else {
      state.keyLabels = [...state.keyLabels, label];
    }
    renderKeys();
    await pushConfig();
  });

  const syncInterval = async (value) => {
    let ms = Number(value);
    if (!Number.isFinite(ms)) ms = 50;
    ms = Math.max(1, Math.min(10000, Math.round(ms)));
    state.intervalMs = ms;
    $("interval-input").value = String(ms);
    await pushConfig();
  };

  $("interval-input").addEventListener("change", (e) => syncInterval(e.target.value));
  $("interval-input").addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      syncInterval(e.target.value);
    }
  });
  $("enable-hotkey").addEventListener("click", () => setCapturing("enable"));
  $("emergency-hotkey").addEventListener("click", () => setCapturing("emergency"));
  window.addEventListener("keydown", onCaptureKeyDown, true);
  window.addEventListener("keyup", onCaptureKeyUp, true);

  $("btn-open-process").addEventListener("click", () => openProcessModal());
  $("process-modal").addEventListener("click", (e) => {
    if (e.target.closest("[data-close-modal]")) {
      closeProcessModal();
    }
  });
  $("btn-refresh-process").addEventListener("click", async () => {
    try {
      await refreshProcesses();
    } catch (err) {
      $("process-list").innerHTML = `<div class="process-empty">刷新失败：${escapeHtml(String(err))}</div>`;
    }
  });
  $("btn-clear-process").addEventListener("click", async () => {
    await setBoundProcess("");
    renderProcessList(state.processes);
    closeProcessModal();
  });
  $("process-list").addEventListener("click", async (e) => {
    const item = e.target.closest("[data-process]");
    if (!item) return;
    await setBoundProcess(item.dataset.process || "");
    closeProcessModal();
  });
  window.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !$("process-modal").hidden && !state.capturing && !state.captureBusy) {
      closeProcessModal();
    }
  });

  $("btn-start").addEventListener("click", async () => {
    try {
      await pushConfig();
      const api = await waitApi();
      const result = await api.Start();
      if (result && result.message) setStatus(result.message, !!result.enabled);
      if (result && typeof result.enabled === "boolean") {
        await onEnabledChanged(result.enabled);
        if (result.message) setStatus(result.message, result.enabled);
      }
    } catch (err) {
      setStatus("开启失败 — " + err);
    }
  });

  $("btn-stop").addEventListener("click", async () => {
    try {
      const api = await waitApi();
      const result = await api.Stop();
      if (result && typeof result.enabled === "boolean") {
        await onEnabledChanged(result.enabled);
      }
      if (result && result.message) setStatus(result.message, false);
    } catch (err) {
      setStatus("关闭失败 — " + err);
    }
  });

  $("btn-save").addEventListener("click", async () => {
    try {
      await pushConfig();
      const api = await waitApi();
      const result = await api.SaveConfig(collectConfig());
      if (result && typeof result.enabled === "boolean" && result.enabled !== state.enabled) {
        await onEnabledChanged(result.enabled);
      }
      setStatus((result && result.message) || "配置已保存", state.enabled);
    } catch (err) {
      setStatus("保存失败 — " + err);
    }
  });

  $("btn-tray").addEventListener("click", async () => {
    try {
      const api = await waitApi();
      await api.HideToTray();
    } catch (err) {
      setStatus("切换后台失败 — " + err, state.enabled);
    }
  });
}

async function boot() {
  if (window.__kAutokeyBooted) return;
  window.__kAutokeyBooted = true;
  bindEvents();
  try {
    const api = await waitApi();
    // 先订阅事件，再推配置，避免自动启停事件丢失
    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn("running", onEnabledChanged);
    }
    const data = await api.GetBootstrap();
    applyBootstrap(data);
    await pushConfig();
    const again = await api.GetBootstrap();
    if (!!again.enabled !== state.enabled) {
      await onEnabledChanged(!!again.enabled);
    } else if (again.status) {
      setStatus(again.status, !!again.enabled);
    }
  } catch (err) {
    setStatus("初始化失败 — " + err);
  }
}

document.addEventListener("DOMContentLoaded", boot);
window.addEventListener("load", boot);
setTimeout(boot, 300);
