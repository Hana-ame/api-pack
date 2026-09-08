// 26.09.07
// 字幕下载功能的单元测试 + 集成测试
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ===================== 字幕页面测试 =====================

func TestServeSubtitleIndex(t *testing.T) {
	r := testRouter()

	for _, path := range []string{"/api/v2/bili/subtitle.html", "/api/v2/bili/ui/subtitle"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content-type = %q, want text/html", path, ct)
		}
		body := w.Body.String()
		if !strings.Contains(body, "哔哩哔哩字幕下载") {
			t.Errorf("%s: page missing title", path)
		}
		// __API_BASE__ 必须被替换成真实前缀
		if strings.Contains(body, "__API_BASE__") {
			t.Errorf("%s: __API_BASE__ not replaced", path)
		}
		if !strings.Contains(body, `let BASE = "/api/v2/bili";`) {
			t.Errorf("%s: BASE not set to prefix", path)
		}
		for _, must := range []string{"/subtitle?", "extractBvids", "fetchSubtitle"} {
			if !strings.Contains(body, must) {
				t.Errorf("%s: page missing %s", path, must)
			}
		}
	}
}

// ===================== Cookie 解析测试 =====================

func TestParseNetscapeCookieFile(t *testing.T) {
	// 创建临时 cookie 文件（Netscape 格式）
	content := `# Netscape HTTP Cookie File
# This is a comment

.bilibili.com	TRUE	/	TRUE	1804167517	SESSDATA	bd6657ca%2C1804167523%2C67c9a
.bilibili.com	TRUE	/	TRUE	1804167517	bili_jct	c03a557810b10350a027681b4a8ceaca
.bilibili.com	TRUE	/	TRUE	1804167517	DedeUserID	12345678
.bilibili.com	TRUE	/	TRUE	1804167517	buvid3	abcdef123456
.bilibili.com	TRUE	/	FALSE	1804167517	theme_style	dark
www.bilibili.com	TRUE	/	TRUE	1804167517	CURRENT_QUALITY	1080P
other.com	TRUE	/	TRUE	1804167517	other_cookie	ignored
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cookies.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	allCookies, err := ParseNetscapeCookieFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// 检查 bilibili.com 域
	biliCookies, ok := allCookies[".bilibili.com"]
	if !ok {
		t.Fatal("missing .bilibili.com domain")
	}

	if biliCookies["SESSDATA"] == "" {
		t.Error("SESSDATA not parsed")
	}
	if biliCookies["bili_jct"] != "c03a557810b10350a027681b4a8ceaca" {
		t.Errorf("bili_jct mismatch: %s", biliCookies["bili_jct"])
	}
	if biliCookies["DedeUserID"] != "12345678" {
		t.Errorf("DedeUserID mismatch: %s", biliCookies["DedeUserID"])
	}

	// 检查子域
	subCookies, ok := allCookies["www.bilibili.com"]
	if !ok {
		t.Fatal("missing www.bilibili.com domain")
	}
	if subCookies["CURRENT_QUALITY"] != "1080P" {
		t.Errorf("CURRENT_QUALITY mismatch: %s", subCookies["CURRENT_QUALITY"])
	}

	// 检查非 bilibili 域
	if _, ok := allCookies["other.com"]; !ok {
		t.Error("other.com should still be parsed (just not selected later)")
	}
}

func TestBuildBiliCookieHeader(t *testing.T) {
	allCookies := map[string]map[string]string{
		".bilibili.com": {
			"SESSDATA":    "test_sessdata_value",
			"bili_jct":    "test_bili_jct",
			"DedeUserID":  "12345",
			"buvid3":      "abcdef",
			"theme_style": "dark",
			"lang":        "zh-Hans",
		},
		"www.bilibili.com": {
			"CURRENT_QUALITY": "1080P",
			"CURRENT_FNVAL":   "4048",
		},
		"other.com": {
			"SESSDATA": "should_not_appear",
		},
	}

	header := buildBiliCookieHeader(allCookies)

	if !strings.Contains(header, "SESSDATA=test_sessdata_value") {
		t.Errorf("header missing SESSDATA: %s", header)
	}
	if !strings.Contains(header, "bili_jct=test_bili_jct") {
		t.Error("header missing bili_jct")
	}
	if !strings.Contains(header, "DedeUserID=12345") {
		t.Error("header missing DedeUserID")
	}
	if !strings.Contains(header, "CURRENT_QUALITY=1080P") {
		t.Error("header missing CURRENT_QUALITY")
	}

	// 其他域的 SESSDATA 不应出现
	if strings.Contains(header, "should_not_appear") {
		t.Error("header should not contain other.com's SESSDATA")
	}

	// 检查分隔符
	if !strings.Contains(header, "; ") {
		t.Error("cookies should be separated by '; '")
	}
}

func TestBuildBiliCookieHeaderNoBili(t *testing.T) {
	allCookies := map[string]map[string]string{
		"other.com": {
			"some_cookie": "value",
		},
	}

	header := buildBiliCookieHeader(allCookies)
	if header != "" {
		t.Errorf("expected empty header, got %s", header)
	}
}

func TestParseCookieFileMissing(t *testing.T) {
	_, err := ParseNetscapeCookieFile("/nonexistent/path/cookies.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ===================== 字幕选择测试 =====================

func TestPickSubtitlePrefersChinese(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "en", LanDoc: "English"},
		{Lan: "zh-CN", LanDoc: "中文(简体)"},
		{Lan: "ja", LanDoc: "日本語"},
	}

	best := pickSubtitle(subs)
	if best == nil {
		t.Fatal("expected non-nil result")
	}
	if best.Lan != "zh-CN" {
		t.Errorf("expected zh-CN, got %s", best.Lan)
	}
}

func TestPickSubtitleFallbackEnglish(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "en-US", LanDoc: "English (US)"},
		{Lan: "ja", LanDoc: "日本語"},
		{Lan: "ko", LanDoc: "한국어"},
	}

	best := pickSubtitle(subs)
	if best == nil {
		t.Fatal("expected non-nil result")
	}
	if best.Lan != "en-US" {
		t.Errorf("expected en-US, got %s", best.Lan)
	}
}

func TestPickSubtitleNoChineseOrEnglish(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "ja", LanDoc: "日本語"},
		{Lan: "ko", LanDoc: "한국어"},
	}

	best := pickSubtitle(subs)
	if best == nil {
		t.Fatal("expected non-nil result")
	}
	// 返回第一个
	if best.Lan != "ja" {
		t.Errorf("expected ja (first), got %s", best.Lan)
	}
}

func TestPickSubtitleEmpty(t *testing.T) {
	best := pickSubtitle([]SubtitleInfo{})
	if best != nil {
		t.Error("expected nil for empty input")
	}
}

func TestPickSubtitleSingleChinese(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "zh-CN", LanDoc: "中文(简体)"},
	}

	best := pickSubtitle(subs)
	if best == nil || best.Lan != "zh-CN" {
		t.Error("expected zh-CN")
	}
}

func TestPickSubtitleCaseInsensitive(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "EN", LanDoc: "English"},
		{Lan: "ZH-HANS", LanDoc: "中文"},
	}

	best := pickSubtitle(subs)
	if best == nil || best.Lan != "ZH-HANS" {
		t.Errorf("expected ZH-HANS (case insensitive), got %v", best)
	}
}

func TestPickSubtitleMultipleChinese(t *testing.T) {
	subs := []SubtitleInfo{
		{Lan: "zh-Hans", LanDoc: "中文(简体)"},
		{Lan: "zh-CN", LanDoc: "中文(简体)"},
		{Lan: "zh-TW", LanDoc: "中文(繁體)"},
	}

	best := pickSubtitle(subs)
	if best == nil {
		t.Fatal("expected non-nil")
	}
	// 第一个中文即可
	if !strings.HasPrefix(best.Lan, "zh") {
		t.Errorf("expected a Chinese language, got %s", best.Lan)
	}
}

// ===================== VTT 转换测试 =====================

func TestFormatVTTTime(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{59.999, "00:00:59.999"},
		{60, "00:01:00.000"},
		{3600, "01:00:00.000"},
		{3661.5, "01:01:01.500"},
		{7200.123, "02:00:00.123"},
	}

	for _, tc := range tests {
		got := formatVTTTime(tc.seconds)
		if got != tc.want {
			t.Errorf("formatVTTTime(%v) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestFormatVTTTimeNegative(t *testing.T) {
	got := formatVTTTime(-1.0)
	if got != "00:00:00.000" {
		t.Errorf("negative should be clamped to 0, got %s", got)
	}
}

func TestToVTTBasic(t *testing.T) {
	lines := []SubtitleLine{
		{From: 0.0, To: 2.5, Content: "Hello"},
		{From: 2.5, To: 5.0, Content: "World"},
	}

	vtt := toVTT(lines)

	if !strings.HasPrefix(vtt, "WEBVTT\n\n") {
		t.Error("VTT should start with WEBVTT header")
	}
	if !strings.Contains(vtt, "1\n00:00:00.000 --> 00:00:02.500\nHello") {
		t.Errorf("missing first subtitle: %s", vtt)
	}
	if !strings.Contains(vtt, "2\n00:00:02.500 --> 00:00:05.000\nWorld") {
		t.Errorf("missing second subtitle: %s", vtt)
	}
}

func TestToVTTUnordered(t *testing.T) {
	// 未排序输入应被自动排序
	lines := []SubtitleLine{
		{From: 5.0, To: 7.0, Content: "Second"},
		{From: 0.0, To: 3.0, Content: "First"},
		{From: 3.0, To: 5.0, Content: "Middle"},
	}

	vtt := toVTT(lines)

	// 验证顺序
	firstIdx := strings.Index(vtt, "First")
	middleIdx := strings.Index(vtt, "Middle")
	secondIdx := strings.Index(vtt, "Second")

	if firstIdx == -1 || middleIdx == -1 || secondIdx == -1 {
		t.Fatalf("missing subtitles in VTT: %s", vtt)
	}
	if !(firstIdx < middleIdx && middleIdx < secondIdx) {
		t.Error("subtitles not in order")
	}
}

func TestToVTTContentTrimming(t *testing.T) {
	lines := []SubtitleLine{
		{From: 0, To: 1, Content: "  Hello  "},
	}

	vtt := toVTT(lines)
	if !strings.Contains(vtt, "Hello\n") {
		t.Error("content should be trimmed")
	}
	if strings.Contains(vtt, "  Hello") {
		t.Error("leading spaces should be trimmed")
	}
}

func TestToVTTEmpty(t *testing.T) {
	vtt := toVTT([]SubtitleLine{})
	if vtt != "WEBVTT\n\n" {
		t.Errorf("empty should be just header: %q", vtt)
	}
}

// ===================== 集成测试（需要网络）=====================

func TestGetSubtitleLive(t *testing.T) {
	// 使用一个有字幕的视频测试
	// BV15ntz6gEHd 是一个测试视频
	result, err := GetSubtitle("BV15ntz6gEHd")
	if err != nil {
		t.Skipf("network or cookie unavailable: %v", err)
	}

	if result.Bvid != "BV15ntz6gEHd" {
		t.Errorf("bvid = %q, want BV15ntz6gEHd", result.Bvid)
	}
	if result.Format != "vtt" {
		t.Errorf("format = %q, want vtt", result.Format)
	}
	if !strings.HasPrefix(result.VTT, "WEBVTT") {
		t.Error("VTT should start with WEBVTT")
	}
	if len(result.Lines) == 0 {
		t.Error("should have subtitle lines")
	}
	if len(result.Available) == 0 {
		t.Error("should have available languages listed")
	}
	if result.Lang == "" {
		t.Error("should have selected language")
	}

	t.Logf("bvid=%s lang=%s lang_doc=%s lines=%d available=%d",
		result.Bvid, result.Lang, result.LangDoc,
		len(result.Lines), len(result.Available))
}

func TestGetSubtitleInvalidInput(t *testing.T) {
	_, err := GetSubtitle("not_a_video")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestGetSubtitleEmptyInput(t *testing.T) {
	_, err := GetSubtitle("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// ===================== Handler 测试 =====================

func TestSubtitleHandlerJSON(t *testing.T) {
	r := testRouter()

	// 坏输入 -> 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/bili/subtitle?url=不是视频", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad input: want 400, got %d (%s)", w.Code, w.Body.String())
	}

	// 路径参数形式（需要网络）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v2/bili/subtitle/BV15ntz6gEHd", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest {
		t.Skipf("network/cookie unavailable: %s", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Fatalf("subtitle: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "vtt") {
		t.Error("JSON response should contain 'vtt' field")
	}
}

func TestSubtitleHandlerVTT(t *testing.T) {
	r := testRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/bili/subtitle/vtt?url=BV15ntz6gEHd", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		t.Skipf("network/cookie unavailable: %s", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Fatalf("VTT: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/vtt") {
		t.Errorf("content-type = %q, want text/vtt", ct)
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Error("VTT response should start with WEBVTT")
	}

	// 检查 header
	bvid := w.Header().Get("X-Bilibili-Bvid")
	if bvid == "" {
		t.Error("missing X-Bilibili-Bvid header")
	}
}

func TestSubtitleHandlerVTTPathParam(t *testing.T) {
	r := testRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/bili/subtitle/vtt/BV15ntz6gEHd", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		t.Skipf("network/cookie unavailable: %s", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Fatalf("VTT path param: want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// ===================== 真实 cookie 文件测试 =====================

func TestLoadRealCookieFile(t *testing.T) {
	// 尝试加载真实的 cookie 文件（需要设置 BILIBILI_COOKIES_FILE）
	path := os.Getenv("BILIBILI_COOKIES_FILE")
	if path == "" {
		t.Skip("BILIBILI_COOKIES_FILE not set")
	}

	allCookies, err := ParseNetscapeCookieFile(path)
	if err != nil {
		t.Fatalf("failed to parse cookie file: %v", err)
	}

	header := buildBiliCookieHeader(allCookies)
	if header == "" {
		t.Error("no bilibili cookies found in file")
		return
	}

	// 检查关键 cookie 是否存在
	hasSessdata := false
	for _, pair := range strings.Split(header, "; ") {
		if strings.HasPrefix(pair, "SESSDATA=") && len(pair) > 9 {
			hasSessdata = true
			break
		}
	}
	if !hasSessdata {
		t.Error("SESSDATA not found in cookie header")
	}

	t.Logf("loaded %d cookies from %s", len(strings.Split(header, "; ")), path)
}

func TestGetBiliCookieNoEnv(t *testing.T) {
	// 确保环境变量未设置
	os.Unsetenv("BILIBILI_COOKIES_FILE")

	// 重置 once 以便重新测试
	biliCookieMu = sync.Once{}
	biliCookieErr = nil
	biliCookie = ""

	_, err := getBiliCookie()
	if err == nil {
		t.Error("expected error when BILIBILI_COOKIES_FILE is not set")
	}
	if !strings.Contains(err.Error(), "BILIBILI_COOKIES_FILE not set") {
		t.Errorf("error should mention missing env var, got: %v", err)
	}
}
