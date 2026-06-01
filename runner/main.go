package main

import (
	"log"
	"net/http"
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

	vm := vnc.NewManager(vnc.Options{
		SecurityTypes:    cfg.VNCSecurityTypes,
		DisableBasicAuth: cfg.VNCDisableBasicAuth,
		SSLOnly:          cfg.VNCSSLOnly,
		Interface:        cfg.VNCInterface,
	})
	h := handler.New(cfg, runStore, store.NewMemoryCodegenStore(), vm)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", handler.CORS(h.Tags))
	mux.HandleFunc("/api/run", handler.CORS(h.Run))
	mux.HandleFunc("/api/stream", handler.CORS(h.Stream))
	mux.HandleFunc("/api/codegen/start", handler.CORS(h.CodegenStart))
	mux.HandleFunc("/api/codegen/stream", handler.CORS(h.CodegenStream))
	mux.HandleFunc("/api/codegen/code", handler.CORS(h.CodegenCode))
	mux.HandleFunc("/api/scenarios", handler.CORS(h.Scenarios))
	mux.HandleFunc("/api/scenarios/tags", handler.CORS(h.ScenarioTags))

	reportDir := filepath.Join(cfg.TestsDir, "playwright-report")
	mux.Handle("/report/", http.StripPrefix("/report/", http.FileServer(http.Dir(reportDir))))
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/report/", http.StatusMovedPermanently)
	})

	log.Printf("runner起動: http://localhost%s", cfg.Port)
	log.Fatal(http.ListenAndServe(cfg.Port, mux))
}
