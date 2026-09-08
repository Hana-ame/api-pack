// 26.09.07
// bilibili 字幕下载：优先中文，fallback 英文。
//
// 流程:
//   1. view API -> 拿 cid
//   2. player/v2 API -> 拿字幕列表（需要 SESSDATA cookie）
//   3. 选首选语言（中文 > 英文 > 其他）
//   4. 下载字幕 JSON -> 转 VTT
//
// Cookie: 从 Netscape HTTP Cookie File 解析（格式同 curl / yt-dlp 导出）。
// 默认路径 /mnt/c/Users/lumin/Downloads/cookies.txt（WSL 环境），
// 也可用环境变量 BILIBILI_COOKIES_FILE 覆盖。
package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tools "github.com/Hana-ame/api-pack/tools/utils"
)

const (
	// playerAPI 字幕信息接口（需要 SESSDATA cookie）
	playerAPI = "https://api.bilibili.com/x/player/v2"

	subtitleCacheSize = 128
	subtitleTimeout   = 15 * time.Second
)

var subtitleCache = tools.NewLRUCache[string, *SubtitleResult](subtitleCacheSize)

// ===================== Cookie 解析 =====================

// ParseNetscapeCookieFile 解析 Netscape HTTP Cookie File 格式。
// 格式: domain\tflag\tpath\tsecure\texpiration\tname\tvalue
// 返回 domain -> (name -> value) 的二级 map。
func ParseNetscapeCookieFile(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookie file: %w", err)
	}
	defer f.Close()

	result := make(map[string]map[string]string)

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024) // 1MB 缓冲，某些 cookie 值很长
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "\t", 7)
		if len(parts) < 7 {
			continue
		}

		domain := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[5])
		value := strings.TrimSpace(parts[6])

		if domain == "" || name == "" {
			continue
		}

		if result[domain] == nil {
			result[domain] = make(map[string]string)
		}
		result[domain][name] = value
	}

	return result, scanner.Err()
}

// buildBiliCookieHeader 从解析后的 cookie map 中筛选 bilibili 相关的 cookie，
// 拼成 Cookie header 字符串。
func buildBiliCookieHeader(allCookies map[string]map[string]string) string {
	// 先判断是否 bilibili 域（含子域）
	isBiliDomain := func(domain string) bool {
		d := strings.ToLower(domain)
		return strings.HasSuffix(d, "bilibili.com")
	}

	// bilibili 域相关的关键 cookie 名
	biliNames := map[string]bool{
		"SESSDATA":              true,
		"bili_jct":              true,
		"buvid3":                true,
		"buvid4":                true,
		"DedeUserID":            true,
		"DedeUserID__ckMd5":     true,
		"b_lsid":                true,
		"theme_style":           true,
		"lang":                  true,
		"rpdid":                 true,
		"CURRENT_QUALITY":       true,
		"CURRENT_FNVAL":         true,
		"bili_ticket":           true,
		"bili_ticket_expires":   true,
	}

	var pairs []string
	seen := make(map[string]bool)

	for domain, cookies := range allCookies {
		if !isBiliDomain(domain) {
			continue
		}
		for name, value := range cookies {
			if !biliNames[name] || seen[name] {
				continue
			}
			pairs = append(pairs, name+"="+value)
			seen[name] = true
		}
	}

	return strings.Join(pairs, "; ")
}

// biliCookie 全局 cookie（惰性加载）
var (
	biliCookie   string
	biliCookieMu sync.Once
	biliCookieErr error
)

