package botmanager

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	mu       sync.Mutex
	cmd      *exec.Cmd
	running  bool
	stopped  bool // true when Stop() was called — suppresses "signal: killed" error
	lastErr  string
)

func pythonBin() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// Start launches the aiogram bot as a subprocess.
func Start(botDir, botToken, adminIDs, panelURL, secretKey string) error {
	mu.Lock()
	defer mu.Unlock()

	if running {
		return fmt.Errorf("bot is already running")
	}
	if botToken == "" {
		return fmt.Errorf("bot token is not configured")
	}

	mainPy := filepath.Join(botDir, "main.py")
	cmd = exec.Command(pythonBin(), mainPy)
	cmd.Dir = botDir
	cmd.Env = append(os.Environ(),
		"BOT_BOT_TOKEN="+botToken,
		"BOT_PANEL_URL="+panelURL,
		"BOT_PANEL_SECRET="+secretKey,
		"BOT_ADMIN_IDS="+adminIDs,
	)

	if err := cmd.Start(); err != nil {
		lastErr = err.Error()
		return fmt.Errorf("failed to start bot: %w", err)
	}

	running = true
	stopped = false
	lastErr = ""
	log.Printf("Telegram bot started (PID %d)", cmd.Process.Pid)

	go func() {
		err := cmd.Wait()
		mu.Lock()
		running = false
		if err != nil && !stopped {
			lastErr = err.Error()
			log.Printf("Telegram bot exited unexpectedly: %v", err)
		} else if !stopped {
			log.Printf("Telegram bot exited normally")
		}
		mu.Unlock()
	}()

	return nil
}

func Stop() error {
	mu.Lock()
	defer mu.Unlock()

	if !running || cmd == nil || cmd.Process == nil {
		running = false
		return nil
	}

	stopped = true
	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to stop bot: %w", err)
	}

	running = false
	lastErr = ""
	log.Printf("Telegram bot stopped")
	return nil
}

func Status() (bool, string) {
	mu.Lock()
	defer mu.Unlock()
	return running, lastErr
}
