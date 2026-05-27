package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/Guseyn/go-currency-rates-service.git/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Env struct {
	DB        *pgxpool.Pool
	TaskQueue chan structs.Task
}

func main() {
	r := chi.NewRouter()
	err := runMigrations()
	if err != nil {
		log.Fatalf("Error during database migration: %v\n", err)
	}
	dbPool := util.InitDbClient()
	taskChannel := make(chan structs.Task, 100)

	env := &Env{
		DB:        dbPool,
		TaskQueue: taskChannel,
	}
	for i := 1; i <= 3; i++ {
		go StartBackgroundWorker(env)
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Post("/rates/update", env.handleUpdateQuote)
	r.Get("/rates/job/{id}", env.handleGetJobStatus)
	r.Get("/rates/latest/{pair}", env.handleGetLatestQuote)
	r.Get("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "swagger.json")
	})
	
	http.ListenAndServe(":8080", r)
}

func (e *Env) handleGetLatestQuote(writer http.ResponseWriter, request *http.Request) {
	pair := chi.URLParam(request, "pair")
	if pair == "" {
		http.Error(writer, "Currency pair parameter is required", http.StatusBadRequest)
		return
	}
	var price float64
	var updatedAt time.Time
	err := e.DB.QueryRow(request.Context(),
		"SELECT price, updated_at FROM latest_prices WHERE currency_pair = $1",
		pair,
	).Scan(&price, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(writer, "Currency pair not found", http.StatusNotFound)
			return
		}
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}
	response := structs.LatestQuoteResponse{
		Price:     price,
		UpdatedAt: updatedAt,
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		// If encoding fails midway, log it or fallback
		http.Error(writer, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}

func (e *Env) handleGetJobStatus(writer http.ResponseWriter, request *http.Request) {
	jobID := chi.URLParam(request, "id")
	if jobID == "" {
		http.Error(writer, `{"error": "Job ID parameter is required"}`, http.StatusBadRequest)
		return
	}
	var currencyPair string
	var status string
	var updatedAt time.Time
	var nullPrice sql.NullFloat64
	err := e.DB.QueryRow(request.Context(),
		"SELECT currency_pair, status, price, updated_at FROM update_jobs WHERE id = $1",
		jobID,
	).Scan(&currencyPair, &status, &nullPrice, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(writer, `{"error": "Job tracking ID not found"}`, http.StatusNotFound)
			return
		}
		http.Error(writer, `{"error": "Internal database read error"}`, http.StatusInternalServerError)
		return
	}
	var responsePrice *float64
	if nullPrice.Valid {
		// If the DB value wasn't null, copy the inner value into a pointer variable
		val := nullPrice.Float64
		responsePrice = &val
	}
	response := structs.JobStatusResponse{
		ID:           jobID,
		CurrencyPair: currencyPair,
		Status:       status,
		Price:        responsePrice,
		UpdatedAt:    updatedAt,
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		http.Error(writer, `{"error": "Failed to serialize JSON payload"}`, http.StatusInternalServerError)
	}
}

func (e *Env) handleUpdateQuote(writer http.ResponseWriter, request *http.Request) {
	var req structs.UpdateRequest
	err := json.NewDecoder(request.Body).Decode(&req)
	if err != nil || req.CurrencyPair == "" {
		http.Error(writer, `{"error": "Invalid request body or missing currency_pair"}`, http.StatusBadRequest)
		return
	}
	jobID := uuid.New().String()
	_, err = e.DB.Exec(request.Context(),
		/*sql*/ `INSERT INTO update_jobs (id, currency_pair, status, updated_at) 
		 VALUES ($1, $2, $3, $4)`,
		jobID, req.CurrencyPair, "pending", time.Now(),
	)
	if err != nil {
		log.Printf("Failed to insert pending job into DB: %v", err)
		http.Error(writer, `{"error": "Internal database error"}`, http.StatusInternalServerError)
		return
	}
	e.TaskQueue <- structs.Task{
		JobID:        jobID,
		CurrencyPair: req.CurrencyPair,
	}
	response := structs.UpdateResponse{ID: jobID}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		log.Printf("Failed to write response payload: %v", err)
	}
}

func StartBackgroundWorker(e *Env) {
	for task := range e.TaskQueue {
		log.Printf("[Worker] Picked up job %s for currency pair: %s", task.JobID, task.CurrencyPair)
		err := processCurrencyUpdate(e, task)
		if err != nil {
			log.Printf("[Worker] Job %s failed: %v", task.JobID, err)
			_, dbErr := e.DB.Exec(context.Background(),
				"UPDATE update_jobs SET status = $1, updated_at = $2 WHERE id = $3",
				"failed", time.Now(), task.JobID,
			)
			if dbErr != nil {
				log.Printf("[Worker] Failed to update fail-status in DB: %v", dbErr)
			}
			continue
		}
		log.Printf("[Worker] Job %s successfully completed", task.JobID)
	}
}

func processCurrencyUpdate(e *Env, task structs.Task) error {
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
