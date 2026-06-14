package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"e2e-runner/internal/config"
	"e2e-runner/internal/handler"
	"e2e-runner/internal/store"
	"e2e-runner/internal/vnc"
)

func main() {
	cfg := config.Load()

	// run 履歴は SQLite に永続化する(再起動でも残る)。DB の置き場所は、
	// DB_PATH が明示指定されていればそれを尊重し、未指定(既定)なら TestsDir 配下の
	// .runs.db を使う ── .tags.json / .environments.json と同じ流儀で、docker の
	// マウントボリュームに乗せて再起動・コンテナ作り直しでも履歴を残すため。
	dbPath := cfg.DBPath
	if os.Getenv("DB_PATH") == "" {
		dbPath = filepath.Join(cfg.TestsDir, ".runs.db")
	}
	runStore, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		log.Fatalf("DB初期化失敗: %v", err)
	}

	vm := vnc.NewManager(vnc.Options{
		SecurityTypes:    cfg.VNCSecurityTypes,
		DisableBasicAuth: cfg.VNCDisableBasicAuth,
		SSLOnly:          cfg.VNCSSLOnly,
		Interface:        cfg.VNCInterface,
	})
	h := handler.New(cfg, runStore, store.NewMemoryCodegenStore(), vm)

	mux := http.NewServeMux()
	h.Register(mux)

	// レポート配信は run.ID 単位(#88)。並列 run で出力先を分離したため、
	// /report/<run.ID>/... でその run のレポートを配信する。FileServer は
	// playwright-report/ をルートに持ち、<run.ID> はその直下のサブディレクトリ。
	// http.FileServer はパスを正規化し ".." を弾くが、run.ID 部分を英数字のみに
	// 制限してパストラバーサルをさらに確実に防ぐ。後方互換として、run.ID を
	// 含まない /report/ 直下アクセスも従来どおり FileServer が処理する。
	reportDir := filepath.Join(cfg.TestsDir, "playwright-report")
	fileServer := http.StripPrefix("/report/", http.FileServer(http.Dir(reportDir)))
	mux.Handle("/report/", validateReportPath(fileServer))
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/report/", http.StatusMovedPermanently)
	})

	log.Printf("runner起動: http://localhost%s", cfg.Port)
	log.Fatal(http.ListenAndServe(cfg.Port, mux))
}

// validateReportPath は /report/ 配下の最初のパス要素(= run.ID)が英数字のみで
// あることを検証してから next に委譲する。run.ID はサーバ生成の16進文字列なので
// 通常は素通りするが、不正な run.ID を含む URL を弾いてパストラバーサルを防ぐ。
// run.ID を含まない /report/ 直下(例: /report/ や /report/index.html)は
// 後方互換のためそのまま通す。
func validateReportPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "/report/" を除いた残りの先頭要素を取り出す。
		rest := strings.TrimPrefix(path.Clean(r.URL.Path), "/report/")
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			first := rest[:i]
			// 先頭要素があり、かつ英数字以外を含むなら拒否する。
			if first != "" && !isAlphanumeric(first) {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isAlphanumeric は s が英数字のみで構成されるか(空でないか)を返す。
func isAlphanumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}
