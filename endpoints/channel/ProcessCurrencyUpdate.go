package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/endpoints"
	"github.com/Guseyn/go-currency-rates-service.git/structs"
)

func ProcessCurrencyUpdate(e *endpoints.Env, task structs.Task) error {
	parts := strings.Split(task.CurrencyPair, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid currency pair format: %s (expected BASE/TARGET)", task.CurrencyPair)
	}
	baseCurrency := strings.ToUpper(parts[0])
	targetCurrency := strings.ToUpper(parts[1])

	apiURL := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", baseCurrency)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to reach exchange rate API: %w", err)
	}
	defer resp.Body.Close() // Always close the network stream to prevent memory leaks

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("external API returned non-200 status: %d", resp.StatusCode)
	}

	var apiData structs.ExchangeRateAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiData); err != nil {
		return fmt.Errorf("failed to decode API JSON response: %w", err)
	}

	ratePrice, exists := apiData.Rates[targetCurrency]
	if !exists {
		return fmt.Errorf("target currency %s not supported by base %s", targetCurrency, baseCurrency)
	}

	now := time.Now()
	ctx := context.Background()

	_, err = e.DB.Exec(ctx,
		"UPDATE update_jobs SET status = $1, price = $2, updated_at = $3 WHERE id = $4",
		"completed", ratePrice, now, task.JobID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job status in DB: %w", err)
	}

	_, err = e.DB.Exec(ctx, `
		INSERT INTO latest_prices (currency_pair, price, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (currency_pair) 
		DO UPDATE SET 
			price = EXCLUDED.price,
			updated_at = EXCLUDED.updated_at
		WHERE EXCLUDED.updated_at > latest_prices.updated_at`,
		task.CurrencyPair, ratePrice, now,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert latest price in DB: %w", err)
	}

	log.Printf("[Worker] Successfully fetched live rate for %s: %f", task.CurrencyPair, ratePrice)
	return nil
}
