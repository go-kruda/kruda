package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// pgoCmd is the Cobra command for `kruda pgo`.
var pgoCmd = &cobra.Command{
	Use:   "pgo",
	Short: "Generate PGO (Profile-Guided Optimization) profile",
	Long: `Generate a CPU profile for Profile-Guided Optimization (PGO).

This command:
  1. Builds your app with pprof enabled (adds net/http/pprof import)
  2. Starts your app alongside a pprof HTTP server
  3. Waits for you to generate representative load (or runs bombardier if available)
  4. Collects a CPU profile and saves it as default.pgo

The default.pgo file in your main package directory is automatically detected by
Go 1.21+ during 'go build', giving you 2-7% free performance improvement.

Usage:
  kruda pgo                     # Interactive mode: you provide the load
  kruda pgo --duration 60       # Auto-load mode: bombardier hits your endpoints
  kruda pgo --port 3000         # Specify your app's port
  kruda pgo --output ./cpu.pgo  # Custom output path`,
	RunE: runPGO,
}

func init() {
	pgoCmd.Flags().IntP("port", "p", 3000, "application port (your app listens on this)")
	pgoCmd.Flags().Int("pprof-port", 6060, "pprof HTTP server port")
	pgoCmd.Flags().IntP("duration", "d", 30, "profiling duration in seconds")
	pgoCmd.Flags().StringP("output", "o", "default.pgo", "output profile path")
	pgoCmd.Flags().StringSlice("endpoints", nil, "endpoints to load (e.g., /,/users/1,/api/health)")
	pgoCmd.Flags().Bool("auto", false, "auto-generate load using bombardier (requires bombardier in PATH)")
	pgoCmd.Flags().IntP("connections", "c", 100, "bombardier connections (only with --auto)")
}

