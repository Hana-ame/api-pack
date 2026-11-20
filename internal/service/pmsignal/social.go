// pm-signal 社交层 — 发现 / 推荐 / profile / 好友 / 陌生人打招呼。
//
// 设计裁定（用户需求逐条对应）：
//  1. 内置发现服务：在线 peer 的 profile 由信令连接携带（URL 参数 profile），
//     GET /discover 返回全部在线人员（含 profile，不含 token）。
//  2. 推荐：在线且「非自己、非已删、打招呼未超限」的人员列表；同活跃度排序。
//  3. 设置自己 profile：连接时带，或 PATCH /profile 改（持 token 才能改自己）。
//  4. 陌生人打招呼：POST /greet 发到服务器收件箱（对方不在线也存着，上线后推送）；
//     每个来源对每个目标限 1 条（防陌生人轰炸）；打招呼不建立 WebRTC 通道。
//  5. 加好友后才能无限发信息：**只有好友之间的 OFFER/ANSWER/CANDIDATE 才被
//     转发**（信令层执行）。非好友的 OFFER 一律拒绝——没有信令就没有 WebRTC
//     通道，物理上无法「无限发消息」。好友关系由 POST /friends/{id} 建立，
//     双方任一添加即算（对等模型，简化握手）。
//  6. 好友可以设置权限是否能控制本地 session：**纯 P2P 语义，服务器完全不参与**。
//     授权状态只存在各端 daemon 的 friend.canControlSession 字段里（默认 false），
//     既不上报服务器、也不出现在 discover/recommend/profile 的任何响应里——
//     别人无法从服务器探测「你能控制谁 / 谁能控制你」。控制请求走已建立的
//     WebRTC 通道，服务端看不到内容。
package pmsignal

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// socialProfile 一个 peer 的公开档案。全部字段有界，防滥用。
type socialProfile struct {
	Name       string `json:"name"`           // 展示名（≤32）
	Bio        string `json:"bio,omitempty"`  // 简介（≤200）
	Tags       string `json:"tags,omitempty"` // 逗号分隔标签（≤100）
	AllowGreet bool   `json:"allowGreet"`     // 是否接受陌生人打招呼（默认 true）
	// 注意：**没有 canControl 字段**。控制本地 session 的权限是纯 P2P 语义，
	// 只存在各端 daemon 的 friend 对象上，服务器不存、不广播、不参与校验。
	Visibility string `json:"visibility,omitempty"` // online | offline（服务器填）
}

// socialState 社交层全部状态。锁粒度：一把大锁（写少读多，规模小）。
type socialState struct {
	mu sync.RWMutex
	// profiles peerId -> 档案（在线时从连接带入；离线后保留一段时间再清）
	profiles map[string]*socialProfile
	// friends 对等好友关系：friends[a] 含 b 且 friends[b] 含 a 即互为好友。
	// 单向添加即写两侧（对等模型：任一方 add 即成立，删除即断）。
	friends map[string]map[string]bool
	// greetings 陌生人打招呼收件箱：target -> src -> 消息（每对限 1 条，新盖旧）。
	greetings map[string]map[string]*greeting
	// greetedAt 打招呼时间（用于推荐降权）
	greetedAt map[string]time.Time
	// lastSeen 活跃度（推荐排序用）
	lastSeen map[string]time.Time
}

