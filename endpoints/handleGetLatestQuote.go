package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (e *Env) HandleGetLatestQuote(writer http.ResponseWriter, request *http.Request) {
	if e == nil {
		log.Println("CRITICAL PANIC PREVENTION: The Env receiver struct is nil!")
		http.Error(writer, `{"error": "Environment layer context lost"}`, http.StatusInternalServerError)
		return
	}
	if e.DB == nil {
		log.Println("CRITICAL PANIC PREVENTION: e.DB database pool connection is nil!")
		http.Error(writer, `{"error": "Database pool unavailable"}`, http.StatusInternalServerError)
		return
	}
	rawPair := chi.URLParam(request, "pair")
	pair, unescErr := url.PathUnescape(rawPair)
	if unescErr != nil {
		writer.Header().Set("Content-Type", "application/json")
		http.Error(writer, `{"error": "Malformed currency pair encoding"}`, http.StatusBadRequest)
		return
	}
	log.Printf("pair", pair)
	var price float64
	var updatedAt time.Time
	dbErr := e.DB.QueryRow(request.Context(),
		"SELECT price, updated_at FROM latest_prices WHERE currency_pair = $1",
		pair,
	).Scan(&price, &updatedAt)
	if dbErr != nil {
		if errors.Is(dbErr, pgx.ErrNoRows) {
			http.Error(writer, "Currency pair not found", http.StatusNotFound)
			return
		}
		errMsg := fmt.Sprintf(`{"error": "Internal database read error", "message": %q}`, dbErr.Error())
		http.Error(writer, errMsg, http.StatusInternalServerError)
		return
	}
	response := structs.LatestQuoteResponse{
		Price:     price,
		UpdatedAt: updatedAt,
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	if encErr := json.NewEncoder(writer).Encode(response); encErr != nil {
		// If encoding fails midway, log it or fallback
		http.Error(writer, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}