func runPGO(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	pprofPort, _ := cmd.Flags().GetInt("pprof-port")
	duration, _ := cmd.Flags().GetInt("duration")
	output, _ := cmd.Flags().GetString("output")
	endpoints, _ := cmd.Flags().GetStringSlice("endpoints")
	autoLoad, _ := cmd.Flags().GetBool("auto")
	connections, _ := cmd.Flags().GetInt("connections")

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Resolve output path
	if !filepath.IsAbs(output) {
		output = filepath.Join(dir, output)
	}

	fmt.Printf("%s[kruda pgo]%s Profile-Guided Optimization\n", colorGreen, colorReset)
	fmt.Printf("%s[kruda pgo]%s App port: %d, pprof port: %d\n", colorGreen, colorReset, port, pprofPort)
	fmt.Printf("%s[kruda pgo]%s Duration: %ds\n", colorGreen, colorReset, duration)
	fmt.Printf("%s[kruda pgo]%s Output: %s\n", colorGreen, colorReset, output)
	fmt.Println()

	// Step 1: Build the app
	binPath := filepath.Join(dir, ".kruda-pgo-tmp")
	fmt.Printf("%s[kruda pgo]%s Building your app...\n", colorYellow, colorReset)

	buildArgs := []string{"build", "-o", binPath, "./"}
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Dir = dir
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	defer os.Remove(binPath)
	fmt.Printf("%s[kruda pgo]%s Build OK\n", colorGreen, colorReset)

	// Step 2: Start the app
	fmt.Printf("%s[kruda pgo]%s Starting app on :%d...\n", colorYellow, colorReset, port)
	appCmd := exec.Command(binPath)
	appCmd.Stdout = os.Stdout
	appCmd.Stderr = os.Stderr
	appCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if err := appCmd.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}
	appProc := appCmd.Process
	defer func() {
		if appProc != nil {
			appProc.Signal(syscall.SIGTERM)
			appProc.Wait()
		}
	}()

	// Wait for app to be ready
	appReady := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
		if err == nil {
			resp.Body.Close()
			appReady = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !appReady {
		return fmt.Errorf("app did not become ready on port %d within 15s", port)
	}
	fmt.Printf("%s[kruda pgo]%s App ready\n", colorGreen, colorReset)

	// Check if pprof is available
	pprofURL := fmt.Sprintf("http://localhost:%d/debug/pprof/", pprofPort)
	pprofReady := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(pprofURL)
		if err == nil {
			resp.Body.Close()
			pprofReady = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !pprofReady {
		// pprof not on separate port, try same port
		pprofURL = fmt.Sprintf("http://localhost:%d/debug/pprof/", port)
		resp, err := http.Get(pprofURL)
		if err == nil {
			resp.Body.Close()
			pprofReady = true
			pprofPort = port
			fmt.Printf("%s[kruda pgo]%s pprof found on app port :%d\n", colorGreen, colorReset, port)
		}
	}

	if !pprofReady {
		fmt.Printf("%s[kruda pgo]%s pprof endpoint not found!\n", colorRed, colorReset)
		fmt.Println()
		fmt.Println("  Add pprof to your app:")
		fmt.Println()
		fmt.Println("    import (")
		fmt.Println("        \"net/http\"")
		fmt.Println("        _ \"net/http/pprof\"")
		fmt.Println("    )")
		fmt.Println()
		fmt.Println("    func main() {")
		fmt.Println("        // Start pprof server")
		fmt.Println("        go http.ListenAndServe(\":6060\", nil)")
		fmt.Println("        // ... rest of your app")
		fmt.Println("    }")
		fmt.Println()
		return fmt.Errorf("pprof endpoint not available — see instructions above")
	}

	fmt.Printf("%s[kruda pgo]%s pprof ready at %s\n", colorGreen, colorReset, pprofURL)

	// Step 3: Start CPU profile collection
	profileURL := fmt.Sprintf("http://localhost:%d/debug/pprof/profile?seconds=%d", pprofPort, duration)
	profileFile := output + ".tmp"

	fmt.Printf("%s[kruda pgo]%s Collecting CPU profile for %ds...\n", colorYellow, colorReset, duration)

	// Start profile download in background
	profileDone := make(chan error, 1)
	go func() {
		resp, err := http.Get(profileURL)
		if err != nil {
			profileDone <- fmt.Errorf("pprof request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		f, err := os.Create(profileFile)
		if err != nil {
			profileDone <- fmt.Errorf("creating profile file: %w", err)
			return
		}
		defer f.Close()

		w := bufio.NewWriter(f)
		if _, err := w.ReadFrom(resp.Body); err != nil {
			profileDone <- fmt.Errorf("writing profile: %w", err)
			return
		}
		w.Flush()
		profileDone <- nil
	}()

	// Give pprof a moment to start sampling
	time.Sleep(2 * time.Second)

	// Step 4: Generate load
	if autoLoad {
		if err := runAutoLoad(port, connections, duration-2, endpoints); err != nil {
			fmt.Printf("%s[warn]%s Auto-load error: %s (profile may be incomplete)\n", colorYellow, colorReset, err)
		}
	} else {
		fmt.Println()
		fmt.Printf("%s[kruda pgo]%s Generate load on your app NOW!\n", colorGreen, colorReset)
		fmt.Println()
		fmt.Printf("  Your app is running on http://localhost:%d\n", port)
		fmt.Printf("  Profile collection ends in ~%ds\n", duration-2)
		fmt.Println()
		fmt.Println("  Suggested:")
		fmt.Printf("    bombardier -c 100 -d %ds http://localhost:%d/\n", duration-4, port)
		fmt.Println()
		fmt.Println("  Or run your test suite, integration tests, or any representative workload.")
		fmt.Println("  Waiting for profile collection to complete...")
		fmt.Println()
	}

	// Handle Ctrl+C during profiling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-profileDone:
		if err != nil {
			return err
		}
	case sig := <-sigCh:
		fmt.Printf("\n%s[kruda pgo]%s Received %s, aborting...\n", colorYellow, colorReset, sig)
		os.Remove(profileFile)
		return fmt.Errorf("interrupted")
	}

	// Step 5: Finalize
	info, err := os.Stat(profileFile)
	if err != nil || info.Size() == 0 {
		os.Remove(profileFile)
		return fmt.Errorf("profile is empty — did you generate enough load?")
	}

	if err := os.Rename(profileFile, output); err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}

	fmt.Println()
	fmt.Printf("%s[kruda pgo]%s Profile saved: %s (%s)\n", colorGreen, colorReset, output, formatBytes(info.Size()))
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    1. Rebuild your app:  go build ./...")
	fmt.Println("       (Go auto-detects default.pgo in main package)")
	fmt.Println("    2. Deploy the optimized binary (+2-7% faster)")
	fmt.Println("    3. Commit default.pgo to your repo")
	fmt.Println()
	fmt.Printf("  Verify PGO is active:  go build -v ./... 2>&1 | grep pgo\n")
	fmt.Println()

	return nil
}

// runAutoLoad uses bombardier to generate representative load on the given endpoints.
func runAutoLoad(port, connections, duration int, endpoints []string) error {
	if _, err := exec.LookPath("bombardier"); err != nil {
		return fmt.Errorf("bombardier not found in PATH — install: go install github.com/codesenberg/bombardier@latest")
	}

	if len(endpoints) == 0 {
		// Auto-discover endpoints by trying common paths
		endpoints = discoverEndpoints(port)
	}

	if len(endpoints) == 0 {
		endpoints = []string{"/"}
	}

	// Split duration across endpoints
	perEndpoint := duration / len(endpoints)
	if perEndpoint < 3 {
		perEndpoint = 3
	}

	for _, ep := range endpoints {
		url := fmt.Sprintf("http://localhost:%d%s", port, ep)
		fmt.Printf("%s[kruda pgo]%s Loading %s (%ds, %d connections)...\n",
			colorYellow, colorReset, ep, perEndpoint, connections)

		args := []string{
			"-c", strconv.Itoa(connections),
			"-d", fmt.Sprintf("%ds", perEndpoint),
			url,
		}
		cmd := exec.Command("bombardier", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%s[warn]%s bombardier error on %s: %s\n", colorYellow, colorReset, ep, err)
		}
	}

	return nil
}

// discoverEndpoints tries common paths and returns those that respond with 2xx.
func discoverEndpoints(port int) []string {
	candidates := []string{"/", "/health", "/api/health", "/users/1", "/api/v1/users"}
	var found []string

	client := &http.Client{Timeout: 2 * time.Second}
	for _, path := range candidates {
		url := fmt.Sprintf("http://localhost:%d%s", port, path)
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				found = append(found, path)
			}
		}
	}

	return found
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// pgoStripCmd removes the PGO profile to build without PGO.
var pgoStripCmd = &cobra.Command{
	Use:   "strip",
	Short: "Remove default.pgo to disable PGO",
	RunE: func(cmd *cobra.Command, args []string) error {
		pgoFile := filepath.Join(".", "default.pgo")
		if _, err := os.Stat(pgoFile); os.IsNotExist(err) {
			fmt.Printf("%s[kruda pgo]%s No default.pgo found — PGO is not active\n", colorYellow, colorReset)
			return nil
		}

		// Rename instead of delete, in case user wants it back
		backup := pgoFile + ".bak"
		if err := os.Rename(pgoFile, backup); err != nil {
			return fmt.Errorf("removing PGO profile: %w", err)
		}

		fmt.Printf("%s[kruda pgo]%s PGO disabled (profile backed up to %s)\n", colorGreen, colorReset, backup)
		fmt.Println("  Rebuild: go build ./...")
		return nil
	},
}

