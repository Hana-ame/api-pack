(async function () {
  // -----------------------------------------------------------
  // 1. 爬虫与智能缓存模块
  // -----------------------------------------------------------
  const gdt = document.querySelector("#gdt") || document.querySelector(".gdt");
  let PAGE_URLS = gdt
    ? Array.from(gdt.querySelectorAll("a"))
        .map((a) => a.href)
        .filter((href) => href.includes("/s/"))
    : [];

  const CACHE = {}; // 结果缓存 (URL)
  const REQS = {}; // 请求去重池 (Promise)
  const RELOAD_KEYS = {}; // Stores the 'nl' key for reloading

  // 获取图片地址 (自动去重 + 缓存 + 提取 Reload Key)
  const getSrc = (index) => {
    if (index < 0 || index >= PAGE_URLS.length) return Promise.resolve(null);
    if (CACHE[index]) return Promise.resolve(CACHE[index]);
    if (REQS[index]) return REQS[index];

    const p = fetch(PAGE_URLS[index])
      .then((r) => r.text())
      .then((html) => {
        const doc = new DOMParser().parseFromString(html, "text/html");
        
        // 1. Try to find the image
        const img = doc.getElementById("img");
        const src = img ? img.src : null;

        // 2. Try to find the reload key (nl) from <a id="loadfail">
        // Format: onclick="return nl('42371-491095')"
        const loadFail = doc.getElementById("loadfail");
        if (loadFail) {
          const onclick = loadFail.getAttribute("onclick");
          const match = onclick ? onclick.match(/nl\('([^']+)'\)/) : null;
          if (match && match[1]) {
            RELOAD_KEYS[index] = match[1];
          }
        }

        if (src) CACHE[index] = src;
        delete REQS[index];
        return src;
      })
      .catch(() => {
        delete REQS[index];
        return null;
      });

    REQS[index] = p;
    return p;
  };

  // 预加载器
  const preload = (i) => {
    for (let j = 1; j <= 5; j++) // Reduced to 5 to save bandwidth
      getSrc(i + j).then((u) => u && (new Image().src = u));
  };
  preload(0);

  // -----------------------------------------------------------
  // 2. 依赖加载
  // -----------------------------------------------------------
  const loadScript = (src) =>
    new Promise((r) => {
      if (document.querySelector(`script[src="${src}"]`)) return r();
      const s = document.createElement("script");
      s.src = src;
      s.crossOrigin = "anonymous";
      s.onload = r;
      document.head.append(s);
    });

  try {
    await loadScript("https://unpkg.com/react@18/umd/react.production.min.js");
    await loadScript(
      "https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"
    );
  } catch (e) {
    return console.error(e);
  }

  // -----------------------------------------------------------
  // 3. 启动按钮
  // -----------------------------------------------------------
  const btn = document.createElement("button");
  btn.innerText = "Start Ultimate Reader";
  btn.style.cssText =
    "position:fixed; top:10px; right:10px; z-index:999999; padding:10px; background:#e91e63; color:#fff; border:none; cursor:pointer; font-weight:bold; box-shadow:0 2px 5px rgba(0,0,0,0.5); border-radius:4px;";
  document.body.append(btn);

  // -----------------------------------------------------------
  // 4. React 组件
  // -----------------------------------------------------------
  btn.onclick = () => {
    const gdt =
      document.querySelector("#gdt") || document.querySelector(".gdt");
    PAGE_URLS = gdt
      ? Array.from(gdt.querySelectorAll("a"))
          .map((a) => a.href)
          .filter((href) => href.includes("/s/"))
      : [];

    if (!PAGE_URLS.length) return alert("No links");
    document.body.innerHTML = '<div id="root"></div>';
    Object.assign(document.body.style, {
      margin: "0",
      background: "#000",
      overflow: "hidden",
    });

    const { useState, useEffect, useRef, createElement: h } = window.React;
    const { createRoot } = window.ReactDOM;

    const App = () => {
      const [index, setIndex] = useState(0);
      const [src, setSrc] = useState(null);
      const [isError, setIsError] = useState(false); // Track image load error

      // 视图状态
      const [scale, setScale] = useState(1);
      const [offset, setOffset] = useState({ x: 0, y: 0 });
      const [isDragging, setIsDragging] = useState(false);

      const dragStart = useRef({ x: 0, y: 0 });
      const lastOffset = useRef({ x: 0, y: 0 });
      const lastTap = useRef(0);

      // 翻页
      const next = () => setIndex((i) => Math.min(i + 1, PAGE_URLS.length - 1));
      const prev = () => setIndex((i) => Math.max(i - 1, 0));

      // 键盘监听
      useEffect(() => {
        const k = (e) => {
          if (e.key === "ArrowLeft") next();
          if (e.key === "ArrowRight") prev();
        };
        window.addEventListener("keydown", k);
        return () => window.removeEventListener("keydown", k);
      }, []);

      // 核心逻辑：页码变化
      useEffect(() => {
        let m = true;
        // Reset View
        setScale(1);
        setOffset({ x: 0, y: 0 });
        lastOffset.current = { x: 0, y: 0 };
        setIsError(false); // Reset error state

        setSrc(null);

        getSrc(index).then((u) => {
          if (m) {
             setSrc(u);
             // If we have a reload key but no src (parse error), consider it an error
             if (!u && RELOAD_KEYS[index]) setIsError(true);
          }
        });

        preload(index);
        return () => (m = false);
      }, [index]);

      // RELOAD FUNCTION
      const handleReload = () => {
        const nlKey = RELOAD_KEYS[index];
        if (!nlKey) {
            alert("No reload key found for this page (try waiting a bit or the page is hard-broken).");
            return;
        }

        // 1. Construct new URL with ?nl=...
        const currentUrl = PAGE_URLS[index];
        // Remove existing params just in case to avoid duplicates
        const baseUrl = currentUrl.split('?')[0]; 
        const newUrl = `${baseUrl}?nl=${nlKey}`;
        
        console.log(`Reloading: ${currentUrl} -> ${newUrl}`);
        
        // 2. Update global state
        PAGE_URLS[index] = newUrl;
        delete CACHE[index];
        delete REQS[index];
        
        // 3. UI Update
        setSrc(null);
        setIsError(false);
        
        // 4. Re-fetch
        getSrc(index).then(u => {
            setSrc(u);
            // If u is null, show error again
            if(!u) setIsError(true);
        });
      };

      // 手势
      const pDown = (e) => {
        e.preventDefault();
        setIsDragging(true);
        dragStart.current = { x: e.clientX, y: e.clientY };
      };
      const pMove = (e) => {
        if (isDragging)
          setOffset({
            x: lastOffset.current.x + (e.clientX - dragStart.current.x),
            y: lastOffset.current.y + (e.clientY - dragStart.current.y),
          });
      };
      const pUp = (e) => {
        setIsDragging(false);
        lastOffset.current = offset;
        if (
          Math.hypot(
            e.clientX - dragStart.current.x,
            e.clientY - dragStart.current.y
          ) > 10
        )
          return;

        const w = window.innerWidth;
        const x = e.clientX;
        const now = Date.now();

        // Only handle clicks for nav if not zoomed
        if (scale === 1) {
          if (x < w * 0.35) return next();
          if (x > w * 0.65) return prev();
        }

        if (x >= w * 0.35 && x <= w * 0.65) {
          if (now - lastTap.current < 300) {
            if (scale === 1) setScale(2);
            else {
              setScale(1);
              setOffset({ x: 0, y: 0 });
              lastOffset.current = { x: 0, y: 0 };
            }
            lastTap.current = 0;
          } else lastTap.current = now;
        }
      };

      return h(
        "div",
        {
          style: {
            display: "grid",
            justifyItems: "center",
            width: "100vw",
            height: "100vh",
            background: "#000",
            overflow: "hidden",
            touchAction: "none",
            userSelect: "none",
            cursor: isDragging ? "grabbing" : "default",
          },
          onPointerDown: pDown,
          onPointerMove: pMove,
          onPointerUp: pUp,
          onPointerLeave: () => setIsDragging(false),
        },
        [
          // Layer 1: Loading Text
          h(
            "div",
            {
              key: "loader",
              style: {
                gridArea: "1/1",
                color: "#666",
                fontSize: "20px",
                fontWeight: "bold",
                zIndex: 0,
                alignSelf: "center"
              },
            },
            src ? "Loading Image..." : "Fetching Info..."
          ),

          // Layer 2: Image
          src && !isError &&
            h("img", {
              key: index, // Force re-render on index change
              src: src,
              draggable: false,
              onError: () => setIsError(true), // Trigger reload button on fail
              style: {
                gridArea: "1/1",
                maxWidth: "100vw",
                maxHeight: "100vh",
                objectFit: "contain",
                transform: `translate(${offset.x}px, ${offset.y}px) scale(${scale})`,
                transition: isDragging ? "none" : "transform 0.2s",
                pointerEvents: "none",
                zIndex: 1,
                willChange: "transform",
              },
            }),

          // Layer 3: Reload Button (Shown if Error)
          isError &&
            h(
                "button",
                {
                    onClick: (e) => { e.stopPropagation(); handleReload(); },
                    style: {
                        gridArea: "1/1",
                        zIndex: 10,
                        padding: "15px 30px",
                        fontSize: "18px",
                        background: "#ff5722",
                        color: "white",
                        border: "none",
                        borderRadius: "8px",
                        boxShadow: "0 4px 10px rgba(0,0,0,0.5)",
                        cursor: "pointer",
                        alignSelf: "center"
                    }
                },
                "⚠️ Image Failed - Tap to Reload"
            ),

          // Layer 4: Page Number
          h(
            "div",
            {
              style: {
                position: "absolute",
                bottom: 20,
                color: "#ccc",
                background: "rgba(0,0,0,0.5)",
                padding: "4px 8px",
                borderRadius: 4,
                fontSize: "12px",
                pointerEvents: "none",
                zIndex: 2,
              },
            },
            `${index + 1} / ${PAGE_URLS.length}`
          ),
        ]
      );
    };

    createRoot(document.getElementById("root")).render(h(App));
  };
})();
