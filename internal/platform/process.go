package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type nativeProcess struct {
	PID      int    `json:"pid"`
	Nonce    string `json:"nonce"`
	Identity string `json:"identity"`
}

func processIdentity(pid int, nonce string) (string, error) {
	raw, err := exec.Command("ps", "-ww", "-p", strconv.Itoa(pid), "-o", "lstart=", "-o", "pgid=", "-o", "args=").Output()
	if err != nil {
		return "", err
	}
	identity := strings.TrimSpace(string(raw))
	if !strings.Contains(identity, nonce) {
		return "", errors.New("native process nonce does not match")
	}
	group, err := syscall.Getpgid(pid)
	if err != nil || group != pid {
		return "", errors.New("native process group does not match")
	}
	return identity, nil
}
func registerProcess(home string, pid int, nonce string) error {
	identity, err := processIdentity(pid, nonce)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(nativeProcess{pid, nonce, identity})
	if err != nil {
		return err
	}
	path := filepath.Join(home, "process.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	directory, err := os.Open(home)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// Runs under the platform data lock, before inputs may be claimed after restart.
func reconcileNativeProcesses(root string) error {
	paths, err := filepath.Glob(filepath.Join(root, "conversations", "*", "native", "process.json"))
	if err != nil {
		return err
	}
	more, err := filepath.Glob(filepath.Join(root, "workflow-processes", "*", "process.json"))
	if err != nil {
		return err
	}
	paths = append(paths, more...)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var process nativeProcess
		if json.Unmarshal(raw, &process) != nil || process.PID < 2 || !strings.HasPrefix(process.Nonce, "agent-platform-run-") {
			return fmt.Errorf("invalid native process checkpoint: %s", path)
		}
		groupErr := syscall.Kill(-process.PID, 0)
		if errors.Is(groupErr, syscall.ESRCH) {
			if err = os.Remove(path); err != nil {
				return err
			}
			continue
		}
		identity, err := processIdentity(process.PID, process.Nonce)
		if err != nil || identity != process.Identity {
			return fmt.Errorf("cannot safely identify prior native process group %d; inspect it before restarting", process.PID)
		}
		if err = syscall.Kill(-process.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		deadline := time.Now().Add(3 * time.Second)
		for syscall.Kill(-process.PID, 0) == nil && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if syscall.Kill(-process.PID, 0) == nil {
			return fmt.Errorf("prior native process group %d has not exited", process.PID)
		}
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
