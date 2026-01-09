(function () {
  "use strict";

  if (location.pathname.startsWith("/g/")){
    base_url = "/";
    api_url = "/api.php";
    popbase = base_url+"gallerypopups.php?gid=3723085&t=582d4b6579&act=";
  }
  
  // 不让访问s.exhentai.org
  const originalOpen = XMLHttpRequest.prototype.open;
  const BLOCKED_DOMAIN = "s.exhentai.org";

  XMLHttpRequest.prototype.open = function (method, url) {
    // 将 url 转换为字符串（以防传入的是 URL 对象）
    const urlString = String(url);

    // 检查是否包含禁止的域名
    if (urlString.includes(BLOCKED_DOMAIN)) {
      console.error(`不样: ${urlString}`);
      // 抛出异常以阻止请求发送
      throw new Error("Blocked: Request to s.exhentai.org is not allowed.");
    }

    // 如果不是黑名单域名，则正常执行
    return originalOpen.apply(this, arguments);
  };

  // =========================================================================
  // 1. 基础 URL 替换功能 (对应 Go 中的 bytes.ReplaceAll)
  // =========================================================================
  function fixBaseUrls() {
    (function () {
      // 定义要替换的目标：匹配 https://exhentai.org 以及 https://s.exhentai.org
      const targetRegex = /https:\/\/(?:s\.)?exhentai\.org/g;

      /**
       * 1. 通用替换配置
       * selector: 选择器
       * attr: 要修改的属性
       * replaceWith: 替换后的内容（默认为空字符串，即转为相对路径）
       */
      const tasks = [
        {
          selector: 'a[href*="exhentai.org"]',
          attr: "href",
          replaceWith: "",
        },
        {
          selector: 'a[onclick*="exhentai.org"]',
          attr: "onclick",
          replaceWith: "",
        },
        {
          selector: 'div[onclick*="exhentai.org"]',
          attr: "onclick",
          replaceWith: "",
        },
        {
          selector: 'link[href*="exhentai.org"]',
          attr: "href",
          replaceWith: "",
        },
        {
          selector: 'form[action*="exhentai.org"]',
          attr: "action",
          replaceWith: "",
        },
        // div 的 style 属性通常包含背景图，按照您的逻辑优先处理特定的 w/ 路径
        {
          selector: 'div[style*="exhentai.org"]',
          attr: "style",
          replaceWith: "",
        },
        // 图片通常映射到 ehgt.org
        {
          selector: 'img[src*="exhentai.org"]',
          attr: "src",
          replaceWith: "https://ehgt.org",
        },
      ];

      // 执行通用任务
      tasks.forEach(({ selector, attr, replaceWith }) => {
        const elements = document.querySelectorAll(selector);
        elements.forEach((el) => {
          let oldVal = el.getAttribute(attr);
          if (oldVal) {
            let newVal = oldVal;
            // 特殊处理：如果是 style 属性，先处理特定的静态资源前缀映射
            if (attr === "style") {
              newVal = newVal.replace(
                "https://s.exhentai.org/w/",
                "https://ehgt.org/w/"
              );
            }
            // 执行主正则替换
            newVal = newVal.replace(targetRegex, replaceWith);
            el.setAttribute(attr, newVal);
          }
        });
      });

      /**
       * 2. 特殊处理：Script 标签
       * 脚本修改 src 后通常不会自动重新触发加载，因此需要重新创建并插入
       */
      const scripts = document.querySelectorAll('script[src*="exhentai.org"]');
      scripts.forEach((oldScript) => {
        const originalSrc = oldScript.getAttribute("src");
        if (originalSrc) {
          let newSrc = originalSrc.replace(targetRegex, "");
  
          // 创建一个新的 script 标签以触发加载
          const newScript = document.createElement("script");
          newScript.src = newSrc;
          newScript.type = "text/javascript";
          // 显式声明异步，有时能解决加载死锁
          newScript.async = true; 
  
          // 2. 关键修改：直接插入到 head 中，确保环境稳定
          document.head.appendChild(newScript);
  
          oldScript.remove();
        }

      });

      console.log("替换完成：已处理 A, DIV, LINK, SCRIPT, FORM, IMG 的属性。");
    })();
  }
  // =========================================================================
  // 2. CSS 样式注入 (已适配移动端)
  // =========================================================================
  function injectStyles() {
    const style = document.createElement("style");
    style.innerHTML = `
            /* 悬浮窗样式 */
            #moonchan-floating-iframe {
                position: fixed;
                bottom: 20px;
                right: 20px;
                width: 300px;
                height: 200px;
                border: 2px solid #ccc;
                border-radius: 8px;
                box-shadow: 0 0 10px rgba(0, 0, 0, 0.2);
                z-index: 100000;
                overflow: hidden;
                background-color: rgba(255,255,255,0.9);
            }
            #moonchan-close-button {
                position: absolute;
                top: 10px;
                right: 10px;
                background-color: red;
                color: white;
                border: none;
                border-radius: 50%;
                width: 48px;
                height: 48px;
                padding: 6px;
                cursor: pointer;
                font-size: 24px;
                line-height: 48px;
                transition: 0.2s;
            }
            
            /* 按钮容器默认样式 (PC) */
            .custom-btn-container {
                height: 60px;
                width: 100px;
                text-align: center;
                position: fixed;
                z-index: 999; /* 提高层级防止被遮挡 */
                display: flex;
                flex-direction: column;
                gap: 5px;
            }

            /* 右侧按钮位置 (默认) */
            .right-container {
                right: 20px; 
                top: 20px;
            }

            /* 左侧按钮位置 (默认) */
            .left-container {
                left: 20px; 
                top: 20px;
            }

            .custom-btn {
                width: 100%;    
                flex: 1;
                font-size: 16px;
                cursor: pointer;
                border: 1px solid #999;
                background: rgba(255, 255, 255, 0.8);
                color: #000;
                font-weight: bold;
            }

            /* === 移动端触屏适配 (屏幕宽度 < 768px) === */
            @media screen and (max-width: 768px) {
                /* 调整悬浮窗位置和大小 */
                #moonchan-floating-iframe {
                    width: 90%;
                    left: 5%;
                    bottom: 10px;
                    right: auto;
                    font-size: 12px;
                }

                /* 移动端按钮容器样式 */
                .custom-btn-container {
                    width: 70px; /* 稍微变窄 */
                    height: auto;
                }

                /* 右侧按钮：移至右下角拇指易触区域 */
                .right-container {
                    top: auto;
                    bottom: 150px; /* 避开底部导航栏或悬浮窗 */
                    right: 10px;
                }

                /* 左侧按钮：移至左下角或折叠，避免遮挡顶部 */
                .left-container {
                    top: auto;
                    bottom: 150px;
                    left: 10px;
                }

                /* 按钮样式优化：增加点击热区，增加透明度防止遮挡 */
                .custom-btn {
                    padding: 10px 0;
                    font-size: 14px;
                    opacity: 0.7; /* 半透明 */
                    border-radius: 8px;
                    box-shadow: 0 2px 5px rgba(0,0,0,0.3);
                }
                
                .custom-btn:active {
                    opacity: 1;
                    background: #eee;
                }
            }
        `;
    document.head.appendChild(style);
  }

  // =========================================================================
  // 3. 重新加载封面功能
  // =========================================================================
  function initReloadCover() {
    if (localStorage.getItem("waterfall") === "false") {
      return;
    }
    // 检查是否存在 gl3t 元素
    const gl3tElements = document.getElementsByClassName("gl3t");
    if (gl3tElements.length === 0) return;

    // 1. 创建按钮容器
    const container = document.createElement("div");
    // 直接将需求中的 style 属性原样复制进来
    container.style.cssText = `
    height: 60px;
    width: 100px;
    text-align: center;
    position: fixed;
    right: 20px; 
    top: 20px;
    z-index: 99;
    display: table-cell;
    vertical-align: middle;
`;

    // 2. 创建按钮
    const btn = document.createElement("button");
    btn.id = "reload-cover";
    btn.innerText = "重新加载封面";
    // 直接将需求中的 style 属性复制进来
    // 注意：原 HTML 中是 display: none，但根据你 JS 注释 "直接判断后显示"，
    // 这里我将其改为 display: block 以便代码运行后你能直接看到按钮
    btn.style.cssText = `
    width: 100%;    
    height: 100%;
    font-size: x-large;
    display: block; 
`;

    // 3. 组装并添加到页面
    container.appendChild(btn);
    document.body.appendChild(container);

    // 绑定点击事件
    btn.addEventListener(
      "click",
async function () {
    window.stop(); // 注意：如果你想检查图片是否加载成功，建议注释掉 window.stop()，否则所有图片都会停止加载
    
    const replace = async function (elements) {
        const elArray = Array.from(elements);

        async function replaceElement(element) {
          
            const link = element.querySelector("a");
            if (!link) return;

            const galleryUrl = link.href;
            const img = link.querySelector("img");
            
            if (!img || !galleryUrl) return;

            // --- 新增判断逻辑 ---
            // 1. img.complete: 图片加载完成（无论成功还是失败都会是 true）
            // 2. img.naturalWidth > 0: 图片的实际宽度大于 0，说明数据有效且非破损图
            // 3. (可选) 检查 src 是否已经包含跳转参数，避免重复处理
            if (img.complete && img.naturalWidth > 0) {
                if (img.src.includes("redirect_to=image")) {
                    console.log(`[跳过] 已经是高清图: ${galleryUrl}`);
                    return;
                }
                // 如果图片本身已经加载成功（缩略图正常），你可能也想跳过
                // console.log(`[跳过] 图片已正常显示: ${galleryUrl}`);
                // continue; 
            }
            // --------------------

            try {
                // 如果图片未加载，或加载失败 (naturalWidth === 0)，则执行 fetch
                const response = await fetch(galleryUrl);
                const html = await response.text();

                const parser = new DOMParser();
                const doc = parser.parseFromString(html, "text/html");
                const gdtContainer = doc.getElementById("gdt");

                if (gdtContainer) {
                    const firstPageLink = gdtContainer.querySelector("a");
                    if (firstPageLink) {
                        const href = firstPageLink.href;
                        const separator = href.includes("?") ? "&" : "?";
                        const newSrc = href + separator + "redirect_to=image";

                        // 替换图片并打印日志
                        console.log(`[替换成功] ${newSrc}`);
                        // 建议直接用完整链接，或者根据站内规则替换
                        img.src = newSrc.replace("https://exhentai.org/", "/"); 
                        
                        // 为了防止 window.stop() 干扰后续新图加载，可以手动触发加载
                        img.onerror = function() {
                            console.error("图片替换后加载失败:", newSrc);
                        };
                    }
                }
            } catch (err) {
                console.error("Fetch出错:", galleryUrl, err);
            }
        }
              for (const element of elArray) {
                replaceElement(element);
              }
    };

    const gl3tElements = document.getElementsByClassName("gl3t");
    await replace(gl3tElements);

    const gl1eElements = document.getElementsByClassName("gl1e");
    await replace(gl1eElements);
},
      false
    );
  }
  // =========================================================================
  // 4. 瀑布流 (Waterfall) 与 复制外链功能 (已适配触屏)
  // =========================================================================
  function initWaterfall() {
    if (!location.pathname.startsWith("/s/")) return;

    (function () {
      "use strict";

      // --- 创建容器 (UI) ---

      // 1. 右侧容器 (包含：重载封面、下拉式1、下拉式2)
      // 为了防止原代码中两个 div 都在 top:20px 重叠，这里使用 Flex 布局垂直排列
      const rightContainer = document.createElement("div");
      rightContainer.style.cssText = `
        width: 100px;
        position: fixed;
        right: 20px;
        top: 20px;
        z-index: 99;
        display: flex;
        flex-direction: column;
        gap: 10px; /* 按钮之间的间距 */
        pointer-events: none; /* 让容器不阻挡点击，子元素恢复点击 */
    `;

      // 2. 左侧容器 (包含：复制图片外链)
      const leftContainer = document.createElement("div");
      leftContainer.style.cssText = `
        height: 60px;
        width: 100px;
        text-align: center;
        position: fixed;
        left: 20px;
        top: 20px;
        z-index: 99;
    `;

      // --- 创建按钮 ---

      // 通用按钮样式工厂
      function createBtnStyle(fontSize = "x-large", display = "block") {
        return `
            width: 100%;
            height: 60px;
            font-size: ${fontSize};
            display: ${display};
            pointer-events: auto;
            cursor: pointer;
            margin-bottom: 5px;
        `;
      }

      // 按钮 1: 重新加载封面 (默认隐藏)
      const btnReload = document.createElement("button");
      btnReload.id = "reload-cover";
      btnReload.innerText = "重新加载封面";
      btnReload.style.cssText = createBtnStyle("x-large", "none"); // display: none

      // 按钮 2: 下拉式1
      const btnWaterfall1 = document.createElement("button");
      btnWaterfall1.id = "waterfall";
      btnWaterfall1.innerText = "下拉式1";
      btnWaterfall1.style.cssText = createBtnStyle();

      // 按钮 3: 下拉式2
      const btnWaterfall2 = document.createElement("button");
      btnWaterfall2.id = "waterfall2";
      btnWaterfall2.innerText = "下拉式2";
      btnWaterfall2.style.cssText = createBtnStyle();

      // 按钮 4: 复制图片外链 (左侧)
      const btnOrigin = document.createElement("button");
      btnOrigin.id = "originBtn";
      btnOrigin.innerText = "复制图片外链";
      btnOrigin.style.cssText = createBtnStyle();

      // --- 组装 DOM ---

      rightContainer.appendChild(btnReload);
      rightContainer.appendChild(btnWaterfall1);
      rightContainer.appendChild(btnWaterfall2);
      leftContainer.appendChild(btnOrigin);

      document.body.appendChild(rightContainer);
      document.body.appendChild(leftContainer);

      // --- 逻辑功能实现 ---

      // 逻辑 1: 下拉式1 (瀑布流加载)
      async function execWaterfall() {
        console.log("!");
        // 移除按钮 (保持原逻辑)
        if (document.getElementById("originBtn"))
          document.getElementById("originBtn").remove();
        if (document.getElementById("waterfall"))
          document.getElementById("waterfall").remove();
        if (document.getElementById("waterfall2"))
          document.getElementById("waterfall2").remove();

        let pn = document.createElement("div");
        let lp = location.href;
        let ln = location.href;

        // 注意：原代码假设页面有名为 i1 的元素作为容器
        const element = document.getElementById("i1");
        if (!element) {
          console.error("未找到 id='i1' 的元素，无法加载瀑布流");
          return;
        }
        element.appendChild(pn);

        // 获取下一页链接
        let nextEl = document.getElementById("next");
        if (!nextEl) return;
        let hn = nextEl.href;

        while (hn != ln) {
          let doc;
          while (!doc) {
            try {
              doc = await fetch(hn)
                .then((resp) => resp.text())
                .then((data) => {
                  // console.log(data); // 原代码有log，略微有些刷屏，可注释
                  let parser = new DOMParser();
                  return parser.parseFromString(data, "text/html");
                });
            } catch (e) {
              console.error("Fetch error:", e);
              break;
            }
          }
          // console.log(doc);

          let img = document.createElement("img");
          let imgElement = doc.getElementById("img"); // 获取新页面里的 img 元素

          if (imgElement) {
            img.src = imgElement.src;
            // 简单的样式优化，防止图片过大溢出
            img.style.maxWidth = "100%";
            img.style.display = "block";
            img.style.margin = "0 auto";

            pn.appendChild(img);
            ln = hn; // 更新当前页
            let nextLink = doc.getElementById("next");
            if (nextLink) {
              hn = nextLink.href; // 更新下一页
              hn = hn.replace("https://exhentai.org", "");
            } else {
              break;
            }
          } else {
            break;
          }
        }
        let p = document.createElement("p");
        p.innerHTML = hn;
        pn.appendChild(p);
      }

      // 逻辑 2: 下拉式2 (跳转 URL)
      async function execWaterfall2() {
        // 获取当前 URL 的路径
        const currentPath = window.location.pathname;
        // 定义新的连接
        const newUrl = "/?host=page.moonchan.xyz#" + currentPath;
        // 跳转到新的连接
        window.location.href = newUrl;
      }

      // 逻辑 3: 复制图片外链
      function copyOriginLink() {
        const currentUrl = window.location.href;
        // 智能添加 ? 或 &
        const hasQuery = currentUrl.includes("?");
        const newUrl =
          currentUrl + (hasQuery ? "&" : "?") + "redirect_to=image";

        if (navigator.clipboard) {
          navigator.clipboard
            .writeText(newUrl)
            .then(() => alert("已复制到剪贴板！"))
            .catch(() => fallbackCopy(newUrl));
        } else {
          fallbackCopy(newUrl);
        }

        function fallbackCopy(text) {
          const input = document.createElement("input");
          input.value = text;
          document.body.appendChild(input);
          input.select();
          try {
            document.execCommand("copy");
            alert("已复制（兼容模式）");
          } catch (err) {
            alert("复制失败，请手动复制");
          } finally {
            document.body.removeChild(input);
          }
        }
      }

      // --- 绑定事件监听 ---

      btnWaterfall1.addEventListener("click", execWaterfall, false);
      btnWaterfall2.addEventListener("click", execWaterfall2, false);
      btnOrigin.addEventListener("click", copyOriginLink, false);

      // reload-cover 原代码中默认隐藏且没有绑定具体事件逻辑，
      // 如果需要显示它，可以在控制台输入 document.getElementById('reload-cover').style.display = 'block'
    })();
  }

  // =========================================================================
  // 5. 注入脚本 (Client Loader / Inline Chat)
  // =========================================================================
  function initScriptLoader() {
    const loadScript = (src) => {
      const script = document.createElement("script");
      script.src = src;
        script.defer = true; // This adds the defer attribute
      document.body.appendChild(script);
    };

    // inline-chat
    if (localStorage.getItem("chat") !== "false") {
      loadScript("https://inline-chat.moonchan.xyz/loader.js");
      console.log("GM Polyfill loaded via localStorage.");
    }

    // ehsyringe
    if (localStorage.getItem("ehsyringe") !== "false") {
      loadScript("/exhentai/gm-polyfill.js");
      loadScript("/exhentai/EhSyringe.user.js");
    }

    // gscript
    if (
      localStorage.getItem("gscript") !== "false" &&
      location.pathname.startsWith("/g/")
    ) {
      loadScript("/exhentai/gscript.js");
    }

    // reader
    if (
      localStorage.getItem("reader") !== "false" &&
      location.pathname.startsWith("/g/")
    ) {
      loadScript("/exhentai/reader.js");
    }
    if (
      localStorage.getItem("reader-test") === "true" &&
      location.pathname.startsWith("/g/")
    ) {
      loadScript("/exhentai/reader-test.js");
    }
  }

  // =========================================================================
  // 6. 右下角悬浮窗
  // =========================================================================
  function initFloatingNotice() {
    const mark = "1228";
    if (localStorage.getItem("iframeClosed") === mark) {
      return;
    }

    const div = document.createElement("div");
    div.id = "moonchan-floating-iframe";
    div.innerHTML = `
            <button id="moonchan-close-button">×</button>
            <div style="padding: 10px; font-size: 14px;">
                <p>moonchan.xyz有DNS污染迹象，请注意迁移到以下节点</p>
                <p style="color: black;">New:<a href="https://ex.810114.xyz/" target="_blank">https://ex.810114.xyz/</a>（无污染永续）</p>			
                <p style="color: black;">更了下程序，如果有问题，及时反馈<a href="https://810114.xyz/" target="_blank">https://810114.xyz/</a></p>			
            </div>
        `;
    document.body.appendChild(div);

    document
      .getElementById("moonchan-close-button")
      .addEventListener("click", function () {
        div.style.display = "none";
        localStorage.setItem("iframeClosed", mark);
      });
  }

  // =========================================================================
  // 主入口
  // =========================================================================
  // 确保 DOM 加载完成后执行
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", main);
  } else {
    main();
  }

  async function main() {
    console.log(2331);
    // fixBaseUrls(); // 1. URL 替换
    injectStyles(); // 2. 样式注入
    initReloadCover(); // 3. 封面重载 (列表页)
    initWaterfall(); // 4. 瀑布流 (阅读页)
    initScriptLoader(); // 5. 外部脚本加载
    initFloatingNotice(); // 6. 悬浮窗


// 假设 rangebar 是一个在当前作用域内可访问的变量，
// 可能是通过 document.getElementById 或其他方式获取的DOM元素引用。
// 同样，build_rangebar 也是一个在当前作用域内可访问的函数。

function checkAndBuildRangebar() {
    // 重新评估 rangebar 的值，以防它在 DOM 加载后才出现
    // 如果 rangebar 变量是动态获取的，例如每次都需要查找DOM，
    // 则可以在这里重新获取它：
    const rangebar = document.getElementById('rangebar');

    if (rangebar && build_rangebar) { // rangebar 存在 (不为 null, undefined, false, 0, "" 等)
        if (rangebar.children.length > 0) {
            console.log("rangebar 的子元素不为空");
        } else { // rangebar 存在，但子元素为空
            console.log("rangebar 存在但子元素为空，尝试构建...");
            build_rangebar?.();
            // fixBaseUrls(); // 1. URL 替换
        }
    } else { // rangebar 不存在 (null 或 undefined)
        console.log("rangebar 不存在，将在 0.2 秒后重试...");
        // 间隔0.2秒后再次运行此逻辑
        setTimeout(checkAndBuildRangebar, 200); // 200毫秒 = 0.2秒
    }
}

// 首次调用该函数以启动逻辑
// setTimeout(checkAndBuildRangebar, 1500);



    
  }
})();
