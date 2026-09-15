// polyfill.js
(function () {
  // --- 定义 GM_info ---
  window.GM_info = {
    script: {
      name: "Polyfill Script",
      version: "1.0.0",
      namespace: "https://example.com/",
      description: "Simulated Greasemonkey Environment",
      author: "Developer",
      includes: [],
      excludes: [],
      matches: ["*://*/*"],
      resources: [],
      unwrap: false,
    },
    scriptHandler: "Tampermonkey", // 模拟脚本管理器名称
    version: "4.18.0", // 模拟脚本管理器的版本
    injectInto: "page",
    platform: {
      browserName: "Chrome",
      browserVersion: "120.0.0",
      os: "Windows",
    },
  };
  // --- 2. 同步 API (GM_*) - IndexedDB 版 ---

  // 配置常量
  const DB_NAME = "GM_Polyfill_DB";
  const STORE_NAME = "GM_Values";
  const BROADCAST_CHANNEL_NAME = "GM_Polyfill_Sync";

  // 内存缓存：保证 GM_getValue 的同步调用
  const valueCache = new Map();
  // 监听器存储
  const listeners = new Map(); // (假设外部已有 listeners 定义，如果没有请取消注释)

  // 跨标签页通讯通道
  const syncChannel = new BroadcastChannel(BROADCAST_CHANNEL_NAME);


  /**
   * 触发值改变监听器
   * @param {string} key - 被修改的键名
   * @param {any} value - 新值
   * @param {any} oldValue - 旧值
   * @param {boolean} remote - 是否来自其他标签页 (true) 还是当前页面 (false)
   */
  function triggerListeners(key, value, oldValue, remote) {
    // 检查是否有该 key 的监听器
    const list = listeners.get(key);
    if (list && list.length > 0) {
      list.forEach((item) => {
        try {
          // 按照 Tampermonkey 标准接口回调: (name, old_value, new_value, remote)
          item.callback(key, oldValue, value, remote);
        } catch (e) {
          console.error(`GM_Polyfill: 监听器回调执行出错 [Key: ${key}]`, e);
        }
      });
    }
  }

  /**
   * 初始化 IndexedDB 并预加载数据到内存缓存
   */
  (function initIndexedDB() {
    const request = indexedDB.open(DB_NAME, 1);

    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME);
      }
    };

    request.onsuccess = (event) => {
      const db = event.target.result;
      const transaction = db.transaction(STORE_NAME, "readonly");
      const store = transaction.objectStore(STORE_NAME);

      // 遍历所有数据存入缓存
      const cursorRequest = store.openCursor();
      cursorRequest.onsuccess = (e) => {
        const cursor = e.target.result;
        if (cursor) {
          valueCache.set(cursor.key, cursor.value);
          cursor.continue();
        } else {
          console.log("GM_Polyfill: 数据已从 IndexedDB 加载完毕");
        }
      };
    };

    request.onerror = (e) => {
      console.error("GM_Polyfill: IndexedDB 打开失败", e);
    };
  })();

  /**
   * 辅助函数：异步写入 IndexedDB
   */
  function dbSave(key, value) {
    const request = indexedDB.open(DB_NAME, 1);
    request.onsuccess = (e) => {
      const db = e.target.result;
      const tx = db.transaction(STORE_NAME, "readwrite");
      tx.objectStore(STORE_NAME).put(value, key);
    };
  }

  /**
   * 辅助函数：异步删除 IndexedDB
   */
  function dbDelete(key) {
    const request = indexedDB.open(DB_NAME, 1);
    request.onsuccess = (e) => {
      const db = e.target.result;
      const tx = db.transaction(STORE_NAME, "readwrite");
      tx.objectStore(STORE_NAME).delete(key);
    };
  }

  // --- 接收跨标签页的更新通知 ---
  syncChannel.onmessage = (event) => {
    const { type, key, value, oldValue } = event.data;
    if (type === "set") {
      valueCache.set(key, value);
      triggerListeners(key, value, oldValue, true); // true 表示来自远程(其他标签页)
    } else if (type === "delete") {
      valueCache.delete(key);
      triggerListeners(key, undefined, oldValue, true);
    }
  };

  // --- API 实现 ---

  window.GM_getValue = function (key, defaultValue) {
    // 直接从内存缓存读取，保证同步返回
    if (valueCache.has(key)) {
      const val = valueCache.get(key);
      // 如果存的是 JSON 字符串形式的对象，这里根据需要决定是否 parse
      // 但 IndexedDB 可以存对象，建议存取保持原样。
      // 为兼容旧逻辑，如果你存进去的是 string，这就返回 string。
      return val;
    }
    return defaultValue;
  };

  window.GM_setValue = function (key, value) {
    const oldValue = valueCache.get(key);

    // 1. 更新内存缓存
    valueCache.set(key, value);

    // 2. 异步持久化到 IndexedDB
    dbSave(key, value);

    // 3. 触发本页面的监听器
    // 注意：IndexedDB 能够直接存储对象，所以这里传给监听器的就是原值，不需要 JSON.stringify
    // 为了兼容旧代码的监听器逻辑（旧代码可能期待字符串），你可能需要调整监听器回调，
    // 但通常直接传对象更符合现代标准。
    triggerListeners(key, value, oldValue, false);

    // 4. 通知其他标签页
    syncChannel.postMessage({
      type: "set",
      key: key,
      value: value,
      oldValue: oldValue
    });
  };

  window.GM_deleteValue = function (key) {
    const oldValue = valueCache.get(key);

    // 1. 更新内存
    valueCache.delete(key);

    // 2. 异步删除
    dbDelete(key);

    // 3. 触发监听器
    triggerListeners(key, undefined, oldValue, false);

    // 4. 通知其他标签页
    syncChannel.postMessage({
      type: "delete",
      key: key,
      oldValue: oldValue
    });
  };

  window.GM_listValues = function () {
    return Array.from(valueCache.keys());
  };

  window.GM_addValueChangeListener = function (key, callback) {
    if (!listeners.has(key)) listeners.set(key, []);
    const id = Math.random().toString(36).substr(2, 9);
    listeners.get(key).push({ id, callback });
    // 注意：不再需要 window.addEventListener('storage')，
    // 因为这由 BroadcastChannel (syncChannel.onmessage) 处理了。
    return id;
  };

  window.GM_removeValueChangeListener = function (id) {
    listeners.forEach((list) => {
      const idx = list.findIndex((item) => item.id === id);
      if (idx !== -1) list.splice(idx, 1);
    });
  };

  window.GM_addStyle = function (css) {
    const style = document.createElement("style");
    style.textContent = css;
    (document.head || document.documentElement).appendChild(style);
    return style;
  };

  window.GM_setClipboard = function (data, info) {
    navigator.clipboard
      .writeText(data)
      .then(() => {
        console.log("Clipboard set");
      })
      .catch((err) => {
        console.error("Clipboard error", err);
      });
  };

  window.GM_openInTab = function (url, options) {
    return window.open(url, "_blank");
  };

  window.GM_notification = function (details, ondone) {
    if (!("Notification" in window)) {
      alert(details.text || details);
      return;
    }
    const show = () => {
      const n = new Notification(details.title || "Notification", {
        body: details.text,
        icon: details.image,
      });
      if (details.timeout) setTimeout(() => n.close(), details.timeout);
      n.onclick = details.onclick || (() => window.focus());
      n.onclose = ondone;
    };
    if (Notification.permission === "granted") show();
    else if (Notification.permission !== "denied") {
      Notification.requestPermission().then((p) => {
        if (p === "granted") show();
      });
    }
  };

  window.GM_xmlhttpRequest = function (details) {
    const controller = new AbortController();

    fetch(details.url, {
      method: details.method || "GET",
      headers: details.headers,
      body: details.data,
      referrerPolicy: "no-referrer", // 核心：禁止发送 Referer
      signal: controller.signal,
      mode: "cors", // 通常需要跨域支持
    })
      .then(async (response) => {
        // 为了模拟 XHR 的行为，我们需要读取文本内容
        // 注意：如果处理二进制数据，这里可能需要改为 response.blob() 或 response.arrayBuffer()
        const text = await response.text();

        // 构造类似 XHR 的响应头字符串
        const responseHeaders = [...response.headers]
          .map(([key, value]) => `${key}: ${value}`)
          .join("\r\n");

        if (details.onload) {
          details.onload({
            status: response.status,
            statusText: response.statusText,
            readyState: 4, // 模拟 XHR 完成状态
            responseText: text,
            response: text,
            responseHeaders: responseHeaders,
            finalUrl: response.url,
          });
        }
      })
      .catch((e) => {
        // 处理 AbortError 或网络错误
        if (e.name === "AbortError") {
          if (details.onabort) details.onabort(e);
        } else {
          if (details.onerror) details.onerror(e);
        }
      });

    return { abort: () => controller.abort() };
  };

  // --- 3. 菜单命令模拟 ---
  const menuCommands = [];
  window.GM_registerMenuCommand = function (name, fn) {
    const id = Math.random().toString(36).substr(2, 9);
    menuCommands.push({ id, name, fn });
    updateMenuUI();
    return id;
  };
  window.GM_unregisterMenuCommand = function (id) {
    const idx = menuCommands.findIndex((m) => m.id === id);
    if (idx !== -1) {
      menuCommands.splice(idx, 1);
      updateMenuUI();
    }
  };

  function updateMenuUI() {
    let box = document.getElementById("gm-menu-polyfill");
    if (!box && menuCommands.length > 0) {
      box = document.createElement("div");
      box.id = "gm-menu-polyfill";
      box.setAttribute(
        "style",
        "position:fixed;top:10px;right:10px;z-index:999999;background:#fff;border:2px solid #000;padding:5px;font-family:sans-serif;font-size:12px;box-shadow:4px 4px 0 #000"
      );
      document.body.appendChild(box);
    }
    if (box) {
      if (menuCommands.length === 0) {
        box.remove();
        return;
      }
      box.innerHTML =
        '<div style="font-weight:bold;border-bottom:1px solid #000;margin-bottom:5px">GM Menu</div>';
      menuCommands.forEach((cmd) => {
        const b = document.createElement("button");
        b.textContent = cmd.name;
        b.style =
          "display:block;width:100%;background:#eee;border:1px solid #ccc;margin-top:2px;cursor:pointer;";
        b.onclick = cmd.fn;
        box.appendChild(b);
      });
    }
  }

  // --- 4. GM_info ---
  window.GM_info = {
    script: {
      name: "Polyfill Script",
      version: "1.0.0",
      description: "Simulated Environment",
      namespace: "polyfill",
    },
    scriptHandler: "Polyfill",
    version: "1.0.0",
  };

  // --- 5. 异步 API (GM.*) 映射 ---
  // 按照 Greasemonkey 4 规范，GM 对象的函数返回 Promise
  window.GM = {
    getValue: (k, d) => Promise.resolve(window.GM_getValue(k, d)),
    setValue: (k, v) => Promise.resolve(window.GM_setValue(k, v)),
    deleteValue: (k) => Promise.resolve(window.GM_deleteValue(k)),
    listValues: () => Promise.resolve(window.GM_listValues()),

    addStyle: (css) => Promise.resolve(window.GM_addStyle(css)),
    setClipboard: (d, i) => Promise.resolve(window.GM_setClipboard(d, i)),
    openInTab: (u, o) => Promise.resolve(window.GM_openInTab(u, o)),
    notification: (d, c) => Promise.resolve(window.GM_notification(d, c)),

    // 注意：Greasemonkey 4 中对应的名称是大写的 H
    xmlHttpRequest: (d) => window.GM_xmlhttpRequest(d),

    registerMenuCommand: (n, f) =>
      Promise.resolve(window.GM_registerMenuCommand(n, f)),
    unregisterMenuCommand: (id) =>
      Promise.resolve(window.GM_unregisterMenuCommand(id)),

    addValueChangeListener: (k, c) => window.GM_addValueChangeListener(k, c),
    removeValueChangeListener: (id) => window.GM_removeValueChangeListener(id),

    info: window.GM_info,
  };

  window.unsafeWindow = window;

  // 1. 配置常量
  const STORAGE_KEY = "custom_loader_scripts";
  const CURRENT_VERSION = "1.0.1"; // 👈 每次修改列表后，增加这个版本号

  // 2. 脚本列表数据
  const FIXED_SCRIPTS = [
    {
      name: "聊天室",
      describe: "页面右下角的蓝色社交气泡",
      url: "https://inline-chat.moonchan.xyz/loader.js",
      defaultEnabled: true,
    },
    {
      name: "EhSyringe (汉化)",
      describe: "E站注射器：将全站 UI 及 37000+ 标签翻译为中文。",
      url: "https://config.810114.xyz/exhentai/EhSyringe.user.js",
      defaultEnabled: false,
    },
  ];

  /**
   * 初始化/更新存储控制器
   */
  function initStorage() {
    const rawData = localStorage.getItem(STORAGE_KEY);
    let needUpdate = false;

    if (!rawData) {
      // 场景 A: 首次运行，完全没有数据
      console.log("📦 首次运行，初始化数据...");
      needUpdate = true;
    } else {
      try {
        const parsedData = JSON.parse(rawData);
        // 场景 B: 检查版本号。如果本地版本与当前脚本版本不一致，则覆盖
        if (parsedData.version !== CURRENT_VERSION) {
          console.log(
            `🆙 版本更新: ${parsedData.version || "unknown"
            } -> ${CURRENT_VERSION}`
          );
          needUpdate = true;
        }
      } catch (e) {
        // 场景 C: 数据损坏，强制重置
        console.error("⚠️ 存储数据格式损坏，正在重置...");
        needUpdate = true;
      }
    }

    if (needUpdate) {
      const payload = {
        version: CURRENT_VERSION,
        scripts: FIXED_SCRIPTS,
        updatedAt: new Date().toISOString(),
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
      console.log("✅ 数据同步成功");
    } else {
      console.log("ℹ️ 数据已是最新版本，无需操作");
    }
  }

  // 执行
  initStorage();
})();
