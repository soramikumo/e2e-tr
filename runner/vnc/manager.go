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
	baseVNCPort   = 5900
	baseNoVNCPort = 6080
	maxSlots      = 10
)

type Session struct {
	Slot      int
	Display   string
	VNCPort   int
	NoVNCPort int
	xvfb      *exec.Cmd
	x11vnc    *exec.Cmd
	novnc     *exec.Cmd
}

func (s *Session) Stop() {
	for _, cmd := range []*exec.Cmd{s.novnc, s.x11vnc, s.xvfb} {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
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
	vncPort := baseVNCPort + slot
	noVNCPort := baseNoVNCPort + slot

	xvfb := exec.Command("Xvfb", display, "-screen", "0", "1280x800x24")
	if err := xvfb.Start(); err != nil {
		m.releaseSlot(slot)
		return nil, fmt.Errorf("Xvfb起動失敗: %v", err)
	}
	if err := waitForDisplay(display); err != nil {
		xvfb.Process.Kill()
		m.releaseSlot(slot)
		return nil, err
	}

	x11vnc := exec.Command("x11vnc",
		"-display", display,
		"-rfbport", fmt.Sprintf("%d", vncPort),
		"-nopw", "-forever", "-shared", "-quiet",
	)
	if err := x11vnc.Start(); err != nil {
		xvfb.Process.Kill()
		m.releaseSlot(slot)
		return nil, fmt.Errorf("x11vnc起動失敗: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	novnc := exec.Command("websockify",
		"--web", "/usr/share/novnc",
		fmt.Sprintf("%d", noVNCPort),
		fmt.Sprintf("localhost:%d", vncPort),
	)
	if err := novnc.Start(); err != nil {
		x11vnc.Process.Kill()
		xvfb.Process.Kill()
		m.releaseSlot(slot)
		return nil, fmt.Errorf("noVNC起動失敗: %v", err)
	}
	if err := waitForPort(noVNCPort); err != nil {
		novnc.Process.Kill()
		x11vnc.Process.Kill()
		xvfb.Process.Kill()
		m.releaseSlot(slot)
		return nil, err
	}

	s := &Session{
		Slot:      slot,
		Display:   display,
		VNCPort:   vncPort,
		NoVNCPort: noVNCPort,
		xvfb:      xvfb,
		x11vnc:    x11vnc,
		novnc:     novnc,
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
