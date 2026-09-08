// 26.09.05
// bilibili 封面获取。
//
// 输入形式（ExtractID 统一处理）:
//   - b23.tv 短链        https://b23.tv/G2o7PT6
//   - 完整视频地址       https://www.bilibili.com/video/BV15ntz6gEHd?...
//   - 裸 BV 号           BV15ntz6gEHd
//   - 裸 aid             117215445649383
//
// 返回 data.pic 即封面直链（i*.hdslb.com/bfs/archive/*.jpg，可直接访问，无需签名）。
//
// 流程: 解短链 -> 提取 BV/aid -> x/web-interface/view -> pic
//
// 重定向: 需要手动跟。main.go 里 http.DefaultClient 的 CheckRedirect 返回
// ErrUseLastResponse(301/302 原样交给客户端); 但测试进程里 DefaultClient 是 Go
// 默认客户端, 会自动跟到底, 所以这里单独用 noRedirectClient。
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	tools "github.com/Hana-ame/api-pack/tools/utils"
)

const (
	// view API: 公开接口, 无需登录/cookie/签名
	viewAPI = "https://api.bilibili.com/x/web-interface/view"

	userAgent = "Mozilla/5.0 (compatible; api-pack/1.0; +https://moonchan.xyz)"

	maxRedirects = 8
	cacheSize    = 4096
)

var (
	bvID    = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)
	bvFind  = regexp.MustCompile(`BV[0-9A-Za-z]{10}`)
	shortTV = regexp.MustCompile(`^https?://(www\.)?b23\.tv/[^\s]+$`)
	digits  = regexp.MustCompile(`^\d+$`)

	infoCache = tools.NewLRUCache[string, *BiliInfo](cacheSize)
)

// noRedirectClient 不自动跟随重定向。不能直接用 http.DefaultClient:
// 在测试进程里 DefaultClient 是 Go 默认客户端, 会把 302 跟到底, 拿不到 Location;
// main.go 里虽然显式设置了 CheckRedirect 返回 ErrUseLastResponse, 但这里自己声明更稳。
var noRedirectClient = &http.Client{
	Timeout:       20 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport:     http.DefaultTransport,
}

// Page 分 P 信息（用于多 P 视频的首帧）
type Page struct {
	Page       int    `json:"page"`
	Part       string `json:"part"`
	Duration   int    `json:"duration"`
	FirstFrame string `json:"first_frame,omitempty"`
}

// BiliInfo 视频元数据（只保留常用字段）
type BiliInfo struct {
	Bvid     string `json:"bvid"`
	Aid      int64  `json:"aid"`
	Cid      int64  `json:"cid"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Pic      string `json:"pic"` // 封面直链（已转 https）
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Pubdate  int64  `json:"pubdate"`
	Owner    string `json:"owner"`
	OwnerID  int64  `json:"owner_id"`
	View     int64  `json:"view"`
	Like     int64  `json:"like"`
	Favorite int64  `json:"favorite"`
	Reply    int64  `json:"reply"`
	Pages    []Page `json:"pages,omitempty"`
}

// viewResp x/web-interface/view 的原始响应
type viewResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Bvid     string `json:"bvid"`
		Aid      int64  `json:"aid"`
		Cid      int64  `json:"cid"`
		Title    string `json:"title"`
		Desc     string `json:"desc"`
		Pic      string `json:"pic"`
		Duration int    `json:"duration"`
		Pubdate  int64  `json:"pubdate"`
		Owner    struct {
			Mid  int64  `json:"mid"`
			Name string `json:"name"`
		} `json:"owner"`
		Dimension struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"dimension"`
		Stat struct {
			View     int64 `json:"view"`
			Like     int64 `json:"like"`
			Favorite int64 `json:"favorite"`
			Reply    int64 `json:"reply"`
		} `json:"stat"`
		Pages []struct {
			Page       int    `json:"page"`
			Part       string `json:"part"`
			Duration   int    `json:"duration"`
			FirstFrame string `json:"first_frame"`
		} `json:"pages"`
	} `json:"data"`
}

