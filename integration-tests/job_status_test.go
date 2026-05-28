package integration_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestScenario_JobStatus_Lifecycle runs an end-to-end integration scenario:
// It creates a job via the update endpoint, then asserts it can be retrieved via the status endpoint.
func TestScenario_JobStatus_Lifecycle(t *testing.T) {
	// =========================================================================
	// STEP 1: Call HandleUpdateQuote to seed a job into the live database
	// =========================================================================
	urlUpdate := fmt.Sprintf("%s/rates/update", baseBufURL)
	jsonPayload := `{"currency_pair": "USD/MXN"}`

	t.Logf("[STEP 1] Posting update request to %s", urlUpdate)
	updateResp, err := http.Post(urlUpdate, "application/json", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to execute POST /rates/update: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected status 202 Accepted from update endpoint, got: %d", updateResp.StatusCode)
	}

	// Unpack the newly created Job ID
	var updateData struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateData); err != nil {
		t.Fatalf("Failed to parse update JSON response: %v", err)
	}

	if updateData.ID == "" {
		t.Fatal("Application returned an empty job tracking UUID token")
	}
	t.Logf("[STEP 1 SUCCESS] Generated tracking Job ID: %s", updateData.ID)

	// =========================================================================
	// STEP 2: Call HandleGetJobStatus using the retrieved Job ID
	// =========================================================================
	urlStatus := fmt.Sprintf("%s/rates/job/%s", baseBufURL, updateData.ID)

	t.Logf("[STEP 2] Fetching job status from %s", urlStatus)
	statusResp, err := http.Get(urlStatus)
	if err != nil {
		t.Fatalf("Failed to execute GET /rates/job/{id}: %v", err)
	}
	defer statusResp.Body.Close()

	// Assert HTTP Status Code is 200 OK
	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200 OK, got: %d", statusResp.StatusCode)
	}

	// Assert Content-Type header
	contentType := statusResp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got: %s", contentType)
	}

	// Read and parse the response body
	bodyBytes, err := io.ReadAll(statusResp.Body)
	if err != nil {
		t.Fatalf("Failed to read status response body: %v", err)
	}

	var statusData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &statusData); err != nil {
		t.Fatalf("Failed to decode status JSON payload: %v. Body was: %s", err, string(bodyBytes))
	}

	// =========================================================================
	// STEP 3: Assert the database properties match our expectations
	// =========================================================================
	t.Log("[STEP 3] Validating response fields...")

	if statusData["id"] != updateData.ID {
		t.Errorf("Expected ID %q, got %q", updateData.ID, statusData["id"])
	}

	if statusData["currency_pair"] != "USD/MXN" {
		t.Errorf("Expected currency_pair 'USD/MXN', got %q", statusData["currency_pair"])
	}

	// Because it was just inserted, the initial status must be "pending"
	if statusData["status"] != "pending" {
		t.Errorf("Expected status to be 'pending', got %q", statusData["status"])
	}

	// The price should be null initially because workers haven't processed it yet
	if statusData["price"] != nil {
		t.Errorf("Expected initial price to be null, got: %v", statusData["price"])
	}

	if statusData["updated_at"] == "" {
		t.Error("Expected updated_at timestamp string to be populated, got empty string")
	}

	t.Log("[SUCCESS] Lifecycle integration test completed successfully.")
}

// TestScenario_JobStatus_NotFound ensures that looking up a fake UUID
// properly hits your pgx.ErrNoRows translation logic and responds with a clean 404 JSON object.
func TestScenario_JobStatus_NotFound(t *testing.T) {
	fakeUUID := "00000000-0000-0000-0000-000000000000"
	urlStatus := fmt.Sprintf("%s/rates/job/%s", baseBufURL, fakeUUID)

	resp, err := http.Get(urlStatus)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404, got: %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if !strings.Contains(bodyStr, `{"error": "Job tracking ID not found"}`) {
		t.Errorf("Expected clean 404 JSON contract, got: %s", bodyStr)
	}
}
