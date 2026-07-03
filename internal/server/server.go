package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/rlnorthcutt/arbor/internal/builder"
	"github.com/rlnorthcutt/arbor/internal/config"
	"github.com/rlnorthcutt/cmdkit/logger"
)

// Server runs the preview HTTP server with live reload.
type Server struct {
	projectRoot string
	port        int
	cfg         *config.Config
	builder     *builder.Builder
	log         *logger.Logger
	upgrader    websocket.Upgrader
	clients     map[*websocket.Conn]bool
	clientsMu   sync.Mutex
	rebuildOpts builder.BuildOptions // options reused on every file-change rebuild
}

// New creates a new Server.
func New(projectRoot string, port int, log *logger.Logger) (*Server, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	b, err := builder.New(projectRoot, log)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	return &Server{
		projectRoot: projectRoot,
		port:        port,
		cfg:         cfg,
		builder:     b,
		log:         log,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[*websocket.Conn]bool),
	}, nil
}

// Start performs an initial build then starts the file server, WebSocket server,
// and fsnotify watcher. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context, opts builder.BuildOptions) error {
	// Store options for subsequent rebuilds (without Force, since that's one-shot).
	s.rebuildOpts = builder.BuildOptions{
		MinifyAssets:    opts.MinifyAssets,
		AggregateAssets: opts.AggregateAssets,
	}

	// Initial build
	s.log.Info("Running initial build...")
	if err := s.builder.Build(ctx, opts); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	outputDir := filepath.Join(s.projectRoot, s.cfg.Build.OutputDir)
	wsPort := s.port + 1

	// Inject script tag for live reload
	reloadScript := fmt.Sprintf(`<script>
(function() {
  var ws = new WebSocket('ws://' + location.hostname + ':%d/ws');
  ws.onmessage = function(e) { if (e.data === 'reload') location.reload(); };
  ws.onclose = function() { setTimeout(function() { location.reload(); }, 1000); };
})();
</script>`, wsPort)

	// HTTP file server with script injection
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(outputDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isHTMLRequest(r) {
			injectingHandler(fileServer, reloadScript).ServeHTTP(w, r)
		} else {
			fileServer.ServeHTTP(w, r)
		}
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// WebSocket server
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/ws", s.handleWebSocket)
	wsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", wsPort),
		Handler: wsMux,
	}

	// Start watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	// Watch directories
	watchDirs := []string{
		filepath.Join(s.projectRoot, "content"),
		filepath.Join(s.projectRoot, "templates"),
		filepath.Join(s.projectRoot, "data"),
		filepath.Join(s.projectRoot, "static"),
	}
	for _, dir := range watchDirs {
		if err := watchDirRecursive(watcher, dir); err != nil {
			s.log.Warn("Could not watch %s: %v", dir, err)
		}
	}
	watcher.Add(filepath.Join(s.projectRoot, "config.toml")) //nolint

	// Start servers in goroutines
	errCh := make(chan error, 2)

	go func() {
		s.log.Info("Serving on http://localhost:%d", s.port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	go func() {
		s.log.Info("WebSocket on ws://localhost:%d/ws", wsPort)
		if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Debounce timer for rapid file changes
	var (
		debounceMu sync.Mutex
		debounce   *time.Timer
	)
	debounceDur := 200 * time.Millisecond

	// Watch loop
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					s.log.Detail("File changed: %s", event.Name)
					debounceMu.Lock()
					if debounce != nil {
						debounce.Stop()
					}
					debounce = time.AfterFunc(debounceDur, func() {
						s.log.Info("Rebuilding...")
						newBuilder, err := builder.New(s.projectRoot, s.log)
						if err != nil {
							s.log.Warn("Rebuild failed (builder): %v", err)
							return
						}
						s.builder = newBuilder
						if err := s.builder.Build(ctx, s.rebuildOpts); err != nil {
							s.log.Warn("Rebuild failed: %v", err)
							return
						}
						s.log.Success("Rebuild complete")
						s.broadcast("reload")
					})
					debounceMu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.log.Warn("Watcher error: %v", err)
			}
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.log.Info("Shutting down preview server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx) //nolint
		wsServer.Shutdown(shutdownCtx)   //nolint
		return nil
	case err := <-errCh:
		return err
	}
}

// handleWebSocket upgrades the connection and registers the client.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
	}()

	// Keep connection open, reading (and discarding) messages
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// broadcast sends a message to all connected WebSocket clients.
func (s *Server) broadcast(msg string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

// isHTMLRequest returns true if the request likely expects an HTML response.
func isHTMLRequest(r *http.Request) bool {
	path := r.URL.Path
	if path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, "/") {
		return true
	}
	// No extension → likely directory index
	if !strings.Contains(filepath.Base(path), ".") {
		return true
	}
	return false
}

// injectingHandler wraps an http.Handler and injects a script before </body>.
// It uses a responseRecorder with its own header map so that Content-Length
// set by http.FileServer does not leak to the real response (the injected
// script would make the body longer than the original Content-Length, causing
// browsers to truncate the page).
func injectingHandler(h http.Handler, script string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{
			ResponseWriter: w,
			header:         make(http.Header),
			body:           &strings.Builder{},
			code:           http.StatusOK,
		}
		h.ServeHTTP(rec, r)

		body := rec.body.String()
		if strings.Contains(body, "</body>") {
			body = strings.Replace(body, "</body>", script+"\n</body>", 1)
		}

		// Copy captured headers to the real response, skipping Content-Length
		// (it will be wrong after injection; let Go compute it from the body).
		dst := w.Header()
		for k, vv := range rec.header {
			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			dst[k] = vv
		}
		if dst.Get("Content-Type") == "" {
			dst.Set("Content-Type", "text/html; charset=utf-8")
		}
		w.WriteHeader(rec.code)
		w.Write([]byte(body)) //nolint
	})
}

// responseRecorder captures response body and headers for injection.
// It uses its own header map to prevent the file server's Content-Length
// from being written directly to the real ResponseWriter before we can
// correct it.
type responseRecorder struct {
	http.ResponseWriter
	header http.Header
	body   *strings.Builder
	code   int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	// Ensure Content-Type is captured if WriteHeader was never called explicitly.
	if r.header.Get("Content-Type") == "" {
		r.header.Set("Content-Type", http.DetectContentType(b))
	}
	return r.body.Write(b)
}

// watchDirRecursive adds all subdirectories to the watcher.
func watchDirRecursive(w *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if info.IsDir() {
			return w.Add(path)
		}
		return nil
	})
}
