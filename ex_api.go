// 26.01.11
// 并不使用这里的代码

// gimini, 不想弄了
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gin-gonic/gin"
)

// --- 配置区域 ---
const (
	UserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	ServerPort      = ":8080"
	CacheCleanCycle = 1 * time.Hour
	CacheTTL        = 24 * time.Hour
	HomeRefreshTime = 2 * time.Minute
	HomeRefreshHits = 10
)

// --- 缓存结构 ---
type HomeQuotaCache struct {
	sync.RWMutex
	Current   int
	Limit     int
	LastFetch time.Time
	HitCount  int
}

type GalleryTimeCache struct {
	sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	PostedTime time.Time
	ExpiresAt  time.Time
}

var (
	quotaCache   = &HomeQuotaCache{}
	galleryCache = &GalleryTimeCache{items: make(map[string]cacheItem)}
	httpClient   = &http.Client{
		Timeout: 30 * time.Second,
		// 关键：禁止自动跟随重定向，我们需要捕获 302
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

func exApi() {
	go startCacheCleaner()

	r := gin.New()
	r.Use(gin.Recovery())

	// 修改点 1: 使用通配符匹配 /fullimg/xxxx 及其子路径
	r.GET("/fullimg/*path", handleFullImg)

	log.Printf("ExHentai 智能代理已启动，监听 %s", ServerPort)
	log.Printf("支持 URL 格式: /fullimg/gid/page/key/filename")
	if err := r.Run(ServerPort); err != nil {
		log.Fatal(err)
	}
}

func handleFullImg(c *gin.Context) {
	// 获取 URL 路径参数，例如 "/3704709/1/qfzwv4qaio6/1_1.jpg"
	path := c.Param("path")

	// 1. Cookie 检查
	cookie := getCookie(c)
	if cookie == "" {
		c.String(http.StatusForbidden, "Missing Cookie")
		return
	}

	// 2. 配额检查 (Home.php)
	if err := checkQuotaSmart(cookie); err != nil {
		log.Printf("[Quota] 配额拦截: %v", err)
		c.String(http.StatusForbidden, "Quota Error: %v", err)
		return
	}

	// 3. 时间规则检查 (必须依赖 Referer)
	referer := c.GetHeader("Referer")
	// 简单的 Referer 校验，必须包含 /s/ (图片详情页)
	if referer == "" || !strings.Contains(referer, "/s/") {
		c.String(http.StatusForbidden, "Refusing to process without '/s/' Referer (Cannot verify gallery time)")
		return
	}

	if err := verifyTimeRule(referer, cookie); err != nil {
		log.Printf("[Rule] 规则拦截: %v | Referer: %s", err, referer)
		c.String(http.StatusForbidden, "Time Rule Block: %v", err)
		return
	}

	// 4. 构造上游请求
	// 注意：e-hentai 的 fullimg url 结构是 https://e-hentai.org/fullimg/gid/page/key/filename
	targetURL := "https://e-hentai.org/fullimg" + path // 也可以是用 exhentai.org

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Req Error: %v", err)
		return
	}

	// 5. 必须透传 Header
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Referer", referer)

	// 6. 发起请求
	resp, err := httpClient.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "Upstream Error: %v", err)
		return
	}
	defer resp.Body.Close()

	// 7. 处理响应 (核心修改点)
	// 我们期望 302，但也可能遇到 403, 404
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusSeeOther {
		location := resp.Header.Get("Location")
		if location != "" {
			// 修改点 2: 透传 Set-Cookie (如果 ExHentai 更新了 Session)
			if sc := resp.Header.Get("Set-Cookie"); sc != "" {
				c.Header("Set-Cookie", sc)
			}

			// 修改点 3: 透传 Content-Disposition (保留文件名提示)
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				c.Header("Content-Disposition", cd)
			}

			// 禁止缓存这个跳转链接
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Expires", "0")

			// 返回 302 给客户端
			c.Redirect(http.StatusFound, location)
			return
		}
	}

	// 如果状态码不是 302，说明可能出错了 (例如 IP 被 Ban 返回了 200 的验证码页面，或者 404)
	c.String(resp.StatusCode, "Upstream returned status %d (Expected 302). Headers: %v", resp.StatusCode, resp.Header)
}

// --- 业务逻辑：溯源与时间判定 ---

func verifyTimeRule(sPageURL string, cookie string) error {
	// 1. 通过 Referer (/s/ URL) 获取 画廊 URL
	galleryURL, err := fetchGalleryURLFromSPage(sPageURL, cookie)
	if err != nil {
		return fmt.Errorf("failed to identify gallery: %v", err)
	}

	// 2. 获取画廊发布时间
	postedTime, err := getGalleryPostedTime(galleryURL, cookie)
	if err != nil {
		return fmt.Errorf("failed to get posted time: %v", err)
	}

	// 3. 规则判定
	return checkExHentaiRule(postedTime)
}