// ExtractID 从任意输入中提取 BV 号或 aid。
// 二者至少有一个非空。b23.tv 短链需要发一次 HTTP 请求解重定向。
func ExtractID(input string) (bvid string, aid int64, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", 0, errors.New("input is empty")
	}

	if bvID.MatchString(input) {
		return input, 0, nil
	}
	if digits.MatchString(input) {
		n, e := strconv.ParseInt(input, 10, 64)
		if e != nil {
			return "", 0, e
		}
		return "", n, nil
	}

	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		if m := bvFind.FindString(input); m != "" {
			return m, 0, nil
		}
		return "", 0, fmt.Errorf("unrecognized input: %s", input)
	}

	// 短链: 跟重定向拿到最终地址
	if shortTV.MatchString(input) {
		followed, e := followRedirects(input)
		if e != nil {
			return "", 0, e
		}
		input = followed
	}

	if m := bvFind.FindString(input); m != "" {
		return m, 0, nil
	}

	// 兜底: 从 query 里找 aid（部分分享地址只有 aid）
	if u, e := url.Parse(input); e == nil {
		if a := u.Query().Get("aid"); digits.MatchString(a) {
			if n, e2 := strconv.ParseInt(a, 10, 64); e2 == nil {
				return "", n, nil
			}
		}
	}

	return "", 0, fmt.Errorf("BV id not found in %s", input)
}

// followRedirects 手动跟 301/302/303/307/308（用 noRedirectClient, 不依赖全局配置）。
// 返回最后一个 URL；只发 GET 探测, 不读取响应体。
func followRedirects(target string) (string, error) {
	current := target
	for i := 0; i < maxRedirects; i++ {
		req, err := http.NewRequest(http.MethodGet, current, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := noRedirectClient.Do(req)
		if err != nil {
			return "", err
		}

		status := resp.StatusCode
		loc := resp.Header.Get("Location")
		resp.Body.Close()

		// resp.Request.URL 始终反映本次请求实际打到的地址
		current = resp.Request.URL.String()

		if status < 300 || status >= 400 {
			return current, nil
		}
		if loc == "" {
			return "", fmt.Errorf("redirect %d without Location at %s", status, current)
		}

		base, err := url.Parse(current)
		if err != nil {
			return "", err
		}
		next, err := url.Parse(loc)
		if err != nil {
			return "", err
		}
		current = base.ResolveReference(next).String()
	}
	return "", fmt.Errorf("too many redirects (>%d) from %s", maxRedirects, target)
}

// GetInfo 按输入拿视频信息（含封面），带 LRU 缓存。
func GetInfo(input string) (*BiliInfo, error) {
	bvid, aid, err := ExtractID(input)
	if err != nil {
		return nil, err
	}

	key := bvid
	if key == "" {
		key = "aid:" + strconv.FormatInt(aid, 10)
	}
	if cached, ok := infoCache.Get(key); ok {
		return cached, nil
	}

	info, err := fetchView(bvid, aid)
	if err != nil {
		return nil, err
	}

	infoCache.Put(key, info)
	return info, nil
}

// fetchView 调用 x/web-interface/view
func fetchView(bvid string, aid int64) (*BiliInfo, error) {
	target := viewAPI + "?"
	if bvid != "" {
		target += "bvid=" + url.QueryEscape(bvid)
	} else {
		target += "aid=" + strconv.FormatInt(aid, 10)
	}

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.bilibili.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var b viewResp
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("decode view response: %w", err)
	}

	if b.Code != 0 {
		return nil, fmt.Errorf("bilibili view code %d: %s", b.Code, b.Message)
	}

	d := b.Data
	info := &BiliInfo{
		Bvid:     d.Bvid,
		Aid:      d.Aid,
		Cid:      d.Cid,
		Title:    d.Title,
		Desc:     d.Desc,
		Pic:      toHTTPS(d.Pic),
		Duration: d.Duration,
		Width:    d.Dimension.Width,
		Height:   d.Dimension.Height,
		Pubdate:  d.Pubdate,
		Owner:    d.Owner.Name,
		OwnerID:  d.Owner.Mid,
		View:     d.Stat.View,
		Like:     d.Stat.Like,
		Favorite: d.Stat.Favorite,
		Reply:    d.Stat.Reply,
	}

	for _, p := range d.Pages {
		info.Pages = append(info.Pages, Page{
			Page:       p.Page,
			Part:       p.Part,
			Duration:   p.Duration,
			FirstFrame: toHTTPS(p.FirstFrame),
		})
	}
	return info, nil
}

// toHTTPS i*.hdslb.com 支持 https, 统一升级避免 mixed content
func toHTTPS(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "http" && strings.Contains(u.Host, "hdslb.com") {
		u.Scheme = "https"
		return u.String()
	}
	return raw
}
