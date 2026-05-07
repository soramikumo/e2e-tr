package domain

import "sync"

type CodegenEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	File    string `json:"file,omitempty"`
}

type Codegen struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Name   string `json:"name"`
	Status string `json:"status"`
	File   string `json:"file,omitempty"`

	mu   sync.Mutex
	subs []chan CodegenEvent
	done bool
}

func NewCodegen(url, name string) *Codegen {
	return &Codegen{
		ID:     RandomID(),
		URL:    url,
		Name:   name,
		Status: "recording",
	}
}

func (c *Codegen) Send(ev CodegenEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (c *Codegen) Subscribe() (<-chan CodegenEvent, func()) {
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

func (c *Codegen) Finish(file string, err error) {
	c.mu.Lock()
	c.done = true
	var finalEv CodegenEvent
	if err != nil {
		c.Status = "error"
		finalEv = CodegenEvent{Type: "error", Message: err.Error()}
	} else {
		c.Status = "done"
		c.File = file
		finalEv = CodegenEvent{Type: "done", File: file}
	}
	subs := c.subs
	c.subs = nil
	c.mu.Unlock()
	for _, ch := range subs {
		ch <- finalEv
		close(ch)
	}
}
