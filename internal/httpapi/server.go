package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"netcatty-center/internal/config"
	"netcatty-center/internal/security"
	"netcatty-center/internal/store"

	"github.com/gin-gonic/gin"
)

type Server struct {
	store  *store.Store
	config config.Config
	engine *gin.Engine
}

func New(st *store.Store, cfg config.Config) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())
	engine.Use(cors())

	srv := &Server{store: st, config: cfg, engine: engine}
	srv.routes()
	return srv
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) routes() {
	s.engine.GET("/api/v1/health", s.health)
	s.engine.GET("/api/v1/catalog", s.catalog)

	s.engine.GET("/api/admin/setup-status", s.setupStatus)
	s.engine.POST("/api/admin/setup", s.setup)
	s.engine.POST("/api/admin/login", s.login)
	s.engine.POST("/api/admin/logout", s.logout)
	s.engine.GET("/api/admin/me", s.requireAdmin, s.me)

	s.engine.GET("/api/admin/hosts", s.requireAdmin, s.listHosts)
	s.engine.POST("/api/admin/hosts", s.requireAdmin, s.createHost)
	s.engine.PUT("/api/admin/hosts/:id", s.requireAdmin, s.updateHost)
	s.engine.DELETE("/api/admin/hosts/:id", s.requireAdmin, s.deleteHost)

	s.engine.GET("/api/admin/keys", s.requireAdmin, s.listKeys)
	s.engine.POST("/api/admin/keys", s.requireAdmin, s.createKey)
	s.engine.DELETE("/api/admin/keys/:id", s.requireAdmin, s.revokeKey)

	s.engine.GET("/api/admin/settings", s.requireAdmin, s.getSettings)
	s.engine.PUT("/api/admin/settings", s.requireAdmin, s.putSettings)

	publicDir := s.config.PublicDir
	s.engine.StaticFile("/styles.css", filepath.Join(publicDir, "styles.css"))
	s.engine.StaticFile("/app.js", filepath.Join(publicDir, "app.js"))
	s.engine.GET("/", s.index)
	s.engine.NoRoute(s.notFound)
}

func (s *Server) health(c *gin.Context) {
	settings, err := s.store.Settings()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"name":    settings.CenterName,
		"version": 1,
	})
}

func (s *Server) catalog(c *gin.Context) {
	plaintext := extractAPIKey(c)
	if plaintext == "" {
		fail(c, http.StatusUnauthorized, "缺少客户端密钥。请使用 Authorization: Bearer <key>")
		return
	}
	if !security.ValidAPIKeyFormat(plaintext) {
		fail(c, http.StatusUnauthorized, "密钥格式无效")
		return
	}
	id, _, ok, err := s.store.FindActiveAPIKeyByHash(security.SHA256Hex(plaintext))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		fail(c, http.StatusUnauthorized, "密钥无效或已吊销")
		return
	}
	_ = s.store.TouchAPIKey(id)
	settings, err := s.store.Settings()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	hosts, err := s.store.ListHosts()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	catalog := make([]store.CatalogHost, 0, len(hosts))
	for _, host := range hosts {
		catalog = append(catalog, store.ToCatalogHost(host))
	}
	c.JSON(http.StatusOK, gin.H{
		"version": 1,
		"center": gin.H{
			"id":   settings.CenterID,
			"name": settings.CenterName,
		},
		"generatedAt": time.Now().UnixMilli(),
		"hosts":       catalog,
	})
}

func (s *Server) setupStatus(c *gin.Context) {
	needs, err := s.store.NeedsSetup()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	settings, err := s.store.Settings()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"needsSetup": needs, "settings": settings})
}

func (s *Server) setup(c *gin.Context) {
	needs, err := s.store.NeedsSetup()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !needs {
		fail(c, http.StatusConflict, "已完成初始化，请直接登录")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求无效")
		return
	}
	username := strings.TrimSpace(body.Username)
	if len(username) < 2 {
		fail(c, http.StatusBadRequest, "用户名至少 2 个字符")
		return
	}
	if len(body.Password) < 8 {
		fail(c, http.StatusBadRequest, "密码至少 8 个字符")
		return
	}
	admin, err := s.store.CreateFirstAdmin(username, body.Password)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	s.issueSession(c, admin.ID)
	settings, _ := s.store.Settings()
	c.JSON(http.StatusOK, gin.H{"admin": admin, "settings": settings})
}

func (s *Server) login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求无效")
		return
	}
	admin, err := s.store.FindAdminByUsername(body.Username)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if admin == nil || !security.VerifyPassword(body.Password, admin.PasswordHash) {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	s.issueSession(c, admin.ID)
	settings, _ := s.store.Settings()
	c.JSON(http.StatusOK, gin.H{
		"admin":    admin.Admin,
		"settings": settings,
	})
}