func checkExHentaiRule(postedTime time.Time) error {
	now := time.Now().UTC()
	months := now.Sub(postedTime).Hours() / 24 / 30

	if months < 3 {
		return nil // 安全
	}
	if months >= 3 && months < 12 {
		if isPeakHour() {
			return fmt.Errorf("peak hours (UTC %d) block for 3-12m gallery", now.Hour())
		}
		return nil // 安全
	}
	// > 1年
	return fmt.Errorf("gallery too old (>1y), requires GP")
}

func isPeakHour() bool {
	now := time.Now().UTC()
	h := now.Hour()
	w := now.Weekday()
	if w == time.Sunday {
		return h >= 5 && h < 20
	}
	// Mon-Sat: 14:00 - 20:00 UTC
	return h >= 14 && h < 20
}

// --- 辅助：HTML 解析与缓存 ---

// 获取 /s/ 页面中的 /g/ 链接
func fetchGalleryURLFromSPage(url string, cookie string) (string, error) {
	// 这里如果不缓存，每次都要请求一次 /s/ 页面，比较费流量。
	// 但由于 /s/ 页面里包含 hash，难以简单复用，暂时保持直接请求。
	resp, err := fetchWithUA(url, cookie, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return "", err
	}

	// 寻找 "Return To Gallery" 链接
	node := htmlquery.FindOne(doc, "//div[@class='sb']/a[contains(@href, '/g/')]")
	if node == nil {
		// 有时候可能 layout 不同，或者 IP 被 ban 显示了 sad panda
		return "", errors.New("gallery link not found (IP might be banned or layout changed)")
	}
	return htmlquery.SelectAttr(node, "href"), nil
}

// 获取画廊发布时间 (带缓存)
func getGalleryPostedTime(galleryURL string, cookie string) (time.Time, error) {
	galleryCache.RLock()
	item, exists := galleryCache.items[galleryURL]
	galleryCache.RUnlock()

	if exists && time.Now().Before(item.ExpiresAt) {
		return item.PostedTime, nil
	}

	resp, err := fetchWithUA(galleryURL, cookie, "")
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return time.Time{}, err
	}

	timeNode := htmlquery.FindOne(doc, `//*[@id="gdd"]/table//tr[1]/td[2]`)
	if timeNode == nil {
		return time.Time{}, errors.New("posted date not found")
	}

	layout := "2006-01-02 15:04"
	t, err := time.Parse(layout, htmlquery.InnerText(timeNode))
	if err != nil {
		return time.Time{}, err
	}

	galleryCache.Lock()
	galleryCache.items[galleryURL] = cacheItem{
		PostedTime: t,
		ExpiresAt:  time.Now().Add(CacheTTL),
	}
	galleryCache.Unlock()

	return t, nil
}

// --- 辅助：配额管理 ---

func checkQuotaSmart(cookie string) error {
	quotaCache.Lock()
	defer quotaCache.Unlock()

	needsRefresh := quotaCache.Limit == 0 ||
		time.Since(quotaCache.LastFetch) > HomeRefreshTime ||
		quotaCache.HitCount >= HomeRefreshHits

	if needsRefresh {
		current, limit, err := fetchHomeQuota(cookie)
		if err != nil {
			// 刷新失败，记录日志但返回错误，安全起见不放行
			return err
		}
		quotaCache.Current = current
		quotaCache.Limit = limit
		quotaCache.LastFetch = time.Now()
		quotaCache.HitCount = 0
	}

	quotaCache.HitCount++
	if quotaCache.Limit > 0 && quotaCache.Current >= quotaCache.Limit {
		return fmt.Errorf("quota exceeded: %d/%d", quotaCache.Current, quotaCache.Limit)
	}
	return nil
}

func fetchHomeQuota(cookie string) (int, int, error) {
	// 使用 exhentai.org 还是 e-hentai.org 取决于 cookie 权限，通常 e-hentai 通用
	resp, err := fetchWithUA("https://e-hentai.org/home.php", cookie, "")
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return 0, 0, err
	}

	nodes := htmlquery.Find(doc, "//div[@class='homebox']//strong")
	if len(nodes) >= 2 {
		curr, _ := strconv.Atoi(htmlquery.InnerText(nodes[0]))
		limit, _ := strconv.Atoi(htmlquery.InnerText(nodes[1]))
		return curr, limit, nil
	}
	return 0, 0, errors.New("cannot parse quota")
}

// --- 基础工具 ---

func fetchWithUA(urlStr, cookie, referer string) (*http.Response, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cookie", cookie)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return httpClient.Do(req)
}

func getCookie(c *gin.Context) string {
	cookie := c.GetHeader("Cookie")
	if cookie == "" {
		cookie = os.Getenv("EXHENTAI_PROXY_COOKIE")
	}
	return cookie
}

func startCacheCleaner() {
	ticker := time.NewTicker(CacheCleanCycle)
	for range ticker.C {
		galleryCache.Lock()
		now := time.Now()
		for k, v := range galleryCache.items {
			if now.After(v.ExpiresAt) {
				delete(galleryCache.items, k)
			}
		}
		galleryCache.Unlock()
	}
}
