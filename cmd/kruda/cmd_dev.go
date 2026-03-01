package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// ANSI color codes for terminal output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// devCmd is the Cobra command for `kruda dev`.
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start hot reload development server",
	Long: `Watch .go files and auto-rebuild/restart the application on changes.

The dev server builds your project with "go build", starts the binary,
and watches for file changes. When a .go file is modified, it rebuilds
and restarts automatically with debounced change detection.`,
	RunE: runDev,
}

func init() {
	devCmd.Flags().IntP("port", "p", 3000, "application port (passed as PORT env var)")
}

// runDev implements the hot reload development server loop.
func runDev(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	binPath := filepath.Join(dir, ".kruda-tmp")

	fmt.Printf("%s[kruda dev]%s Watching %s for .go file changes\n", colorGreen, colorReset, dir)
	fmt.Printf("%s[kruda dev]%s Port: %d\n", colorGreen, colorReset, port)

	// Initial build and start.
	var proc *os.Process
	if buildErr := build(binPath); buildErr != nil {
		printBuildError(buildErr)
	} else {
		proc, err = start(binPath, port)
		if err != nil {
			fmt.Printf("%s[error]%s Failed to start: %s\n", colorRed, colorReset, err)
		} else {
			fmt.Printf("%s[kruda dev]%s Listening on :%d\n", colorGreen, colorReset, port)
		}
	}

	// Set up file watcher.
	w := newWatcher(dir)
	if err := w.init(); err != nil {
		return fmt.Errorf("initializing watcher: %w", err)
	}

	done := make(chan struct{})
	changes := w.watch(done)

	// Handle OS signals for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigCh:
			fmt.Printf("\n%s[kruda dev]%s Received %s, shutting down...\n", colorYellow, colorReset, sig)
			close(done)
			if proc != nil {
				stopProcess(proc)
			}
			// Clean up temp binary.
			os.Remove(binPath)
			return nil

		case files, ok := <-changes:
			if !ok {
				return nil
			}

			// Display changed files.
			for _, f := range files {
				rel, _ := filepath.Rel(dir, f)
				if rel == "" {
					rel = f
				}
				fmt.Printf("%s[change]%s %s\n", colorYellow, colorReset, rel)
			}

			// Stop the old process.
			if proc != nil {
				stopProcess(proc)
				proc = nil
			}

			// Rebuild.
			fmt.Printf("%s[kruda dev]%s Building...\n", colorYellow, colorReset)
			startTime := time.Now()
			if buildErr := build(binPath); buildErr != nil {
				printBuildError(buildErr)
				continue // Keep watching for more changes.
			}
			elapsed := time.Since(startTime)
			fmt.Printf("%s[kruda dev]%s Build OK (%s)\n", colorGreen, colorReset, elapsed.Round(time.Millisecond))

			// Restart.
			proc, err = start(binPath, port)
			if err != nil {
				fmt.Printf("%s[error]%s Failed to start: %s\n", colorRed, colorReset, err)
				continue
			}
			fmt.Printf("%s[kruda dev]%s Listening on :%d\n", colorGreen, colorReset, port)
		}
	}
}

// build compiles the project in the current directory to the given output path.
func build(output string) error {
	cmd := exec.Command("go", "build", "-o", output, "./")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// start launches the built binary as a child process with the PORT env var set.
func start(binPath string, port int) (*os.Process, error) {
	cmd := exec.Command(binPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

// stopProcess sends SIGTERM to the process, waits up to 5 seconds,
// then sends SIGKILL if it hasn't exited.
func stopProcess(proc *os.Process) {
	// Send SIGTERM for graceful shutdown.
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited.
		proc.Wait()
		return
	}

	// Wait up to 5 seconds for the process to exit.
	exited := make(chan struct{})
	go func() {
		proc.Wait()
		close(exited)
	}()

	select {
	case <-exited:
		// Process exited gracefully.
	case <-time.After(5 * time.Second):
		// Force kill after timeout.
		fmt.Printf("%s[kruda dev]%s Process did not exit, sending SIGKILL\n", colorYellow, colorReset)
		proc.Kill()
		proc.Wait()
	}
}

// printBuildError displays build errors in red.
func printBuildError(err error) {
	msg := err.Error()
	// exec.ExitError contains stderr output already printed by the build command,
	// so just print a summary line.
	if strings.Contains(msg, "exit status") {
		fmt.Printf("%s[build error]%s Build failed (see errors above)\n", colorRed, colorReset)
	} else {
		fmt.Printf("%s[build error]%s %s\n", colorRed, colorReset, msg)
	}
	fmt.Printf("%s[kruda dev]%s Waiting for changes...\n", colorYellow, colorReset)
}
