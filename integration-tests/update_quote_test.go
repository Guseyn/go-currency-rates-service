package integration_tests

import (
	"bytes"
	"net/http"
	"testing"
)

func TestScenario_UpdateQuote_Success(t *testing.T) {
	urlUpdate := baseBufURL + "/rates/update"
	jsonPayload := `{"currency_pair": "EUR/USD"}`

	resp, err := http.Post(urlUpdate, "application/json", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("HTTP POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status code 202, got: %d", resp.StatusCode)
	}
}

func TestScenario_LatestQuote_NotFound(t *testing.T) {
	urlLatest := baseBufURL + "/rates/latest/NON%2FEXI"

	resp, err := http.Get(urlLatest)
	if err != nil {
		t.Fatalf("HTTP GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404, got: %d", resp.StatusCode)
	}
}
