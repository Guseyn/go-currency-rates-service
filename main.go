package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {

	r := chi.NewRouter()
	err := runMigrations()
	if err != nil {
		log.Fatalf("Error during database migration: %v\n", err)
	}
	dbPool := initDbClient()
	env := &Env{DB: dbPool}

	r.Post("/rates/update", env.handleUpdateQuote)
	r.Get("/rates/job/{id}", env.handleGetJobStatus)
	r.Get("/rates/latest/{pair}", env.handleGetLatestQuote)

	http.ListenAndServe(":8080", r)
}

func (e *Env) handleGetLatestQuote(writer http.ResponseWriter, request *http.Request) {

	writer.WriteHeader(http.StatusAccepted)
	writer.Write([]byte(`{"message": "Job received"}`))
}

func (e *Env) handleGetJobStatus(writer http.ResponseWriter, request *http.Request) {

}

func (e *Env) handleUpdateQuote(writer http.ResponseWriter, request *http.Request) {

}
