(async function() {
    'use strict';

    // 1. 检查页面URL格式
    // 格式要求：/g/数字/字母数字/ (例如: /g/3706983/69a4906183/)
    const path = window.location.pathname;
    const urlPattern = /^\/g\/\d+\/[\w\d]+\/?$/;
    
    // 如果不匹配目标格式，则停止运行
    if (!urlPattern.test(path)) {
        console.log("当前页面URL不符合脚本运行格式，脚本已停止。");
        return;
    }

    console.log("脚本开始执行...");

    // 2. XPath 选中 Length 并获取页数
    // 目标结构: <div id="gdd">...<td class="gdt1">Length:</td><td class="gdt2">104 pages</td>...</div>
    const xpath = '//div[@id="gdd"]//td[@class="gdt1" and contains(text(), "Length")]/following-sibling::td[@class="gdt2"]';
    const lengthNode = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;

    if (!lengthNode) {
        console.error("未能通过XPath找到Length信息。");
        return;
    }

    // 解析数量 (例如: "104 pages" -> 104)
    const totalImages = parseInt(lengthNode.textContent);
    const itemsPerPage = 40;
    // 计算总页数 (从0开始算，所以总页数决定了循环次数)
    const totalPages = Math.ceil(totalImages / itemsPerPage);

    console.log(`总图片数: ${totalImages}, 每页: ${itemsPerPage}, 总页数: ${totalPages}`);

    // 3. 隐藏 class=gtb 的所有元素 (通常是翻页器)
    const paginators = document.querySelectorAll('.gtb');
    paginators.forEach(el => el.style.display = 'none');

    // 4. 准备容器
    const galleryContainer = document.getElementById('gdt');
    if (!galleryContainer) return;

    // 清空当前容器，确保按顺序重组
    galleryContainer.innerHTML = ''; 

    // 5. 循环 Fetch 并合并
    for (let p = 0; p < totalPages; p++) {
        try {
            // 构建带参数的URL
            const fetchUrl = new URL(window.location.href);
            fetchUrl.searchParams.set('p', p);
            
            console.log(`正在获取第 ${p + 1} 页 (p=${p})...`);
            
            const response = await fetch(fetchUrl.toString());
            const htmlText = await response.text();
            
            // 解析返回的HTML
            const parser = new DOMParser();
            const doc = parser.parseFromString(htmlText, 'text/html');
            
            // 找到 id=gdt 的元素下的所有 children (即 <a> 标签)
            const newItems = doc.querySelectorAll('#gdt > a');
            
            if (newItems.length > 0) {
                // 创建文档片段以减少重绘
                const fragment = document.createDocumentFragment();
                
                newItems.forEach(item => {
                    // --- 新增步骤：替换 href 属性 ---
                    const currentHref = item.getAttribute('href');
                    if (currentHref) {
                        // 将 https://exhentai.org 替换为空字符串
                        // 这通常会将绝对路径转换为相对路径 (例如 /s/hash/id)
                        const newHref = currentHref.replace('https://exhentai.org', '');
                        item.setAttribute('href', newHref);
                    }
                    // -----------------------------

                    fragment.appendChild(item);
                });
                
                // 合并到当前页面的 id=gdt 中
                galleryContainer.appendChild(fragment);
            }
            
        } catch (err) {
            console.error(`获取第 ${p} 页失败:`, err);
        }
    }
    
    console.log("所有页面加载完成。");

})();
