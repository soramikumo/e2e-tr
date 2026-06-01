package vnc

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	baseDisplay   = 99
	baseNoVNCPort = 6080
	maxSlots      = 10
	geometry      = "1600x900"
	depth         = "24"
	httpdDir      = "/usr/share/kasmvnc/www"
)

type Session struct {
	Slot      int
	Display   string
	NoVNCPort int
	xvnc      *exec.Cmd
}

func killAndWait(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

func (s *Session) Stop() {
	killAndWait(s.xvnc)
}

// Options は KasmVNC(Xvnc)のセキュリティ構成。config から注入する。
// vnc パッケージは config に依存しない（依存方向を main 経由の一方向に保つ）。
type Options struct {
	SecurityTypes    string // -SecurityTypes（空なら "None"）
	DisableBasicAuth bool   // true なら -DisableBasicAuth を付与
	SSLOnly          bool   // true なら -sslOnly 1、false なら 0
	Interface        string // -interface（空なら "0.0.0.0"）
}

func (o Options) withDefaults() Options {
	if o.SecurityTypes == "" {
		o.SecurityTypes = "None"
	}
	if o.Interface == "" {
		o.Interface = "0.0.0.0"
	}
	return o
}

type Manager struct {
	mu        sync.Mutex
	sessions  map[string]*Session
	freeSlots []int
	opts      Options
}

func NewManager(opts Options) *Manager {
	slots := make([]int, maxSlots)
	for i := range slots {
		slots[i] = i
	}
	return &Manager{
		sessions:  make(map[string]*Session),
		freeSlots: slots,
		opts:      opts.withDefaults(),
	}
}

func (m *Manager) Start(sessionID string) (*Session, error) {
	m.mu.Lock()
	if len(m.freeSlots) == 0 {
		m.mu.Unlock()
		return nil, fmt.Errorf("最大同時セッション数(%d)に達しました", maxSlots)
	}
	slot := m.freeSlots[0]
	m.freeSlots = m.freeSlots[1:]
	m.mu.Unlock()

	display := fmt.Sprintf(":%d", baseDisplay+slot)
	noVNCPort := baseNoVNCPort + slot

	// KasmVNC の Xvnc は X サーバー + VNC + Web を 1 プロセスで提供する。
	// セキュリティ構成（SecurityTypes/BasicAuth/sslOnly/interface）は config 由来の
	// Options で可変。既定は内部/コンテナ localhost 前提の無認証・平文 ws で、
	// Azure 等へ HTTPS 公開する段階では env で締める（VNC_SECURITY_TYPES など）。
	sslOnly := "0"
	if m.opts.SSLOnly {
		sslOnly = "1"
	}
	args := []string{display,
		"-geometry", geometry,
		"-depth", depth,
		"-SecurityTypes", m.opts.SecurityTypes,
		"-sslOnly", sslOnly,
		"-websocketPort", fmt.Sprintf("%d", noVNCPort),
		"-httpd", httpdDir,
		"-interface", m.opts.Interface,
	}
	if m.opts.DisableBasicAuth {
		args = append(args, "-DisableBasicAuth")
	}
	xvnc := exec.Command("Xvnc", args...)

	if err := xvnc.Start(); err != nil {
		m.releaseSlot(slot)
		return nil, fmt.Errorf("Xvnc起動失敗: %v", err)
	}
	if err := waitForDisplay(display); err != nil {
		killAndWait(xvnc)
		m.releaseSlot(slot)
		return nil, err
	}
	if err := waitForPort(noVNCPort); err != nil {
		killAndWait(xvnc)
		m.releaseSlot(slot)
		return nil, err
	}

	s := &Session{
		Slot:      slot,
		Display:   display,
		NoVNCPort: noVNCPort,
		xvnc:      xvnc,
	}

	m.mu.Lock()
	m.sessions[sessionID] = s
	m.mu.Unlock()

	return s, nil
}

func (m *Manager) Stop(sessionID string) {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	s.Stop()
	m.releaseSlot(s.Slot)
}

func (m *Manager) Get(sessionID string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

func (m *Manager) releaseSlot(slot int) {
	m.mu.Lock()
	m.freeSlots = append(m.freeSlots, slot)
	m.mu.Unlock()
}

// TCP ポートが接続を受け付けるまで最大 10 秒ポーリングする
func waitForPort(port int) error {
	addr := fmt.Sprintf("localhost:%d", port)
	for i := 0; i < 100; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %d が 10 秒以内に起動しませんでした", port)
}

// Xvfb が X11 ソケットを作成するまで最大 2 秒ポーリングする
func waitForDisplay(display string) error {
	num := strings.TrimPrefix(display, ":")
	socket := "/tmp/.X11-unix/X" + num
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(socket); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("display %s が 2 秒以内に起動しませんでした", display)
}
