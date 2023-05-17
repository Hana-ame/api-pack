package exproxy

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"

	"github.com/andybalholm/brotli"
)

const (
	TRUE_HOST = "exhentai.org"
	COOKIE    = `ipb_member_id=4761956; ipb_pass_hash=16f4dc00b025b2e51f59e2a2365d4490; yay=louder; igneous=fb28508a0; sl=dm_1`
)

var host map[string]([]string)
var cookie string
var trueHost string
var ipArr []string
var re *regexp.Regexp

var client *http.Client = func() *http.Client {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
		Jar:       jar,
	}
}()

func Main(listen string) {

	re = regexp.MustCompile(`Domain=[\w\.]*;`)
	host = make(map[string][]string)
	cookie = COOKIE
	trueHost = TRUE_HOST
	// listen := "127.111.111.113:8080"

	host[trueHost] = []string{
		"178.175.129.252",
		"178.175.129.254",
		"178.175.128.252",
		"178.175.128.254",
		"178.175.132.20",
		"178.175.132.22",
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/", httpHandler(trueHost))
	server := &http.Server{Addr: listen, Handler: handler}

	err := server.ListenAndServe()

	log.Println(err)
}

// return the proxy function
func httpHandler(trueHost string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		makeRequestWithoutSNI(w, r, trueHost)
	}
}
func getIPLocaly(s string) string {
	return host[s][rand.Intn(len(host[s]))]
}
func getPlainTextReader(body io.ReadCloser, encoding string) io.ReadCloser {
	switch encoding {
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			log.Fatal("error decoding gzip response", reader)
		}
		return reader
	case "br":
		reader := brotli.NewReader(body)
		if reader == nil {
			log.Fatal("error decoding br response", reader)
		}
		return io.NopCloser(reader)
	default:
		return body
	}
}
func makeRequestWithoutSNI(w http.ResponseWriter, r *http.Request, trueHost string) []byte {
	// tr := &http.Transport{
	// 	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	// }
	// client := &http.Client{
	// 	CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// 		return http.ErrUseLastResponse
	// 	},
	// 	Transport: tr,
	// }

	newUrl := r.URL
	newUrl.Host = getIPLocaly(trueHost) // IP
	newUrl.Scheme = "https"

	// the search is done by get,
	// so only use get for prevent post with the provided account
	req, err := http.NewRequest("GET", newUrl.String(), nil)
	if err != nil {
		fmt.Println(`Error On NewRequest`, err)
		return nil
	}
	// it seems that override the request here...
	req.Header = r.Header
	req.Method = r.Method // wait?
	req.Body = r.Body     //
	req.Host = trueHost   //Host
	req.Header.Set("Cookie", cookie)

	// never mind...
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(`Error On Do Request`, err)
		return nil
	}
	defer resp.Body.Close()

	// fmt.Println(resp.Header.Get("Content-Encoding"))
	// fmt.Println(resp.Header.Get("Content-Type"))

	// fmt.Println(resp.Header)

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") ||
		strings.HasPrefix(contentType, "application/javascript") ||
		strings.HasPrefix(contentType, "text/css") {

		body := getPlainTextReader(resp.Body, resp.Header.Get("Content-Encoding"))
		text, err := io.ReadAll(body)
		if err != nil {
			fmt.Println(`Error On Read Body`, err)
			return nil
		}

		textReplaced := strings.Replace(string(text), "https://exhentai.org/", "/", -1)
		textReplaced = strings.Replace(textReplaced, "https://s.exhentai.org/", "https://s-ex.moonchan.xyz/", -1)
		if trueHost == "exhentai.org" && strings.HasPrefix(r.URL.Path, `/s/`) {
			textReplaced = addWaterFallViewButton(textReplaced)
		}

		for k, v := range resp.Header {
			if k == "Content-Length" {
				continue
			}
			for _, vi := range v {
				w.Header().Add(k, vi)
			}
		}
		w.Header().Del("Content-Encoding")

		// w.WriteHeader(http.StatusOK)
		w.Write([]byte(textReplaced))

	} else {
		for k, v := range resp.Header {
			if k == "Content-Length" {
				continue
			}
			for _, vi := range v {
				if k == "Set-Cookie" {
					vi = string(re.ReplaceAll([]byte(vi), []byte{}))
				}
				w.Header().Add(k, vi)
			}
		}

		// w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
	}

	return nil
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
		下拉式
	  </button>
	</div>
  <script type="text/javascript">
	async function execWaterfall(){
		console.log('!');
		document.getElementById("waterfall").remove();
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
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	</script>`, 1)
}
