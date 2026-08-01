// chatto_reg 提供 Chatto 用户注册服务。
// 通过 Chatto operator API Unix socket 直接创建已验证邮箱的用户,不依赖 SMTP 邮件。
// 监听地址与 socket 路径均可由环境变量配置,默认 CHATTO_REG 为监听地址, CHATTO_REG_SOCKET 为 socket 路径(默认 /tmp/chatto/operator.sock)。
package chatto_reg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	operatorConnectPath = "/api/connect/chatto.operator.v1.OperatorUserService/CreateUser"
	defaultSocketPath   = "/tmp/chatto/operator.sock"
	maxLoginLength      = 32
	minLoginLength      = 2
	minPasswordLength   = 8
	maxPasswordLength   = 128
	loginRuleHint       = "2-32 位,字母或数字开头,可含 . _ -"
)

func Run(addr string) {
	socketPath := os.Getenv("CHATTO_REG_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	r := gin.Default()
	r.GET("/", indexPage)
	r.POST("/register", register(socketPath))
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.Run(addr)
}

func indexPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>注册</title>
<style>
body{font-family:system-ui,sans-serif;max-width:380px;margin:80px auto;padding:0 16px;color:#1f2328}
h1{font-size:20px}
label{display:block;margin:14px 0 6px;font-size:14px}
input{width:100%;box-sizing:border-box;padding:9px 12px;border:1px solid #d0d7de;border-radius:6px;font-size:14px}
button{width:100%;margin-top:20px;padding:10px;background:#1f6feb;color:#fff;border:none;border-radius:6px;font-size:15px;cursor:pointer}
button:disabled{opacity:.6;cursor:not-allowed}
#msg{margin-top:14px;font-size:14px;white-space:pre-wrap}
.err{color:#cf222e}
.ok{color:#1a7f37}
.hint{font-size:12px;color:#57606a;margin-top:4px}
</style>
</head>
<body>
<h1>注册 Chatto 账号</h1>
<form id="f">
<label for="username">用户名</label>
<input id="username" type="text" autocomplete="username" maxlength="32" required>
<div class="hint">2-32 位,字母或数字开头,可含 . _ -</div>
<label for="email">邮箱</label>
<input id="email" type="email" autocomplete="email" required>
<label for="password">密码</label>
<input id="password" type="password" autocomplete="new-password" required>
<div class="hint">密码至少 8 位,最长 128 位</div>
<label for="password2">确认密码</label>
<input id="password2" type="password" autocomplete="new-password" required>
<button id="btn" type="submit">注册</button>
</form>
<div id="msg"></div>
<script>
const f=document.getElementById('f'),btn=document.getElementById('btn'),msg=document.getElementById('msg');
f.addEventListener('submit',async e=>{
e.preventDefault();
msg.textContent='';msg.className='';
const username=document.getElementById('username').value.trim();
const email=document.getElementById('email').value.trim();
const p1=document.getElementById('password').value,p2=document.getElementById('password2').value;
if(p1!==p2){msg.textContent='两次输入的密码不一致';msg.className='err';return;}
btn.disabled=true;btn.textContent='注册中…';
try{
const r=await fetch('/register',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username,email,password:p1})});
const d=await r.json();
if(d&&d.ok){msg.innerHTML='注册成功!<br>邮箱: '+d.email+'<br>用户名: '+d.login+'<br><br>点击访问: <a href="https://chatto.moonchan.xyz" target="_blank" rel="noopener">https://chatto.moonchan.xyz</a>';msg.className='ok';f.reset();}
else{
let m=(d&&d.error)||'注册失败';
if(d&&d.detail){m+='\n'+d.detail;}
msg.textContent=m;msg.className='err';
}
}catch(err){msg.textContent='网络错误: '+err;msg.className='err';}
btn.disabled=false;btn.textContent='注册';
});
</script>
</body>
</html>`))
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	OK     bool   `json:"ok"`
	Email  string `json:"email,omitempty"`
	Login  string `json:"login,omitempty"`
	Url    string `json:"url,omitempty"`
	Error  string `json:"error,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// createUserRequest 是 chatto.operator.v1.CreateUserRequest 的 JSON 视图
// (ConnectRPC JSON 协议使用 lowerCamelCase 字段名)。
type createUserRequest struct {
	Login         string `json:"login"`
	DisplayName   string `json:"displayName"`
	Password      string `json:"password"`
	VerifiedEmail string `json:"verifiedEmail"`
}

func register(socketPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := registerResponse{}
		var req registerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error = "请求格式错误"
			c.JSON(http.StatusOK, resp)
			return
		}
		email := strings.TrimSpace(req.Email)
		if err := validateEmail(email); err != nil {
			resp.Error = err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}
		if err := validatePassword(req.Password); err != nil {
			resp.Error = err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}

		login := strings.TrimSpace(req.Username)
		if err := validateLogin(login); err != nil {
			resp.Error = fmt.Sprintf("用户名不合法:%s", loginRuleHint)
			resp.Detail = loginRuleHint
			c.JSON(http.StatusOK, resp)
			return
		}

		code, body, err := createUser(c.Request.Context(), socketPath, createUserRequest{
			Login:         login,
			DisplayName:   login,
			Password:      req.Password,
			VerifiedEmail: email,
		})
		if err != nil {
			resp.Error = "注册服务内部错误"
			resp.Detail = err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}
		if code != http.StatusOK {
			resp.Error = "注册失败"
			resp.Detail = operatorErrorMessage(body)
			c.JSON(http.StatusOK, resp)
			return
		}

		resp.OK = true
		resp.Email = email
		resp.Login = login
		resp.Url = "https://chatto.moonchan.xyz"
		c.JSON(http.StatusOK, resp)
	}
}

// createUser 通过 Unix socket 调用 Chatto operator API 创建用户。
func createUser(ctx context.Context, socketPath string, payload createUserRequest) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://chatto-operator"+operatorConnectPath, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, rb, nil
}

// operatorErrorMessage 从 ConnectRPC JSON 错误响应中提取 message 字段。
func operatorErrorMessage(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &e); err == nil && e.Message != "" {
		return e.Message
	}
	if len(body) > 300 {
		return string(body[:300])
	}
	return string(body)
}

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	if len(email) > 320 {
		return fmt.Errorf("邮箱过长")
	}
	if !strings.Contains(email, "@") {
		return fmt.Errorf("邮箱格式不正确")
	}
	at := strings.LastIndex(email, "@")
	if at == 0 || at == len(email)-1 {
		return fmt.Errorf("邮箱格式不正确")
	}
	if strings.ContainsAny(email[at+1:], " \t\r\n@") {
		return fmt.Errorf("邮箱格式不正确")
	}
	return nil
}

func validatePassword(p string) error {
	if len(p) < minPasswordLength {
		return fmt.Errorf("密码至少 %d 位", minPasswordLength)
	}
	if len(p) > maxPasswordLength {
		return fmt.Errorf("密码最长 %d 位", maxPasswordLength)
	}
	return nil
}

// validateLogin 与 Chatto 的 login 规则保持一致:字母/数字开头,仅含字母数字 . _ -。
func validateLogin(login string) error {
	if len(login) < minLoginLength || len(login) > maxLoginLength {
		return fmt.Errorf("login length out of range")
	}
	for i, r := range login {
		isLetterOrDigit := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if i == 0 && !isLetterOrDigit {
			return fmt.Errorf("invalid login")
		}
		if !isLetterOrDigit && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("invalid login")
		}
	}
	if strings.HasSuffix(login, ".") {
		return fmt.Errorf("invalid login")
	}
	return nil
}
