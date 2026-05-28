package integration_tests

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

const baseBufURL = "http://localhost:8080"

func TestMain(m *testing.M) {
	log.Println("🚀 1. Booting full Docker Compose environment...")

	// Start the docker compose stack from the parent directory
	cmdUp := exec.Command("docker", "compose", "up", "-d", "--build")
	cmdUp.Dir = "../"

	var errBuffer bytes.Buffer
	cmdUp.Stderr = &errBuffer

	if err := cmdUp.Run(); err != nil {
		log.Fatalf("Failed to spin up docker compose stack: %v", err)
	}

	// Ensure that no matter what happens, docker compose down runs at the very end
	defer func() {
		log.Println("🛑 4. Shutting down Docker Compose environment...")
		cmdDown := exec.Command("docker", "compose", "down", "-v")
		cmdDown.Dir = "../"
		_ = cmdDown.Run()
	}()

	log.Println("⏳ 2. Waiting for web application container to be healthy...")
	if err := waitForTargetServer(baseBufURL + "/rates/latest/USD%2FMXN"); err != nil {
		log.Fatalf("Server cluster failed to become healthy: %v", err)
	}

	log.Println("🏃 3. Executing all test files in the directory...")
	// CRITICAL: m.Run() automatically sweeps this folder and runs ALL tests
	// inside update_quote_test.go, latest_quote_test.go, etc.
	exitCode := m.Run()

	// Exit with the code returned by m.Run() (0 if all passed, 1 if any failed)
	os.Exit(exitCode)
}

func waitForTargetServer(url string) error {
	retries := 15
	for i := 0; i < retries; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("target server at %s never booted", url)
}
