// 直接oss里面存就行了，不用这么麻烦。
// 虽然会duplicate。

package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB
var httpClient = &http.Client{Timeout: 15 * time.Second}

func main() {
	var err error
	db, err = sql.Open("sqlite", "./games_assets.db")
	if err != nil {
		log.Fatal("数据库打开失败:", err)
	}

	http.HandleFunc("/", handleRequest)

	fmt.Println("RPG Proxy Server Started at :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// 去掉开头的 /
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "favicon.ico" {
		http.NotFound(w, r)
		return
	}

	// 解析 game_id 和 剩余路径
	parts := strings.SplitN(path, "/", 2)
	gameID := parts[0]
	assetPath := "index.html" // 默认访问 index.html
	if len(parts) > 1 && parts[1] != "" {
		assetPath = parts[1]
	}

	// 查询该路径对应的 OSS URL
	var targetURL string
	// 我们在 Python 已经把 target_url 存入 blobs，并通过 files 关联
	err := db.QueryRow(`
		SELECT b.url FROM blobs b 
		JOIN files f ON f.hash = b.hash 
		WHERE f.game_id = ? AND f.path = ? LIMIT 1`,
		gameID, assetPath).Scan(&targetURL)

	if err != nil {
		log.Printf("[404] %s/%s", gameID, assetPath)
		http.Error(w, "Asset not found in database", 404)
		return
	}

	targetURL = strings.Replace(targetURL, "oss.moonchan.xyz", "dlsite.810114.xyz", 1)

	// 判断是否是 index.html
	if strings.HasSuffix(assetPath, "index.html") {
		// 转发转发内容 (Proxy)
		proxyIndex(w, targetURL)
		return
	}

	// 其他全量资源执行 301
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
}

func proxyIndex(w http.ResponseWriter, url string) {
	resp, err := httpClient.Get(url)
	if err != nil {
		http.Error(w, "Failed to fetch index from OSS", 502)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "OSS returned error for index.html", resp.StatusCode)
		return
	}

	// 保持地址栏域名不变，转发 HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}