// pgoInfoCmd shows whether PGO is active and profile info.
var pgoInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show PGO profile status",
	RunE: func(cmd *cobra.Command, args []string) error {
		pgoFile := filepath.Join(".", "default.pgo")
		info, err := os.Stat(pgoFile)
		if os.IsNotExist(err) {
			fmt.Printf("%s[kruda pgo]%s No default.pgo found — PGO is NOT active\n", colorYellow, colorReset)
			fmt.Println()
			fmt.Println("  Generate a profile: kruda pgo --auto --duration 30")
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Printf("%s[kruda pgo]%s PGO is ACTIVE\n", colorGreen, colorReset)
		fmt.Printf("  File:     %s\n", pgoFile)
		fmt.Printf("  Size:     %s\n", formatBytes(info.Size()))
		fmt.Printf("  Modified: %s\n", info.ModTime().Format(time.RFC3339))
		fmt.Println()

		// Check Go version
		goVer, err := exec.Command("go", "version").Output()
		if err == nil {
			ver := strings.TrimSpace(string(goVer))
			fmt.Printf("  Go:       %s\n", ver)
			if strings.Contains(ver, "go1.20") || strings.Contains(ver, "go1.19") ||
				strings.Contains(ver, "go1.18") {
				fmt.Printf("  %s[warn]%s Go 1.21+ required for auto PGO detection\n", colorYellow, colorReset)
			}
		}

		return nil
	},
}