// getBiliCookie 获取 bilibili Cookie header 字符串。
// 惰性加载：第一次调用时从文件解析，之后缓存。
// 必须设置环境变量 BILIBILI_COOKIES_FILE 指向 Netscape 格式 cookie 文件，
// 否则返回错误。
func getBiliCookie() (string, error) {
	biliCookieMu.Do(func() {
		path := os.Getenv("BILIBILI_COOKIES_FILE")
		if path == "" {
			biliCookieErr = errors.New("BILIBILI_COOKIES_FILE not set; set it to a Netscape-format cookies.txt path")
			return
		}

		allCookies, err := ParseNetscapeCookieFile(path)
		if err != nil {
			biliCookieErr = err
			return
		}

		biliCookie = buildBiliCookieHeader(allCookies)
		if biliCookie == "" {
			biliCookieErr = fmt.Errorf("no bilibili cookies found in %s", path)
		}
	})
	return biliCookie, biliCookieErr
}

// ===================== 字幕数据结构 =====================

// SubtitleInfo 单个字幕的语言信息（player API 返回）
type SubtitleInfo struct {
	Lan              string `json:"lan"`
	LanDoc           string `json:"lan_doc"`
	SubtitleURL      string `json:"subtitle_url"`
	SubtitleURLBackup string `json:"subtitle_url_backup"`
}

// SubtitleLine 字幕 JSON 中的一行
type SubtitleLine struct {
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Content  string  `json:"content"`
	Location int     `json:"location"`
}

// SubtitleResult 字幕下载结果
type SubtitleResult struct {
	Bvid      string           `json:"bvid"`
	Title     string           `json:"title"`
	Lang      string           `json:"lang"`
	LangDoc   string           `json:"lang_doc"`
	Format    string           `json:"format"` // "vtt"
	VTT       string           `json:"vtt"`
	Lines     []SubtitleLine   `json:"lines"`
	Available []SubtitleInfo   `json:"available"`
}

// playerResp x/player/v2 响应
type playerResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Subtitle struct {
			Subtitles []SubtitleInfo `json:"subtitles"`
		} `json:"subtitle"`
	} `json:"data"`
}

// subtitleJSON 字幕 JSON 文件的结构
type subtitleJSON struct {
	Body []SubtitleLine `json:"body"`
}

// ===================== 核心逻辑 =====================

// subtitleClient 带 cookie 的 HTTP 客户端
var subtitleClient = &http.Client{
	Timeout: subtitleTimeout,
	Transport: http.DefaultClient.Transport,
}

// fetchSubtitleList 从 player API 获取字幕列表
func fetchSubtitleList(bvid string, aid int64, cid int64) ([]SubtitleInfo, error) {
	target := playerAPI + "?bvid=" + url.QueryEscape(bvid) +
		"&cid=" + strconv.FormatInt(cid, 10) +
		"&aid=" + strconv.FormatInt(aid, 10)

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvid)

	// 添加 Cookie
	cookie, err := getBiliCookie()
	if err != nil {
		return nil, fmt.Errorf("cookie load: %w", err)
	}
	req.Header.Set("Cookie", cookie)

	resp, err := subtitleClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("player API status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var b playerResp
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("decode player response: %w", err)
	}
	if b.Code != 0 {
		return nil, fmt.Errorf("bilibili player code %d: %s", b.Code, b.Message)
	}

	return b.Data.Subtitle.Subtitles, nil
}

// downloadSubtitleJSON 下载字幕 JSON 并解析
func downloadSubtitleJSON(subtitleURL string) ([]SubtitleLine, error) {
	// subtitle_url 有时是相对路径，补全
	if !strings.HasPrefix(subtitleURL, "http://") && !strings.HasPrefix(subtitleURL, "https://") {
		if strings.HasPrefix(subtitleURL, "//") {
			subtitleURL = "https:" + subtitleURL
		} else if strings.HasPrefix(subtitleURL, "/") {
			subtitleURL = "https://aisubtitle.bilibili.com" + subtitleURL
		}
	}

	req, err := http.NewRequest(http.MethodGet, subtitleURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.bilibili.com/")

	cookie, err := getBiliCookie()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookie)

	resp, err := subtitleClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subtitle download status %d: %s", resp.StatusCode, subtitleURL)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}

	var sub subtitleJSON
	if err := json.Unmarshal(raw, &sub); err != nil {
		return nil, fmt.Errorf("decode subtitle json: %w", err)
	}

	return sub.Body, nil
}

