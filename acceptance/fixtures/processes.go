package fixtures

import (
	"os"
	"os/exec"
	"time"
)

// stopProcess asks a fixture process to stop, and kills it if it does not stop
// within the grace it is given.
func stopProcess(cmd *exec.Cmd, log *os.File, grace time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(grace):
		_ = cmd.Process.Kill()
		<-done
	}
	if log != nil {
		log.Close()
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T20:53:38+02:00","module_hash":"2e75f9eb7a23495a3b0bd588c77d68ac7b57995e60810bd7dcc3f8dcf3aec9d9","functions":[{"id":"func/stopProcess","name":"stopProcess","line":11,"end_line":30,"hash":"4e07348f062c3c9e19cf73bed0a19b6dce8dca38a42600a71533bd485eb24bc9"}]}
// mutate4go-manifest-end
