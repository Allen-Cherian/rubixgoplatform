package core

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	
	"github.com/rubixchain/rubixgoplatform/wrapper/logger"
)

// ShutdownManager handles graceful shutdown of all components
type ShutdownManager struct {
	log            logger.Logger
	core           *Core
	mu             sync.Mutex
	shutdownSteps  []ShutdownStep
	isShuttingDown bool
}

// ShutdownStep represents a step in the shutdown process
type ShutdownStep struct {
	Name     string
	Function func() error
	Timeout  time.Duration
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(core *Core) *ShutdownManager {
	sm := &ShutdownManager{
		log:  core.log.Named("ShutdownManager"),
		core: core,
	}
	
	// Define shutdown steps in order
	sm.shutdownSteps = []ShutdownStep{
		{
			Name: "Stop IPFS Scalability Manager",
			Function: func() error {
				if sm.core.ipfsScalability != nil {
					sm.core.ipfsScalability.Stop()
				}
				return nil
			},
			Timeout: 5 * time.Second,
		},
		{
			Name: "Stop IPFS Health Manager",
			Function: func() error {
				if sm.core.ipfsHealth != nil {
					sm.core.ipfsHealth.Stop()
				}
				return nil
			},
			Timeout: 5 * time.Second,
		},
		{
			Name: "Stop IPFS Recovery Manager",
			Function: func() error {
				if sm.core.ipfsRecovery != nil {
					sm.core.ipfsRecovery.Stop()
				}
				return nil
			},
			Timeout: 5 * time.Second,
		},
		{
			Name: "Stop IPFS Daemon",
			Function: func() error {
				return sm.stopIPFSDaemon()
			},
			Timeout: 15 * time.Second,
		},
		{
			Name: "Stop HTTP Listener",
			Function: func() error {
				if sm.core.l != nil {
					sm.core.l.Shutdown()
				}
				return nil
			},
			Timeout: 5 * time.Second,
		},
		{
			Name: "Close Database Connections",
			Function: func() error {
				if sm.core.s != nil {
					sm.core.s.Close()
				}
				// Wallet doesn't have a Close method, just log
				if sm.core.w != nil {
					sm.log.Debug("Wallet cleanup completed")
				}
				return nil
			},
			Timeout: 5 * time.Second,
		},
	}
	
	return sm
}

// Shutdown performs graceful shutdown
func (sm *ShutdownManager) Shutdown() error {
	sm.mu.Lock()
	if sm.isShuttingDown {
		sm.mu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	sm.isShuttingDown = true
	sm.mu.Unlock()
	
	sm.log.Info("Starting graceful shutdown")
	
	var lastErr error
	for _, step := range sm.shutdownSteps {
		sm.log.Info("Executing shutdown step", "step", step.Name)
		
		ctx, cancel := context.WithTimeout(context.Background(), step.Timeout)
		defer cancel()
		
		done := make(chan error, 1)
		go func() {
			done <- step.Function()
		}()
		
		select {
		case err := <-done:
			if err != nil {
				sm.log.Error("Shutdown step failed", "step", step.Name, "error", err)
				lastErr = err
			} else {
				sm.log.Info("Shutdown step completed", "step", step.Name)
			}
		case <-ctx.Done():
			sm.log.Error("Shutdown step timed out", "step", step.Name, "timeout", step.Timeout)
			lastErr = fmt.Errorf("step %s timed out", step.Name)
		}
	}
	
	sm.log.Info("Graceful shutdown completed", "hadErrors", lastErr != nil)
	return lastErr
}

// stopIPFSDaemon stops the IPFS daemon with proper process management
func (sm *ShutdownManager) stopIPFSDaemon() error {
	if !sm.core.GetIPFSState() {
		sm.log.Debug("IPFS daemon not running, skip stop")
		return nil
	}
	
	sm.log.Info("Stopping IPFS daemon")
	
	// First try graceful shutdown via channel
	select {
	case sm.core.ipfsChan <- true:
		sm.log.Debug("Sent shutdown signal to IPFS")
	default:
		sm.log.Warn("IPFS channel blocked, trying alternative methods")
	}
	
	// Wait for graceful shutdown
	gracefulTimeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-gracefulTimeout:
			// Graceful shutdown failed, try forceful methods
			sm.log.Warn("Graceful IPFS shutdown timed out, attempting forceful shutdown")
			return sm.forceStopIPFS()
		case <-ticker.C:
			if !sm.core.GetIPFSState() {
				sm.log.Info("IPFS daemon stopped gracefully")
				return nil
			}
		}
	}
}

