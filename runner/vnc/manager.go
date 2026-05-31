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

type Manager struct {
	mu        sync.Mutex
	sessions  map[string]*Session
	freeSlots []int
}

func NewManager() *Manager {
	slots := make([]int, maxSlots)
	for i := range slots {
		slots[i] = i
	}
	return &Manager{
		sessions:  make(map[string]*Session),
		freeSlots: slots,
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
	// -SecurityTypes None -DisableBasicAuth -sslOnly 0 は内部ツール/コンテナ内
	// localhost 前提の無認証・平文 ws 構成（Azure を HTTPS 越しに公開する段階で
	// wss/認証の再検討が必要）。
	xvnc := exec.Command("Xvnc", display,
		"-geometry", geometry,
		"-depth", depth,
		"-SecurityTypes", "None",
		"-DisableBasicAuth",
		"-sslOnly", "0",
		"-websocketPort", fmt.Sprintf("%d", noVNCPort),
		"-httpd", httpdDir,
		"-interface", "0.0.0.0",
	)

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
