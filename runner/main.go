package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const port = ":8080"

var testsDir = func() string {
	if d := os.Getenv("TESTS_DIR"); d != "" {
		return d
	}
	return "../tests"
}()

// ============================================================================
// Utilities
// ============================================================================

func randomID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func sseStart(w http.ResponseWriter) (http.Flusher, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return f, true
}

func sseWrite(w http.ResponseWriter, f http.Flusher, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// sanitizeName prevents path traversal in user-supplied filenames.
func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "scenario-" + randomID()
	}
	return name
}

// ============================================================================
// Run — テスト実行セッション
// ============================================================================

type RunStatus string

const (
	StatusRunning RunStatus = "running"
	StatusDone    RunStatus = "done"
	StatusFailed  RunStatus = "failed"
)

type Run struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag,omitempty"`
	File      string    `json:"file,omitempty"`
	Status    RunStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`

	mu   sync.RWMutex
	logs []string
	subs []chan string
	done bool
}

func (r *Run) addLog(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, line)
	for _, ch := range r.subs {
		select {
		case ch <- line:
		default:
		}
	}
}

func (r *Run) subscribe() (<-chan string, func()) {
	ch := make(chan string, 512)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range r.logs {
		ch <- line
	}
	if r.done {
		close(ch)
		return ch, func() {}
	}
	r.subs = append(r.subs, ch)
	cancel := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, sub := range r.subs {
			if sub == ch {
				r.subs = append(r.subs[:i], r.subs[i+1:]...)
				return
			}
		}
	}
	return ch, cancel
}

func (r *Run) finish(success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if success {
		r.Status = StatusDone
	} else {
		r.Status = StatusFailed
	}
	r.done = true
	for _, ch := range r.subs {
		close(ch)
	}
	r.subs = nil
}

var (
	runs   = map[string]*Run{}
	runsMu sync.RWMutex
)

func newRun(tag, file string) *Run {
	r := &Run{ID: randomID(), Tag: tag, File: file, Status: StatusRunning, StartedAt: time.Now()}
	runsMu.Lock()
	runs[r.ID] = r
	runsMu.Unlock()
	return r
}

func getRun(id string) (*Run, bool) {
	runsMu.RLock()
	defer runsMu.RUnlock()
	r, ok := runs[id]
	return r, ok
}

func executeTest(run *Run) {
	var cmd *exec.Cmd
	if run.File != "" {
		file := sanitizeName(run.File)
		if !strings.HasSuffix(file, ".spec.ts") {
			file += ".spec.ts"
		}
		run.addLog(fmt.Sprintf("[info] テスト開始: %s", file))
		cmd = exec.Command("npx", "playwright", "test", "tests/"+file, "--reporter=line,html")
	} else {
		run.addLog(fmt.Sprintf("[info] テスト開始: @%s", run.Tag))
		cmd = exec.Command("npx", "playwright", "test", "--grep", "@"+run.Tag, "--reporter=line,html")
	}
	cmd.Dir = testsDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		run.addLog(fmt.Sprintf("[error] 起動失敗: %v", err))
		run.finish(false)
		return
	}

	var wg sync.WaitGroup
	pipe := func(r io.Reader, prefix string) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				run.addLog(prefix + line)
			}
		}
	}
	wg.Add(2)
	go pipe(stdout, "")
	go pipe(stderr, "[stderr] ")
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		run.addLog(fmt.Sprintf("[info] テスト終了: 失敗 (%v)", err))
		run.finish(false)
	} else {
		run.addLog("[info] テスト終了: 成功")
		run.finish(true)
	}
}

// ============================================================================
// Codegen — シナリオ記録セッション
// ============================================================================

type CodegenEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	File    string `json:"file,omitempty"`
}

type Codegen struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Name   string `json:"name"`
	Status string `json:"status"` // recording | done | error
	File   string `json:"file,omitempty"`

	mu   sync.Mutex
	subs []chan CodegenEvent
	done bool
}

func (c *Codegen) send(ev CodegenEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (c *Codegen) subscribe() (<-chan CodegenEvent, func()) {
	ch := make(chan CodegenEvent, 32)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		if c.Status == "done" {
			ch <- CodegenEvent{Type: "done", File: c.File}
		} else {
			ch <- CodegenEvent{Type: "error", Message: "記録に失敗しました"}
		}
		close(ch)
		return ch, func() {}
	}
	c.subs = append(c.subs, ch)
	cancel := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, sub := range c.subs {
			if sub == ch {
				c.subs = append(c.subs[:i], c.subs[i+1:]...)
				return
			}
		}
	}
	return ch, cancel
}

func (c *Codegen) finish(file string, err error) {
	c.mu.Lock()
	c.done = true
	var finalEv CodegenEvent
	if err != nil {
		c.Status = "error"
		finalEv = CodegenEvent{Type: "error", Message: err.Error()}
	} else {
		c.Status = "done"
		c.File = filepath.Base(file)
		finalEv = CodegenEvent{Type: "done", File: filepath.Base(file)}
	}
	subs := c.subs
	c.subs = nil
	c.mu.Unlock()

	for _, ch := range subs {
		ch <- finalEv
		close(ch)
	}
}

var (
	codegens   = map[string]*Codegen{}
	codegensMu sync.RWMutex
)

func newCodegen(url, name string) *Codegen {
	c := &Codegen{ID: randomID(), URL: url, Name: name, Status: "recording"}
	codegensMu.Lock()
	codegens[c.ID] = c
	codegensMu.Unlock()
	return c
}

func getCodegen(id string) (*Codegen, bool) {
	codegensMu.RLock()
	defer codegensMu.RUnlock()
	c, ok := codegens[id]
	return c, ok
}

func executeCodegen(c *Codegen) {
	name := sanitizeName(c.Name)
	outputFile := filepath.Join(testsDir, "tests", name+".spec.ts")

	c.send(CodegenEvent{Type: "status", Message: "ブラウザを起動しています..."})

	cmd := exec.Command("npx", "playwright", "codegen", "--output", outputFile, c.URL)
	cmd.Dir = testsDir

	if err := cmd.Start(); err != nil {
		c.finish("", fmt.Errorf("起動失敗: %v", err))
		return
	}

	c.send(CodegenEvent{Type: "status", Message: "記録中... ブラウザを閉じると保存されます"})

	err := cmd.Wait()

	if _, statErr := os.Stat(outputFile); statErr != nil {
		if err != nil {
			c.finish("", fmt.Errorf("記録失敗: %v", err))
		} else {
			c.finish("", fmt.Errorf("ファイルが保存されませんでした"))
		}
		return
	}

	c.finish(outputFile, nil)
}

// ============================================================================
// Scenarios & Tags
// ============================================================================

type Scenario struct {
	Name     string    `json:"name"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}