// pickSubtitle 按偏好选择字幕语言：中文优先，fallback 英文。
// 返回选中的 SubtitleInfo 指针。
func pickSubtitle(subs []SubtitleInfo) *SubtitleInfo {
	if len(subs) == 0 {
		return nil
	}

	// 语言优先级：中文(0) > 英文(1) > 其他(2)
	langRank := func(lang string) int {
		l := strings.ToLower(lang)
		if strings.HasPrefix(l, "zh") {
			return 0
		}
		if strings.HasPrefix(l, "en") {
			return 1
		}
		return 2
	}

	best := &subs[0]
	bestRank := langRank(best.Lan)

	for i := 1; i < len(subs); i++ {
		r := langRank(subs[i].Lan)
		if r < bestRank {
			best = &subs[i]
			bestRank = r
		}
	}

	return best
}

// downloadSubtitleWithFallback 下载字幕，失败时尝试备用 URL
func downloadSubtitleWithFallback(sub *SubtitleInfo) ([]SubtitleLine, error) {
	// 先试主 URL
	if sub.SubtitleURL != "" {
		lines, err := downloadSubtitleJSON(sub.SubtitleURL)
		if err == nil && len(lines) > 0 {
			return lines, nil
		}
	}

	// 试备用 URL
	if sub.SubtitleURLBackup != "" {
		lines, err := downloadSubtitleJSON(sub.SubtitleURLBackup)
		if err == nil && len(lines) > 0 {
			return lines, nil
		}
	}

	return nil, errors.New("all subtitle URLs failed for lang " + sub.Lan)
}

// ===================== VTT 转换 =====================

// toVTT 将字幕行转换为 WebVTT 格式
func toVTT(lines []SubtitleLine) string {
	sorted := make([]SubtitleLine, len(lines))
	copy(sorted, lines)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].From < sorted[j].From
	})

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	for i, line := range sorted {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(formatVTTTime(line.From))
		sb.WriteString(" --> ")
		sb.WriteString(formatVTTTime(line.To))
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(line.Content))
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// formatVTTTime 将秒数转为 VTT 时间格式 HH:MM:SS.mmm
func formatVTTTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMs := int(math.Round(seconds * 1000))
	h := totalMs / 3600000
	m := (totalMs % 3600000) / 60000
	s := (totalMs % 60000) / 1000
	ms := totalMs % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// ===================== 主入口 =====================

// GetSubtitle 获取字幕（主入口）。
// 优先中文，fallback 英文。返回 VTT 格式字幕内容。
func GetSubtitle(input string) (*SubtitleResult, error) {
	info, err := GetInfo(input)
	if err != nil {
		return nil, err
	}

	if info.Cid == 0 {
		return nil, errors.New("cid is 0, cannot fetch subtitles")
	}

	// 缓存 key
	key := info.Bvid
	if key == "" {
		key = "aid:" + strconv.FormatInt(info.Aid, 10)
	}
	if cached, ok := subtitleCache.Get(key); ok {
		return cached, nil
	}

	// 获取字幕列表
	subs, err := fetchSubtitleList(info.Bvid, info.Aid, info.Cid)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return nil, errors.New("no subtitles available for " + info.Bvid)
	}

	// 选择最佳语言
	best := pickSubtitle(subs)
	if best == nil {
		return nil, errors.New("no matching subtitle found")
	}

	// 下载字幕
	lines, err := downloadSubtitleWithFallback(best)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("subtitle content is empty")
	}

	// 转 VTT
	vtt := toVTT(lines)

	result := &SubtitleResult{
		Bvid:      info.Bvid,
		Title:     info.Title,
		Lang:      best.Lan,
		LangDoc:   best.LanDoc,
		Format:    "vtt",
		VTT:       vtt,
		Lines:     lines,
		Available: subs,
	}

	subtitleCache.Put(key, result)
	return result, nil
}