// forceStopIPFS forcefully stops IPFS using OS-specific methods
func (sm *ShutdownManager) forceStopIPFS() error {
	sm.log.Warn("Attempting to force stop IPFS daemon")
	
	// Try to find and kill IPFS process
	switch runtime.GOOS {
	case "windows":
		return sm.killIPFSWindows()
	case "linux", "darwin":
		return sm.killIPFSUnix()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// killIPFSWindows kills IPFS on Windows
func (sm *ShutdownManager) killIPFSWindows() error {
	// First try taskkill with /T flag to kill process tree
	cmd := exec.Command("taskkill", "/F", "/T", "/IM", "ipfs.exe")
	output, err := cmd.CombinedOutput()
	if err != nil {
		sm.log.Error("Failed to kill IPFS with taskkill", "error", err, "output", string(output))
		
		// Try PowerShell as fallback
		psCmd := `Get-Process ipfs -ErrorAction SilentlyContinue | Stop-Process -Force`
		cmd = exec.Command("powershell", "-Command", psCmd)
		output, err = cmd.CombinedOutput()
		if err != nil {
			sm.log.Error("Failed to kill IPFS with PowerShell", "error", err, "output", string(output))
			return err
		}
	}
	
	sm.log.Info("IPFS process killed on Windows")
	sm.core.SetIPFSState(false)
	return nil
}

// killIPFSUnix kills IPFS on Unix-like systems
func (sm *ShutdownManager) killIPFSUnix() error {
	// First try pkill
	cmd := exec.Command("pkill", "-f", "ipfs daemon")
	if err := cmd.Run(); err != nil {
		sm.log.Debug("pkill failed, trying killall", "error", err)
		
		// Try killall as fallback
		cmd = exec.Command("killall", "-9", "ipfs")
		if err := cmd.Run(); err != nil {
			sm.log.Debug("killall failed, trying pgrep/kill", "error", err)
			
			// Last resort: find PID and kill directly
			cmd = exec.Command("pgrep", "-f", "ipfs daemon")
			output, err := cmd.Output()
			if err != nil {
				sm.log.Error("Failed to find IPFS process", "error", err)
				return err
			}
			
			pids := string(output)
			if pids == "" {
				sm.log.Info("No IPFS process found")
				sm.core.SetIPFSState(false)
				return nil
			}
			
			// Kill each PID
			for _, pid := range splitLines(pids) {
				if pid == "" {
					continue
				}
				killCmd := exec.Command("kill", "-9", pid)
				if err := killCmd.Run(); err != nil {
					sm.log.Error("Failed to kill PID", "pid", pid, "error", err)
				} else {
					sm.log.Info("Killed IPFS process", "pid", pid)
				}
			}
		}
	}
	
	sm.log.Info("IPFS process killed on Unix")
	sm.core.SetIPFSState(false)
	return nil
}

// splitLines splits a string by newlines and returns non-empty lines
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// FindIPFSProcess checks if IPFS process is running
func (sm *ShutdownManager) FindIPFSProcess() (bool, int) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq ipfs.exe")
		output, err := cmd.Output()
		if err != nil {
			return false, 0
		}
		return strings.Contains(string(output), "ipfs.exe"), 0
		
	case "linux", "darwin":
		cmd := exec.Command("pgrep", "-f", "ipfs daemon")
		output, err := cmd.Output()
		if err != nil {
			return false, 0
		}
		pidStr := strings.TrimSpace(string(output))
		if pidStr == "" {
			return false, 0
		}
		// Return first PID
		pids := splitLines(pidStr)
		if len(pids) > 0 {
			var pid int
			fmt.Sscanf(pids[0], "%d", &pid)
			return true, pid
		}
		return false, 0
		
	default:
		return false, 0
	}
}