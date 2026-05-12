package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"e2e-runner/config"
	"e2e-runner/handler"
	"e2e-runner/store"
	"e2e-runner/vnc"
)

func main() {
	cfg := config.Load()

	// SQLiteに切り替えるには以下をコメントアウトして下の行を使う:
	// runStore, err := store.NewSQLiteRunStore(cfg.DBPath)
	// if err != nil { log.Fatalf("DB初期化失敗: %v", err) }
	runStore := store.NewMemoryRunStore()

	h := handler.New(cfg, runStore, store.NewMemoryCodegenStore(), vnc.NewManager())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", handler.CORS(h.Tags))
	mux.HandleFunc("/api/run", handler.CORS(h.Run))
	mux.HandleFunc("/api/stream", handler.CORS(h.Stream))
	mux.HandleFunc("/api/codegen/start", handler.CORS(h.CodegenStart))
	mux.HandleFunc("/api/codegen/stream", handler.CORS(h.CodegenStream))
	mux.HandleFunc("/api/scenarios", handler.CORS(h.Scenarios))

	reportDir := filepath.Join(cfg.TestsDir, "playwright-report")
	mux.Handle("/report/", http.StripPrefix("/report/", http.FileServer(http.Dir(reportDir))))
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/report/", http.StatusMovedPermanently)
	})

	var srv http.Handler = mux
	if user, pass := os.Getenv("AUTH_USER"), os.Getenv("AUTH_PASS"); user != "" && pass != "" {
		srv = handler.BasicAuth(user, pass, mux)
	}
	log.Printf("runner起動: http://localhost%s", cfg.Port)
	log.Fatal(http.ListenAndServe(cfg.Port, srv))
}
