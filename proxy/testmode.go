package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// TestConfig holds settings for the quick test mode.
type TestConfig struct {
	ServerIP     string // external IP for tg:// links
	Domain       string // fake-TLS domain (default: google.com)
	PortOfficial int    // port for C proxy (default: 10443)
	PortTelemt   int    // port for telemt Rust proxy (default: 10444)
}

// RunTestMode spins up one Official (C) and one Telemt (Rust) proxy container,
// prints tg:// links for both, and waits for Ctrl+C to tear them down.
func RunTestMode(tc TestConfig) {
	if tc.Domain == "" {
		tc.Domain = "google.com"
	}
	if tc.PortOfficial == 0 {
		tc.PortOfficial = 10443
	}
	if tc.PortTelemt == 0 {
		tc.PortTelemt = 10444
	}
	if tc.ServerIP == "" {
		tc.ServerIP = "YOUR_SERVER_IP"
	}

	secret := GenerateSecret(tc.Domain)

	type testProxy struct {
		label   string
		backend string
		port    int
		name    string
	}

	proxies := []testProxy{
		{"C (official)", "official", tc.PortOfficial, "mtproxy-test-official"},
		{"Rust (telemt)", "telemt", tc.PortTelemt, "mtproxy-test-telemt"},
	}

	fmt.Println("=== MTProxy Test Mode ===")
	fmt.Printf("Secret : %s\n", secret)
	fmt.Printf("Domain : %s\n", tc.Domain)
	fmt.Printf("Key    : %s\n", ExtractKey(secret))
	fmt.Println()

	var started []testProxy

	// Pull missing images before starting
	for _, p := range proxies {
		backend := GetBackend(p.backend)
		img := backend.Info().Image
		if out, _ := exec.Command("docker", "images", "-q", img).Output(); len(strings.TrimSpace(string(out))) == 0 {
			log.Printf("Image %s not found, pulling...", img)
			pull := exec.Command("docker", "pull", img)
			pull.Stdout = os.Stdout
			pull.Stderr = os.Stderr
			if err := pull.Run(); err != nil {
				log.Printf("Failed to pull %s: %v", img, err)
			}
		}
	}

	for _, p := range proxies {
		backend := GetBackend(p.backend)
		if backend == nil {
			log.Printf("[%s] backend not found, skipping", p.label)
			continue
		}

		// Stop leftover container from previous test run
		exec.Command("docker", "stop", p.name).Run()
		exec.Command("docker", "rm", p.name).Run()

		args := backend.BuildRunArgs(p.name, p.port, []string{secret}, tc.Domain)
		log.Printf("[%s] docker %s", p.label, strings.Join(args, " "))

		out, err := exec.Command("docker", args...).CombinedOutput()
		if err != nil {
			log.Printf("[%s] FAILED: %v\n%s", p.label, err, string(out))
			continue
		}

		cid := strings.TrimSpace(string(out))
		if len(cid) > 12 {
			cid = cid[:12]
		}
		log.Printf("[%s] container %s started (ID: %s)", p.label, p.name, cid)

		// Health check — wait and verify container is still alive
		time.Sleep(3 * time.Second)
		inspect, _ := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", p.name).Output()
		if strings.TrimSpace(string(inspect)) != "true" {
			log.Printf("[%s] CRASHED — container logs:", p.label)
			logs, _ := exec.Command("docker", "logs", p.name).CombinedOutput()
			fmt.Println(string(logs))
			exec.Command("docker", "rm", p.name).Run()
			continue
		}

		started = append(started, p)
		link := BuildTgLink(tc.ServerIP, p.port, secret, p.backend, tc.Domain)
		fmt.Printf("\n  %s  port %d\n  %s\n", p.label, p.port, link)
	}

	if len(started) == 0 {
		fmt.Println("\nNo proxies started. Check Docker and images (docker images | grep -E 'telegrammessenger|telemt').")
		os.Exit(1)
	}

	fmt.Printf("\n%d/%d proxies running. Press Ctrl+C to stop and clean up.\n\n", len(started), len(proxies))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\n=== Container logs ===")
	for _, p := range started {
		fmt.Printf("\n--- %s (%s) ---\n", p.label, p.name)
		logs, _ := exec.Command("docker", "logs", "--tail", "30", p.name).CombinedOutput()
		fmt.Println(string(logs))
	}

	fmt.Println("Stopping test containers...")
	for _, p := range started {
		exec.Command("docker", "stop", p.name).Run()
		exec.Command("docker", "rm", p.name).Run()
		log.Printf("[%s] stopped", p.label)
	}
	fmt.Println("Done.")
}
