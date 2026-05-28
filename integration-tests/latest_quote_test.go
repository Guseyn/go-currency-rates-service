package integration_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestScenario_LatestQuote_Lifecycle verifies the full asynchronous pipeline:
// 1. Request a rate update via HandleUpdateQuote.
// 2. Wait for the background worker container to process the job and seed 'latest_prices'.
// 3. Query HandleGetLatestQuote to fetch the processed price cache.
func TestScenario_LatestQuote_Lifecycle(t *testing.T) {
	targetPair := "EUR/USD"

	// =========================================================================
	// STEP 1: Trigger the update worker pool via HandleUpdateQuote
	// =========================================================================
	urlUpdate := fmt.Sprintf("%s/rates/update", baseBufURL)
	jsonPayload := fmt.Sprintf(`{"currency_pair": %q}`, targetPair)

	t.Logf("[STEP 1] Triggering rate update for %s", targetPair)
	updateResp, err := http.Post(urlUpdate, "application/json", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to post update request: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected 202 Accepted from update endpoint, got: %d", updateResp.StatusCode)
	}

	// =========================================================================
	// STEP 2: Poll HandleGetLatestQuote while background worker processes the task
	// =========================================================================
	// Because URL paths contain slashes, we must URL-encode "EUR/USD" to "EUR%2FUSD"
	urlLatest := fmt.Sprintf("%s/rates/latest/EUR%%2FUSD", baseBufURL)
	t.Logf("[STEP 2] Polling %s waiting for background worker to populate cache...", urlLatest)

	var price float64
	var updatedAtStr string
	workerSuccess := false

	// Give the background worker up to 5 seconds to finish processing the async queue
	for i := 0; i < 10; i++ {
		resp, err := http.Get(urlLatest)
		if err != nil {
			t.Fatalf("HTTP GET request failed: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			// Worker finished processing and row now exists!
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var data map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				t.Fatalf("Failed to parse latest quote JSON: %v", err)
			}

			price = data["price"].(float64)
			updatedAtStr = data["updated_at"].(string)
			workerSuccess = true
			break
		}

		// If we get a 404, the background worker hasn't written to 'latest_prices' yet
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			t.Log("Worker processing... cache still empty. Retrying in 500ms...")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Treat any other status code (like a 500) as an immediate test failure
		resp.Body.Close()
		t.Fatalf("Unexpected status code received while polling: %d", resp.StatusCode)
	}

	if !workerSuccess {
		t.Fatal("Timeout: The background worker failed to update the latest_prices table within 5 seconds")
	}

	// =========================================================================
	// STEP 3: Validate the price cache output properties
	// =========================================================================
	t.Log("[STEP 3] Validating latest quote response records...")

	// Price must be a valid positive rate returned by your exchange simulator
	if price <= 0 {
		t.Errorf("Expected a valid positive price rate, got: %f", price)
	}

	// Ensure the timestamp string was compiled and returned
	if updatedAtStr == "" {
		t.Error("Expected updated_at field string timestamp, got empty value")
	}

	t.Logf("[SUCCESS] Latest quote integration validated. Current %s Rate: %f (Fetched at: %s)", targetPair, price, updatedAtStr)
}

// TestScenario_LatestQuote_NotFound ensures looking up an un-seeded, encoded
// currency pair safely drops through to your 404 plain-text fallback block.
func TestScenario_LatestQuote_NotFound_Case(t *testing.T) {
	// Query an asset pair that has never been processed or created
	urlLatest := fmt.Sprintf("%s/rates/latest/CAD%%2FCHF", baseBufURL)

	resp, err := http.Get(urlLatest)
	if err != nil {
		t.Fatalf("Request execution failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404 Not Found, got: %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	// Verifies alignment with your specific code fallback: http.Error(writer, "Currency pair not found", http.StatusNotFound)
	if bodyStr != "Currency pair not found\n" {
		t.Errorf("Expected body string message 'Currency pair not found', got: %q", bodyStr)
	}
}
