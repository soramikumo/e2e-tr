package domain

import (
	"sync"
	"testing"
	"time"
)

// drainChannel reads all currently buffered messages from ch and returns them.
func drainChannel(ch <-chan string) []string {
	var lines []string
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return lines
			}
			lines = append(lines, line)
		default:
			return lines
		}
	}
}

// TestAddLog_Subscribe_ExistingLogsDelivered verifies that existing logs are
// sent to a new subscriber channel before the channel is returned.
func TestAddLog_Subscribe_ExistingLogsDelivered(t *testing.T) {
	r := NewRun("tag1", "file1")
	r.AddLog("line1")
	r.AddLog("line2")
	r.AddLog("line3")

	ch, cancel := r.Subscribe()
	defer cancel()

	got := drainChannel(ch)
	if len(got) != 3 {
		t.Fatalf("expected 3 log lines, got %d: %v", len(got), got)
	}
	for i, want := range []string{"line1", "line2", "line3"} {
		if got[i] != want {
			t.Errorf("log[%d] = %q, want %q", i, got[i], want)
		}
	}
}

// TestAddLog_Subscribe_NewLogsDelivered verifies that logs added after
// subscribing are delivered to the subscriber channel.
func TestAddLog_Subscribe_NewLogsDelivered(t *testing.T) {
	r := NewRun("tag1", "file1")
	ch, cancel := r.Subscribe()
	defer cancel()

	r.AddLog("new line")

	select {
	case got := <-ch:
		if got != "new line" {
			t.Errorf("got %q, want %q", got, "new line")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log line")
	}
}

// TestSubscribe_AfterFinish_ChannelClosedImmediately verifies that subscribing
// after Finish closes the returned channel without any blocking.
func TestSubscribe_AfterFinish_ChannelClosedImmediately(t *testing.T) {
	r := NewRun("tag1", "file1")
	r.AddLog("existing")
	r.Finish(true)

	ch, _ := r.Subscribe()

	// Drain existing logs and expect channel to be closed.
	var receivedExisting bool
	for line := range ch {
		if line == "existing" {
			receivedExisting = true
		}
	}
	if !receivedExisting {
		t.Error("expected to receive existing log before channel was closed")
	}
}

// TestMultipleSubscribers_CancelOne_OthersUnaffected verifies that cancelling
// one subscriber does not affect other subscribers.
func TestMultipleSubscribers_CancelOne_OthersUnaffected(t *testing.T) {
	r := NewRun("tag1", "file1")

	ch1, cancel1 := r.Subscribe()
	ch2, cancel2 := r.Subscribe()
	defer cancel2()

	// Cancel subscriber 1.
	cancel1()

	// Add a log; only ch2 should receive it (ch1 is unsubscribed).
	r.AddLog("only for 2")

	select {
	case got := <-ch2:
		if got != "only for 2" {
			t.Errorf("ch2 got %q, want %q", got, "only for 2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log on ch2")
	}

	// ch1 should not receive anything (it was cancelled before AddLog).
	select {
	case msg, ok := <-ch1:
		if ok {
			t.Errorf("ch1 unexpectedly received %q after cancel", msg)
		}
		// closed channel is acceptable (no message)
	default:
		// nothing received — correct
	}
}

// TestFinish_AfterAddLog_NoPanic verifies that calling AddLog after Finish
// (when subs is nil and done is true) does not panic.
func TestFinish_AfterAddLog_NoPanic(t *testing.T) {
	r := NewRun("tag1", "file1")
	r.Finish(true)

	// Should not panic.
	r.AddLog("late log")

	// The late log should still be appended to internal logs.
	logs := r.Logs()
	if len(logs) != 1 || logs[0] != "late log" {
		t.Errorf("expected ['late log'], got %v", logs)
	}
}

// TestConcurrent_AddLog_Subscribe_Cancel verifies there are no race conditions
// when multiple goroutines call AddLog, Subscribe, and cancel concurrently.
// Run with: go test -race ./domain/...
func TestConcurrent_AddLog_Subscribe_Cancel(t *testing.T) {
	r := NewRun("tag1", "file1")

	const (
		numWriters     = 4
		numSubscribers = 4
		logsPerWriter  = 50
	)

	var wg sync.WaitGroup

	// Writers: add logs concurrently.
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerWriter; j++ {
				r.AddLog("msg")
			}
		}(i)
	}

	// Subscribers: subscribe, read a few messages, then cancel.
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := r.Subscribe()
			// Read up to 5 messages then cancel.
			for k := 0; k < 5; k++ {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-time.After(500 * time.Millisecond):
					cancel()
					return
				}
			}
			cancel()
		}()
	}

	wg.Wait()
	r.Finish(true)

	// Subscribing after finish must not block.
	ch, _ := r.Subscribe()
	for range ch {
		// drain
	}
}

// TestFinish_Success_StatusDone checks GetStatus after successful finish.
func TestFinish_Success_StatusDone(t *testing.T) {
	r := NewRun("tag", "file")
	r.Finish(true)
	if got := r.GetStatus(); got != StatusDone {
		t.Errorf("expected %q, got %q", StatusDone, got)
	}
}

// TestFinish_Failure_StatusFailed checks GetStatus after failed finish.
func TestFinish_Failure_StatusFailed(t *testing.T) {
	r := NewRun("tag", "file")
	r.Finish(false)
	if got := r.GetStatus(); got != StatusFailed {
		t.Errorf("expected %q, got %q", StatusFailed, got)
	}
}

// TestSubscribe_Finish_ChannelClosed verifies that finishing closes all active
// subscriber channels.
func TestSubscribe_Finish_ChannelClosed(t *testing.T) {
	r := NewRun("tag", "file")
	ch1, _ := r.Subscribe()
	ch2, _ := r.Subscribe()

	r.Finish(true)

	// Both channels must be closed (range should exit immediately after drain).
	done := make(chan struct{}, 2)
	go func() {
		for range ch1 {
		}
		done <- struct{}{}
	}()
	go func() {
		for range ch2 {
		}
		done <- struct{}{}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out: subscriber channel not closed after Finish")
		}
	}
}
