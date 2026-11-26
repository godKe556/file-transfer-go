package main

import (
	"net/http"

	"chuan/internal/auth"
	"chuan/internal/handlers"
	"chuan/internal/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// setupRouter 设置路由和中间件
func setupRouter(config *Config) http.Handler {
	// 初始化处理器
	h := handlers.NewHandler()

	// 初始化访问控制（密码为空时不启用）
	authManager := auth.NewManager(config.Password)

	router := chi.NewRouter()

	// 设置中间件
	setupMiddleware(router)

	// 设置API路由
	setupAPIRoutes(router, h, authManager)

	// 设置前端路由
	router.Handle("/*", web.CreateFrontendHandler())

	return router
}

// setupMiddleware 设置中间件
func setupMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CORS 配置
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Auth-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

// setupAPIRoutes 设置API路由
func setupAPIRoutes(r *chi.Mux, h *handlers.Handler, authManager *auth.Manager) {
	// 认证相关接口（无需密码即可访问）
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/status", authManager.HandleStatus)
		r.Post("/login", authManager.HandleLogin)
		r.Post("/logout", authManager.HandleLogout)
	})

	// 以下接口需要先通过密码校验
	r.Group(func(r chi.Router) {
		r.Use(authManager.Middleware)

		// WebRTC信令WebSocket路由
		r.Get("/api/ws/webrtc", h.HandleWebRTCWebSocket)
		r.Get("/ws/webrtc", h.HandleWebRTCWebSocket)

		// WebSocket 数据中继路由（P2P降级方案）
		r.Get("/api/ws/relay", h.HandleRelayWebSocket)
		r.Get("/ws/relay", h.HandleRelayWebSocket)

		// WebRTC房间API
		r.Post("/api/create-room", h.CreateRoomHandler)
		r.Get("/api/room-info", h.WebRTCRoomStatusHandler)
		r.Get("/api/webrtc-room-status", h.WebRTCRoomStatusHandler)
	})
}
