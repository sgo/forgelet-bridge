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
