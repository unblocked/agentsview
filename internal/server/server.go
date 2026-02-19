package server

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
	"github.com/wesm/agentsview/internal/web"
)

// Server is the HTTP server that serves the SPA and REST API.
type Server struct {
	mu     gosync.RWMutex
	cfg    config.Config
	db     *db.DB
	engine *sync.Engine
	mux    *http.ServeMux

	spaFS      fs.FS
	spaHandler http.Handler
}

// New creates a new Server.
func New(
	cfg config.Config, database *db.DB, engine *sync.Engine,
) *Server {
	dist, err := web.Assets()
	if err != nil {
		log.Fatalf("embedded frontend not found: %v", err)
	}

	s := &Server{
		cfg:        cfg,
		db:         database,
		engine:     engine,
		mux:        http.NewServeMux(),
		spaFS:      dist,
		spaHandler: http.FileServerFS(dist),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// API v1 routes
	s.mux.Handle("GET /api/v1/sessions", s.withTimeout(s.handleListSessions))
	s.mux.Handle("GET /api/v1/sessions/{id}", s.withTimeout(s.handleGetSession))
	s.mux.Handle(
		"GET /api/v1/sessions/{id}/messages", s.withTimeout(s.handleGetMessages),
	)
	s.mux.Handle(
		"GET /api/v1/sessions/{id}/minimap", s.withTimeout(s.handleGetMinimap),
	)
	// SSE: Do not use timeout, as this is a long-lived connection.
	s.mux.HandleFunc(
		"GET /api/v1/sessions/{id}/watch", s.handleWatchSession,
	)
	// Export: Do not use timeout handler to support large downloads and avoid buffering.
	s.mux.Handle(
		"GET /api/v1/sessions/{id}/export", http.HandlerFunc(s.handleExportSession),
	)
	s.mux.Handle(
		"POST /api/v1/sessions/{id}/publish", s.withTimeout(s.handlePublishSession),
	)
	s.mux.Handle(
		"POST /api/v1/sessions/upload", s.withTimeout(s.handleUploadSession),
	)
	s.mux.Handle("GET /api/v1/analytics/summary", s.withTimeout(s.handleAnalyticsSummary))
	s.mux.Handle("GET /api/v1/analytics/activity", s.withTimeout(s.handleAnalyticsActivity))
	s.mux.Handle("GET /api/v1/analytics/heatmap", s.withTimeout(s.handleAnalyticsHeatmap))
	s.mux.Handle("GET /api/v1/analytics/projects", s.withTimeout(s.handleAnalyticsProjects))

	s.mux.Handle("GET /api/v1/search", s.withTimeout(s.handleSearch))
	s.mux.Handle("GET /api/v1/projects", s.withTimeout(s.handleListProjects))
	s.mux.Handle("GET /api/v1/machines", s.withTimeout(s.handleListMachines))
	s.mux.Handle("GET /api/v1/stats", s.withTimeout(s.handleGetStats))
	s.mux.HandleFunc("POST /api/v1/sync", s.handleTriggerSync)
	s.mux.HandleFunc("GET /api/v1/sync", s.handleTriggerSync)
	s.mux.Handle("GET /api/v1/sync/status", s.withTimeout(s.handleSyncStatus))
	s.mux.Handle("GET /api/v1/config/github", s.withTimeout(s.handleGetGithubConfig))
	s.mux.Handle(
		"POST /api/v1/config/github", s.withTimeout(s.handleSetGithubConfig),
	)

	// SPA fallback: serve embedded frontend
	// Do not use timeout handler for static assets to avoid buffering.
	s.mux.Handle("/", http.HandlerFunc(s.handleSPA))
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	// Try to serve the exact file
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	f, err := s.spaFS.Open(path)
	if err == nil {
		f.Close()
		s.spaHandler.ServeHTTP(w, r)
		return
	}

	// SPA fallback: serve index.html for all routes
	r.URL.Path = "/"
	s.spaHandler.ServeHTTP(w, r)
}

// SetGithubToken updates the GitHub token for testing.
func (s *Server) SetGithubToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.GithubToken = token
}

// githubToken returns the current GitHub token (thread-safe).
func (s *Server) githubToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.GithubToken
}

// Handler returns the http.Handler with middleware applied.
func (s *Server) Handler() http.Handler {
	return corsMiddleware(logMiddleware(s.mux))
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Printf("Starting server at http://%s", addr)
	return srv.ListenAndServe()
}

// FindAvailablePort finds an available port starting from the given port.
func FindAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return start
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set(
				"Access-Control-Allow-Origin", "*",
			)
			w.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, OPTIONS",
			)
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type",
			)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
