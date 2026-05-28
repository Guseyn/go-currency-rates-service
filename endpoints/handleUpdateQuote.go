package endpoints

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/google/uuid"
)

func (e *Env) HandleUpdateQuote(writer http.ResponseWriter, request *http.Request) {
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
	var req structs.UpdateRequest
	encErr := json.NewDecoder(request.Body).Decode(&req)
	if encErr != nil || req.CurrencyPair == "" {
		http.Error(writer, `{"error": "Invalid request body or missing currency_pair"}`, http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	_, dbErr := e.DB.Exec(request.Context(),
		/*sql*/ `INSERT INTO update_jobs (id, currency_pair, status, updated_at) 
		 VALUES ($1, $2, $3, $4)`,
		jobID, req.CurrencyPair, "pending", time.Now(),
	)
	if dbErr != nil {
		log.Printf("Failed to insert pending job into DB: %v", dbErr)
		errMsg := fmt.Sprintf(`{"error": "Internal database error", "message": %q}`, dbErr.Error())
		http.Error(writer, errMsg, http.StatusInternalServerError)
		return
	}

	e.TaskQueue <- structs.Task{
		JobID:        jobID,
		CurrencyPair: req.CurrencyPair,
	}

	response := structs.UpdateResponse{ID: jobID}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	if encErr := json.NewEncoder(writer).Encode(response); encErr != nil {
		log.Printf("Failed to write response payload: %v", encErr)
	}
}
