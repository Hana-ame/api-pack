package exhentai

import (
	"bytes"
	"errors"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// toolkik: xpath

const InnerText string = "INNER_TEXT"

func findOneAndSelectAttr(top *html.Node, expr string, name string) (v string, err error) {
	elem := htmlquery.FindOne(top, expr)
	if elem == nil {
		err = errors.New(expr + ":" + name + "is null")
		return
	}
	if name == InnerText {
		v = htmlquery.InnerText(elem)
	} else {
		v = htmlquery.SelectAttr(elem, name)
	}
	return
}

func findAll(top *html.Node, expr, name string) (v []string) {
	elemArray := htmlquery.Find(top, expr)
	v = make([]string, len(elemArray))
	for i, e := range elemArray {
		if name == InnerText {
			v[i] = htmlquery.InnerText(e)
		} else {
			v[i] = htmlquery.SelectAttr(e, name)
		}
	}
	return
}

func addReloadCoverButton(html []byte) []byte {
	html = bytes.Replace(html, []byte("<body>"), []byte(`<body>
	<div style="
	  height: 60px;
	  width: 100px;
	  text-align: center;
	  /* background-color: violet; */
	  position: fixed;
	  right: 20px; 
	  top: 20px;
	  z-index: 99;
	  display: table-cell;
	  vertical-align: middle;
	  /* float: right; */
	">
	  <button id="reload-cover" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
			display: none;
	  ">
			重新加载封面
	  </button>
	</div>
`), 1)

	html = bytes.Replace(html, []byte("</body>"), []byte(`<script type="text/javascript">
	// 获取所有class为gl3t的元素
	var gl3tElements = document.getElementsByClassName('gl3t');

	if (gl3tElements.length > 0) 
		document.getElementById('reload-cover').style.display = 'block';

	async function execReload() {
		window.stop();

		var gl3tElements = document.getElementsByClassName('gl3t');
		// 遍历每个gl3t元素
		for (var i = 0; i < gl3tElements.length; i++) {
			// 获取当前元素中的所有a标签
			var links = gl3tElements[i].getElementsByTagName('a');
			for (var j = 0; j < links.length; j++) {
				// 获取a标签的href
				var href = links[j].href;
				console.log(links[j]);
				// 获取当前a标签中的img标签
				var imgs = links[j].getElementsByTagName('img');
				for (var k = 0; k < imgs.length; k++) {
					// 修改img的src属性
					imgs[k].src = href + '?redirect_to=cover';
				}
			}
		}
	}
	document.getElementById("reload-cover").addEventListener("click", execReload, false); 

	// if (gl3tElements.length > 0) 
	// 	execReload();

	</script>

</body>`), 1)

	return html
}

func addWaterFallViewButton(html string) string {
	return strings.Replace(html, "<body>", `<body>
	<div style="
	  height: 60px;
	  width: 100px;
	  text-align: center;
	  /* background-color: violet; */
	  position: fixed;
	  right: 20px; 
	  top: 20px;
	  z-index: 99;
	  display: table-cell;
	  vertical-align: middle;
	  /* float: right; */
	">
	  <button id="waterfall" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
	  ">
			下拉式1
	  </button>
	  <button id="waterfall2" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
	  ">
			下拉式2
	  </button>
	</div>
	<!-- 新增的左上角按钮、不行要C，还是删了 -->
	<div style="
		height: 60px;
		width: 100px;
		text-align: center;
		position: fixed;
		left: 20px; 
		top: 20px;
		z-index: 99;"
	>
		<button id="originBtn" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;"
		>复制图片外链</button>
	</div>
  <script type="text/javascript">
	async function execWaterfall(){
		console.log('!');
		document.getElementById("originBtn").remove();
		document.getElementById("waterfall").remove();
		document.getElementById("waterfall2").remove();
		let pn = document.createElement('div');
		let lp = location.href;
		let ln = location.href;
		const element = document.getElementById('i1');
		element.appendChild(pn);
		let hn = document.getElementById('next').href;
		while (hn != ln) {
		  let doc;
		  while(!doc) {
			doc = await fetch(hn).then(resp => resp.text())			
			  .then(data => {
			    console.log(data);
			    let parser = new DOMParser();
			    let doc = parser.parseFromString(data, "text/html");
			    return doc;
			  });
			}
		  console.log(doc);
		  let img = document.createElement('img');
		  let element = doc.getElementById('img');
		  if (element) {
			img.src = element.src;
			pn.appendChild(img);
			ln = hn;
			hn = doc.getElementById('next').href;
		  }
		}
		let p = document.createElement('p');
		p.innerHTML = hn;
	  }
	async function execWaterfall2(){
		// 获取当前 URL 的路径
		const currentPath = window.location.pathname;

		// 定义新的连接
		const newUrl = '/?host=page.moonchan.xyz#' + currentPath;

		// 跳转到新的连接
		window.location.href = newUrl;
	  }
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	document.getElementById("waterfall2").addEventListener("click", execWaterfall2, false); 
	document.getElementById("originBtn").addEventListener("click", function() {

  //   const currentUrl = window.location.href.split('?')[0];
  //   window.location.host = "eh-web-viewer.moonchan.xyz";
  const currentUrl = window.location.href;
  // 方法二：兼容现有参数（智能添加 ? 或 &）
  const hasQuery = currentUrl.includes('?');
  const newUrl = currentUrl + (hasQuery ? '&' : '?') + 'redirect_to=image';

  if (navigator.clipboard) {
    navigator.clipboard.writeText(newUrl)
      .then(() => alert('已复制到剪贴板！'))
      .catch(() => fallbackCopy(newUrl));
  } else {
    fallbackCopy(newUrl);
  }
  function fallbackCopy(text) {
    const input = document.createElement('input');
    input.value = text;
    document.body.appendChild(input);
    input.select();
    try {
      document.execCommand('copy');
      alert('已复制（兼容模式）');
    } catch (err) {
      alert('复制失败，请手动复制');
    } finally {
      document.body.removeChild(input);
    }
  }
	
    });
	</script>`, 1)
}

func addInlineChatRoom(html []byte) []byte {
	// 注入客户端 Loader JS
	// 它会读取 localStorage 中的 custom_loader_scripts，并动态创建 <script> 标签
	clientLoader := `<script>
	(function() {
		// 这里设置你在 localStorage 中存储的键名，例如 "use_polyfill"
		// 假设当值为 "true" 时加载
		if (localStorage.getItem("chat") !== "false") { // default is true
			var script = document.createElement("script");
			script.src = "https://inline-chat.moonchan.xyz/loader.js";
			// 添加到 head 或 body 中
			document.body.appendChild(script);
			console.log("GM Polyfill loaded via localStorage.");
		}
		if (localStorage.getItem("ehsyringe") !== "false") { 
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/gm-polyfill.js";	
				document.body.appendChild(script);
			}
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/EhSyringe.user.js";	
				document.body.appendChild(script);
			}
		}
		if (localStorage.getItem("gscript") !== "false" && location.pathname.startsWith("/g/")) { 
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/gscript.js";	
				document.body.appendChild(script);
			}
		}
		if (localStorage.getItem("reader") !== "false" && location.pathname.startsWith("/g/")) { 
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/reader.js";
				document.body.appendChild(script);
			}
		}
	})();

	</script>
	`

	// 将引导脚本插入到 </body> 之前
	html = bytes.Replace(html, []byte("</body>"), append([]byte(clientLoader), []byte("\n</body>")...), 1)
	return html
}

func addFloatingIframeAtRightBottom(html []byte) []byte {
	html = bytes.Replace(html,
		[]byte("</head>"),
		[]byte(`
	<style>
		#moonchan-floating-iframe {
			position: fixed;
			bottom: 20px; /* 距离底部的距离 */
			right: 20px; /* 距离右侧的距离 */
			width: 300px; /* 根据需要调整宽度 */
			height: 200px; /* 根据需要调整高度 */
			border: 2px solid #ccc; /* 边框样式 */
			border-radius: 8px; /* 圆角边框 */
			box-shadow: 0 0 10px rgba(0, 0, 0, 0.2); /* 阴影效果 */
			z-index: 100000; /* 确保在最上层 */
			overflow: hidden; /* 确保内容不超出边框 */
			background-color: rgba(255,255,255,0.5); /* 背景颜色 */
		}       
#moonchan-close-button {
    position: absolute;
    top: 10px;
    right: 10px;
    background-color: red;
    color: white;
    border: none;
    border-radius: 50%;
    width: 48px;  /* iOS规范最小值44px的适配值 */
    height: 48px;
    padding: 6px; /* 增强触控容错 */
    cursor: pointer;
    font-size: 24px;
    line-height: 48px;
    transition: 0.2s;
    /* 扩展热区 */
    &:after {
        content: '';
        position: absolute;
        top: -10px;
        right: -10px;
        bottom: -10px;
        left: -10px;
    }
    /* 按压反馈 */
    &:active {
        transform: scale(0.9);
    }
    /* 禁用状态 */
    &[disabled] {
        opacity: 0.6;
        pointer-events: none;
    }
}
	</style>
</head>`), 1)
	html = bytes.Replace(html,
		[]byte("<body>"),
		[]byte(`<body>
    <div id="moonchan-floating-iframe" style="display: none;">
        <button id="moonchan-close-button" onclick="moonchanCloseIframe()">×</button>
        <div>
			<p>moonchan.xyz有DNS污染迹象，请注意迁移到以下节点</p>
			<p style="color: black;">New:<a href="https://ex.810114.xyz/">https://ex.810114.xyz/</a>（无污染永续）</p>			
			<p style="color: black;">新年快乐，更新一下程序，所以1月1日可能麻烦大家挡一下小白鼠</p>			
			<p><a style="color: black;" href="/uconfig.php">点击上方Settings（点这句话也可以）选择希望开启的脚本</a></p>
			<p><a style="color: black;" href="/uconfig.php">有的脚本没做是因为有技术限制，有想要的tamper monkey脚本可以发一下做fork，来https://810114.xyz/ (去掉ex就行)</a></p>
		</div>
    </div>

    <script>
		const mark = '1228';
        // 检查 localStorage 中的值
        if (localStorage.getItem('iframeClosed') !== mark) {
            document.getElementById('moonchan-floating-iframe').style.display = 'block';
        }

        function moonchanCloseIframe() {
            const iframeContainer = document.getElementById('moonchan-floating-iframe');
            iframeContainer.style.display = 'none'; // 隐藏 iframe
            localStorage.setItem('iframeClosed', mark); // 设置 localStorage 标记
        }
    </script>

`), 1)
	return html
}