func listScenarios() []Scenario {
	pattern := filepath.Join(testsDir, "tests", "*.spec.ts")
	files, _ := filepath.Glob(pattern)
	scenarios := make([]Scenario, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		scenarios = append(scenarios, Scenario{
			Name:     filepath.Base(f),
			Modified: info.ModTime(),
			Size:     info.Size(),
		})
	}
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].Modified.After(scenarios[j].Modified)
	})
	return scenarios
}

var tagRe = regexp.MustCompile(`@([a-zA-Z][a-zA-Z0-9]*)`)

func scanTags() []string {
	pattern := filepath.Join(testsDir, "tests", "*.spec.ts")
	files, _ := filepath.Glob(pattern)
	tagSet := map[string]struct{}{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range tagRe.FindAllSubmatch(data, -1) {
			tagSet[string(m[1])] = struct{}{}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func handleTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tags": scanTags()})
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Tag  string `json:"tag"`
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Tag == "" && body.File == "" {
		http.Error(w, "tag or file required", http.StatusBadRequest)
		return
	}
	run := newRun(body.Tag, body.File)
	go executeTest(run)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": run.ID})
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	run, ok := getRun(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := sseStart(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, cancel := run.subscribe()
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, open := <-ch:
			if !open {
				run.mu.RLock()
				status := run.Status
				run.mu.RUnlock()
				sseWrite(w, flusher, map[string]string{"type": "done", "status": string(status)})
				return
			}
			sseWrite(w, flusher, map[string]string{"type": "log", "message": line})
		}
	}
}

func handleCodegenStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		body.Name = "scenario-" + randomID()
	}

	c := newCodegen(body.URL, body.Name)
	go executeCodegen(c)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": c.ID, "name": c.Name})
}

func handleCodegenStream(w http.ResponseWriter, r *http.Request) {
	c, ok := getCodegen(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := sseStart(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, cancel := c.subscribe()
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			sseWrite(w, flusher, ev)
		}
	}
}

func handleScenarios(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"scenarios": listScenarios()})

	case http.MethodDelete:
		name := sanitizeName(r.URL.Query().Get("name"))
		if !strings.HasSuffix(name, ".spec.ts") {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		path := filepath.Join(testsDir, "tests", name)
		if err := os.Remove(path); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/tags", cors(handleTags))
	mux.HandleFunc("/api/run", cors(handleRun))
	mux.HandleFunc("/api/stream", cors(handleStream))
	mux.HandleFunc("/api/codegen/start", cors(handleCodegenStart))
	mux.HandleFunc("/api/codegen/stream", cors(handleCodegenStream))
	mux.HandleFunc("/api/scenarios", cors(handleScenarios))

	reportDir := filepath.Join(testsDir, "playwright-report")
	mux.Handle("/report/", http.StripPrefix("/report/", http.FileServer(http.Dir(reportDir))))
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/report/", http.StatusMovedPermanently)
	})

	log.Printf("runner起動: http://localhost%s", port)
	log.Printf("tests dir: %s", testsDir)
	log.Fatal(http.ListenAndServe(port, mux))
}
