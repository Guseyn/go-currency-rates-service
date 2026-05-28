package endpoints

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (e *Env) HandleGetJobStatus(writer http.ResponseWriter, request *http.Request) {
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
	jobID := chi.URLParam(request, "id")
	if jobID == "" {
		http.Error(writer, `{"error": "Job ID parameter is required"}`, http.StatusBadRequest)
		return
	}

	var currencyPair string
	var status string
	var updatedAt time.Time
	var nullPrice sql.NullFloat64

	dbErr := e.DB.QueryRow(
		request.Context(),
		"SELECT currency_pair, status, price, updated_at FROM update_jobs WHERE id = $1",
		jobID,
	).Scan(&currencyPair, &status, &nullPrice, &updatedAt)
	if dbErr != nil {
		if errors.Is(dbErr, pgx.ErrNoRows) {
			http.Error(writer, `{"error": "Job tracking ID not found"}`, http.StatusNotFound)
			return
		}
		errMsg := fmt.Sprintf(`{"error": "Internal database read error", "message": %q}`, dbErr.Error())
		http.Error(writer, errMsg, http.StatusInternalServerError)
		return
	}

	var responsePrice *float64
	if nullPrice.Valid {
		responsePrice = new(nullPrice.Float64)
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
	if encErr := json.NewEncoder(writer).Encode(response); encErr != nil {
		http.Error(writer, `{"error": "Failed to serialize JSON payload"}`, http.StatusInternalServerError)
	}
}
