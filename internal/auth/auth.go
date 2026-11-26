package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CookieName 会话 Cookie 名称
const CookieName = "chuan_auth"

// 会话有效期（秒）
const sessionMaxAge = 7 * 24 * 60 * 60

// 失败次数限制
const (
	maxFailures  = 8
	lockDuration = 60 * time.Second
)

type failureRecord struct {
	count    int
	lockedAt time.Time
}

// Manager 简单的密码访问控制
// 密码为空时表示不启用，所有请求直接放行
type Manager struct {
	password string
	token    string

	mu       sync.Mutex
	failures map[string]*failureRecord
}

// NewManager 创建访问控制管理器
func NewManager(password string) *Manager {
	m := &Manager{
		password: password,
		failures: make(map[string]*failureRecord),
	}
	if password != "" {
		// 会话令牌随进程生成，重启后所有会话失效（需重新输入密码）
		m.token = randomToken()
	}
	return m
}

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下退化为时间戳，保证服务可用
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

// Enabled 是否启用了密码
func (m *Manager) Enabled() bool { return m.password != "" }

// IsAuthenticated 判断请求是否已通过校验（Cookie 或请求头）
func (m *Manager) IsAuthenticated(r *http.Request) bool {
	if !m.Enabled() {
		return true
	}
	token := ""
	if c, err := r.Cookie(CookieName); err == nil {
		token = c.Value
	}
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(m.token)) == 1
}

// checkPassword 校验密码
func (m *Manager) checkPassword(pw string) bool {
	return subtle.ConstantTimeCompare([]byte(pw), []byte(m.password)) == 1
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// locked 是否处于锁定状态
func (m *Manager) locked(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.failures[ip]
	if !ok {
		return false
	}
	if time.Since(rec.lockedAt) > lockDuration {
		delete(m.failures, ip)
		return false
	}
	return rec.count >= maxFailures
}

// recordFailure 记录失败次数
func (m *Manager) recordFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.failures[ip]
	if !ok || time.Since(rec.lockedAt) > lockDuration {
		rec = &failureRecord{lockedAt: time.Now()}
		m.failures[ip] = rec
	}
	rec.count++
}

// clearFailures 登录成功后清除失败记录
func (m *Manager) clearFailures(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, ip)
}

func writeJSON(w http.ResponseWriter, status int, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// HandleStatus 查询当前访问状态
func (m *Manager) HandleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"enabled": m.Enabled(),
		"authed":  m.IsAuthenticated(r),
	})
}

// HandleLogin 校验密码并下发会话 Cookie
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"message": "方法不允许",
		})
		return
	}

	if !m.Enabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
		return
	}

	ip := clientIP(r)
	if m.locked(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"success": false,
			"message": "尝试次数过多，请稍后再试",
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "请求格式错误",
		})
		return
	}

	if !m.checkPassword(req.Password) {
		m.recordFailure(ip)
		// 轻微延迟，降低暴力尝试速度
		time.Sleep(300 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "密码错误",
		})
		return
	}

	m.clearFailures(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    m.token,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// HandleLogout 退出登录（清除 Cookie）
func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Middleware 保护需要密码才能访问的接口（含 WebSocket 升级）
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.IsAuthenticated(r) {
			next.ServeHTTP(w, r)
			return
		}

		// WebSocket 升级请求不能返回 JSON 之外的东西，直接 401
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "未授权，请输入访问密码",
		})
	})
}