func (s *Server) logout(c *gin.Context) {
	token, _ := c.Cookie(config.SessionCookie)
	if token != "" {
		_ = s.store.DeleteSession(security.SHA256Hex(token))
	}
	s.clearSession(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) me(c *gin.Context) {
	admin := currentAdmin(c)
	settings, err := s.store.Settings()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"admin": admin, "settings": settings})
}

func (s *Server) listHosts(c *gin.Context) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"hosts": hosts})
}

func (s *Server) createHost(c *gin.Context) {
	input, err := bindHostInput(c)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	host, err := s.store.CreateHost(input)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"host": host})
}

func (s *Server) updateHost(c *gin.Context) {
	input, err := bindHostInput(c)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	host, err := s.store.UpdateHost(c.Param("id"), input)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if host == nil {
		fail(c, http.StatusNotFound, "主机不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"host": host})
}

func (s *Server) deleteHost(c *gin.Context) {
	ok, err := s.store.DeleteHost(c.Param("id"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		fail(c, http.StatusNotFound, "主机不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listKeys(c *gin.Context) {
	keys, err := s.store.ListAPIKeys()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func (s *Server) createKey(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Netcatty 客户端"
	}
	generated := security.GenerateAPIKey()
	key, err := s.store.CreateAPIKey(name, generated.Hash, generated.Prefix, generated.Plaintext)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"key": key,
	})
}

func (s *Server) revokeKey(c *gin.Context) {
	ok, err := s.store.RevokeAPIKey(c.Param("id"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		fail(c, http.StatusNotFound, "密钥不存在或已吊销")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) getSettings(c *gin.Context) {
	settings, err := s.store.Settings()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (s *Server) putSettings(c *gin.Context) {
	var body struct {
		CenterName string `json:"centerName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.CenterName) == "" {
		fail(c, http.StatusBadRequest, "组织中心名称不能为空")
		return
	}
	settings, err := s.store.SetCenterName(body.CenterName)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (s *Server) requireAdmin(c *gin.Context) {
	token, err := c.Cookie(config.SessionCookie)
	if err != nil || token == "" {
		fail(c, http.StatusUnauthorized, "未登录")
		c.Abort()
		return
	}
	s.store.PurgeExpiredSessions()
	session, err := s.store.FindSession(security.SHA256Hex(token))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		c.Abort()
		return
	}
	if session == nil || session.ExpiresAt < time.Now().UnixMilli() {
		s.clearSession(c)
		fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
		c.Abort()
		return
	}
	admin, err := s.store.GetAdmin(session.AdminID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		c.Abort()
		return
	}
	if admin == nil {
		fail(c, http.StatusUnauthorized, "未登录")
		c.Abort()
		return
	}
	c.Set("admin", *admin)
	c.Next()
}

func (s *Server) issueSession(c *gin.Context, adminID string) {
	token := security.RandomToken()
	expiresAt := time.Now().UnixMilli() + config.SessionTTLMS
	_ = s.store.CreateSession(adminID, security.SHA256Hex(token), expiresAt)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     config.SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   config.SessionTTLMS / 1000,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.CookieSecure,
	})
}

func (s *Server) clearSession(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     config.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) index(c *gin.Context) {
	c.File(filepath.Join(s.config.PublicDir, "index.html"))
}

func (s *Server) notFound(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		fail(c, http.StatusNotFound, "Not found")
		return
	}
	if c.Request.Method == http.MethodGet {
		s.index(c)
		return
	}
	fail(c, http.StatusNotFound, "Not found")
}

func currentAdmin(c *gin.Context) store.Admin {
	value, _ := c.Get("admin")
	admin, _ := value.(store.Admin)
	return admin
}

func extractAPIKey(c *gin.Context) string {
	authorization := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return strings.TrimSpace(c.GetHeader("X-Netcatty-Key"))
}

func bindHostInput(c *gin.Context) (store.HostInput, error) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		return store.HostInput{}, errors.New("请求无效")
	}
	input := store.HostInput{
		Label:      asString(raw["label"]),
		Hostname:   asString(raw["hostname"]),
		Port:       asInt(raw["port"], 22),
		Username:   asString(raw["username"]),
		Group:      asString(raw["group"]),
		Tags:       asTags(raw["tags"]),
		OS:         asString(raw["os"]),
		Protocol:   asString(raw["protocol"]),
		DeviceType: asString(raw["deviceType"]),
		Notes:      asString(raw["notes"]),
		Password:   asString(raw["password"]),
		PrivateKey: asString(raw["privateKey"]),
		Passphrase: asString(raw["passphrase"]),
	}
	return input, nil
}

func asString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return strings.Trim(string(raw), `"`)
}

func asInt(raw json.RawMessage, fallback int) int {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return fallback
	}
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil && text != "" {
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed
		}
	}
	return fallback
}

func asTags(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err == nil {
		return tags
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parts := strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == '，'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}

func fail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Netcatty-Key")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func PublicDirExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}