type greeting struct {
	From string    `json:"from"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func newSocialState() *socialState {
	return &socialState{
		profiles:  make(map[string]*socialProfile),
		friends:   make(map[string]map[string]bool),
		greetings: make(map[string]map[string]*greeting),
		greetedAt: make(map[string]time.Time),
		lastSeen:  make(map[string]time.Time),
	}
}

// sanitizeProfile 校验并裁剪档案字段。全字段有界。
func sanitizeProfile(raw map[string]any) *socialProfile {
	p := &socialProfile{AllowGreet: true}
	if raw == nil {
		return p
	}
	str := func(k string, max int) string {
		v, _ := raw[k].(string)
		v = strings.TrimSpace(v)
		if len(v) > max {
			v = v[:max]
		}
		return v
	}
	p.Name = str("name", 32)
	p.Bio = str("bio", 200)
	p.Tags = str("tags", 100)
	if v, ok := raw["allowGreet"].(bool); ok {
		p.AllowGreet = v
	}
	return p
}

// onConnect 连接建立：登记 profile + 活跃度。
func (ss *socialState) onConnect(id string, prof *socialProfile) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if prof == nil {
		prof = &socialProfile{AllowGreet: true}
	}
	ss.profiles[id] = prof
	ss.lastSeen[id] = time.Now()
}

// onDisconnect 连接断开：保留档案（离线可见），活跃度更新。
func (ss *socialState) onDisconnect(id string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.lastSeen[id] = time.Now()
}

// setProfile 持 token 修改自己的档案。
func (ss *socialState) setProfile(id string, raw map[string]any) *socialProfile {
	p := sanitizeProfile(raw)
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.profiles[id] = p
	return p
}

// getProfile 单查。
func (ss *socialState) getProfile(id string) (socialProfile, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	p, ok := ss.profiles[id]
	if !ok {
		return socialProfile{}, false
	}
	return *p, true
}

// addFriend 对等加好友：任一方添加即成立。
func (ss *socialState) addFriend(a, b string) {
	if a == b || a == "" || b == "" {
		return
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.friends[a] == nil {
		ss.friends[a] = make(map[string]bool)
	}
	if ss.friends[b] == nil {
		ss.friends[b] = make(map[string]bool)
	}
	ss.friends[a][b] = true
	ss.friends[b][a] = true
	log.Printf("[pm-signal] social: %s ⇄ %s 成为好友", shortID(a), shortID(b))
}

// removeFriend 断好友：双方都清。
func (ss *socialState) removeFriend(a, b string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.friends[a], b)
	delete(ss.friends[b], a)
	log.Printf("[pm-signal] social: %s ✕ %s 断开好友", shortID(a), shortID(b))
}

// isFriend 互为好友才放行信令。
func (ss *socialState) isFriend(a, b string) bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.friends[a][b] && ss.friends[b][a]
}

// discover 在线人员（含档案）。onlineIDs 由信令层传入（当前真实在线者）。
func (ss *socialState) discover(onlineIDs []string, exclude string) []map[string]any {
	onlineSet := make(map[string]bool, len(onlineIDs))
	for _, id := range onlineIDs {
		onlineSet[id] = true
	}
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	out := make([]map[string]any, 0, len(onlineSet))
	for id := range onlineSet {
		if id == exclude {
			continue
		}
		entry := map[string]any{"peerId": id, "online": true}
		if p, ok := ss.profiles[id]; ok {
			entry["name"] = p.Name
			entry["bio"] = p.Bio
			entry["tags"] = p.Tags
			entry["allowGreet"] = p.AllowGreet
		} else {
			entry["name"] = ""
			entry["allowGreet"] = true
		}
		out = append(out, entry)
	}
	return out
}

// recommend 推荐：在线、非自己、非好友优先，最近活跃优先，打招呼过的降权。
func (ss *socialState) recommend(onlineIDs []string, me string, limit int) []map[string]any {
	onlineSet := make(map[string]bool, len(onlineIDs))
	for _, id := range onlineIDs {
		onlineSet[id] = true
	}
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	type cand struct {
		id    string
		score int64
	}
	cands := make([]cand, 0, len(onlineSet))
	for id := range onlineSet {
		if id == me {
			continue
		}
		p, hasProf := ss.profiles[id]
		// 打招呼目标不接受陌生人 → 不推荐
		if hasProf && !p.AllowGreet {
			continue
		}
		seen := ss.lastSeen[id]
		score := seen.Unix()
		if ss.friends[me][id] { // 已是好友 → 强降权（排最后）
			score -= 1 << 40
		}
		if t, ok := ss.greetedAt[me+"→"+id]; ok { // 打过招呼 → 降权
			score -= int64(time.Since(t).Seconds())
		}
		cands = append(cands, cand{id, score})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if len(cands) > limit {
		cands = cands[:limit]
	}
	out := make([]map[string]any, 0, len(cands))
	for _, c := range cands {
		entry := map[string]any{"peerId": c.id, "online": true}
		if p, ok := ss.profiles[c.id]; ok {
			entry["name"] = p.Name
			entry["bio"] = p.Bio
			entry["tags"] = p.Tags
			entry["allowGreet"] = p.AllowGreet
		} else {
			entry["name"] = ""
			entry["allowGreet"] = true
		}
		out = append(out, entry)
	}
	return out
}

// greet 陌生人打招呼：写入收件箱（每对 1 条，新盖旧）；目标须 allowGreet。
func (ss *socialState) greet(from, to, text string) error {
	text = strings.TrimSpace(text)
	if len(text) == 0 || len(text) > 200 {
		return errGreetText
	}
	if from == to {
		return errGreetSelf
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if p, ok := ss.profiles[to]; ok && !p.AllowGreet {
		return errGreetBlocked
	}
	if ss.greetings[to] == nil {
		ss.greetings[to] = make(map[string]*greeting)
	}
	ss.greetings[to][from] = &greeting{From: from, Text: text, At: time.Now()}
	ss.greetedAt[from+"→"+to] = time.Now()
	log.Printf("[pm-signal] social: %s → %s 打招呼", shortID(from), shortID(to))
	return nil
}

// drainGreetings 取走并清空自己的收件箱（上线后由客户端轮询调用）。
func (ss *socialState) drainGreetings(id string) []*greeting {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	box := ss.greetings[id]
	if box == nil {
		return nil
	}
	out := make([]*greeting, 0, len(box))
	for _, g := range box {
		out = append(out, g)
	}
	delete(ss.greetings, id)
	return out
}

// friendsOf 我的好友列表（含档案与在线标记）。
func (ss *socialState) friendsOf(id string, onlineSet map[string]bool) []map[string]any {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	set := ss.friends[id]
	out := make([]map[string]any, 0, len(set))
	for fid := range set {
		entry := map[string]any{"peerId": fid, "online": onlineSet[fid]}
		if p, ok := ss.profiles[fid]; ok {
			entry["name"] = p.Name
		}
		out = append(out, entry)
	}
	return out
}

// onlineSetSnapshot 信令层的在线集合（供 friendsOf 用）。
func (ss *socialState) onlineSetSnapshot(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// socialError 带错误码的社交层错误。
type socialError struct {
	Code int
	Msg  string
}

func (e *socialError) Error() string { return e.Msg }

var (
	errGreetSelf    = &socialError{Code: http.StatusBadRequest, Msg: "不能跟自己打招呼"}
	errGreetBlocked = &socialError{Code: http.StatusForbidden, Msg: "对方不接受陌生人打招呼"}
	errGreetText    = &socialError{Code: http.StatusBadRequest, Msg: "打招呼内容为空或超过 200 字"}
)

// handleSocial 社交层 HTTP API（挂在信令服务器同端口）。
//
//	GET  /discover            → 全部在线人员（含档案）
//	GET  /recommend           → 推荐人员（?limit=20）
//	GET  /profile?id=...      → 单查档案
//	PATCH /profile            → 改自己的档案（需 X-Peer-Id + X-Peer-Token 头）
//	POST /friends/{peerId}    → 加好友（对等：任一方添加即成立）
//	DELETE /friends/{peerId}  → 断好友
//	GET  /friends             → 我的好友列表
//	POST /greet               → 陌生人打招呼 {to, text}（需 X-Peer-Id 头）
//	GET  /greetings           → 取走我的收件箱
func (s *server) handleSocial(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	// ── 发现与推荐（公开只读）────────────────────────────────
	case path == "discover" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"people": s.social.discover(s.reg.snapshot(), ""),
		})
	case path == "recommend" && r.Method == http.MethodGet:
		me := r.Header.Get("X-Peer-Id")
		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			n := 0
			for _, c := range v {
				if c < '0' || c > '9' {
					n = -1
					break
				}
				n = n*10 + int(c-'0')
			}
			if n > 0 {
				limit = n
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"recommended": s.social.recommend(s.reg.snapshot(), me, limit),
		})

	// ── 档案 ─────────────────────────────────────────────────
	case path == "profile" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "missing id"})
			return
		}
		p, ok := s.social.getProfile(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "profile not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "profile": p, "online": s.reg.find(id) != nil})
	case path == "profile" && r.Method == http.MethodPatch:
		id, token := r.Header.Get("X-Peer-Id"), r.Header.Get("X-Peer-Token")
		if !s.reg.verifyOwner(id, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "bad peer credentials"})
			return
		}
		var raw map[string]any
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&raw); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid json"})
			return
		}
		p := s.social.setProfile(id, raw)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "profile": p})

	// ── 好友 ─────────────────────────────────────────────────
	case path == "friends" && r.Method == http.MethodGet:
		me := r.Header.Get("X-Peer-Id")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"friends": s.social.friendsOf(me, s.social.onlineSetSnapshot(s.reg.snapshot())),
		})
	case strings.HasPrefix(path, "friends/") && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		me, token := r.Header.Get("X-Peer-Id"), r.Header.Get("X-Peer-Token")
		if !s.reg.verifyOwner(me, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "bad peer credentials"})
			return
		}
		other := strings.TrimPrefix(path, "friends/")
		if other == "" || !validPeerID(other) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid peer id"})
			return
		}
		if r.Method == http.MethodPost {
			s.social.addFriend(me, other)
			// 加好友后通知对方（在线则推一条 SocialNotice；离线等它上线 discover）。
			if target := s.reg.find(other); target != nil {
				target.send(map[string]any{"type": "SOCIAL-NOTICE", "src": me, "payload": map[string]any{"event": "friend-added"}})
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		} else {
			s.social.removeFriend(me, other)
			if target := s.reg.find(other); target != nil {
				target.send(map[string]any{"type": "SOCIAL-NOTICE", "src": me, "payload": map[string]any{"event": "friend-removed"}})
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		}

	// ── 打招呼 ───────────────────────────────────────────────
	case path == "greet" && r.Method == http.MethodPost:
		me := r.Header.Get("X-Peer-Id")
		if me == "" || s.reg.find(me) == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unknown peer"})
			return
		}
		var body struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil || body.To == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid json"})
			return
		}
		if err := s.social.greet(me, body.To, body.Text); err != nil {
			if se, ok := err.(*socialError); ok {
				writeJSON(w, se.Code, map[string]any{"ok": false, "error": se.Msg})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "greet failed"})
			return
		}
		// 对方在线则即时推一条（SOCIAL-NOTICE），离线等它 drainGreetings。
		if target := s.reg.find(body.To); target != nil {
			target.send(map[string]any{"type": "SOCIAL-NOTICE", "src": me, "payload": map[string]any{"event": "greet", "text": body.Text}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case path == "greetings" && r.Method == http.MethodGet:
		me := r.Header.Get("X-Peer-Id")
		if me == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "missing peer id"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "greetings": s.social.drainGreetings(me)})

	default:
		http.NotFound(w, r)
	}
}

// validPeerID 校验 peer ID 形状（dsh-pm- 前缀 + 16-64 字符 [a-z0-9-]）。
func validPeerID(id string) bool {
	if len(id) < 16 || len(id) > 64 || !strings.HasPrefix(id, "dsh-pm-") {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
