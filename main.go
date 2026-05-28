package main

import (
	"log"
	"net/http"

	"github.com/Guseyn/go-currency-rates-service.git/endpoints"
	"github.com/Guseyn/go-currency-rates-service.git/endpoints/channel"
	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/Guseyn/go-currency-rates-service.git/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	r := chi.NewRouter()
	err := runMigrations()
	if err != nil {
		log.Fatalf("Error during database migration: %v\n", err)
	}
	dbPool := util.InitDbClient()
	defer dbPool.Close()
	taskChannel := make(chan structs.Task, 100)

	env := &endpoints.Env{
		DB:        dbPool,
		TaskQueue: taskChannel,
	}
	for i := 1; i <= 3; i++ {
		go channel.StartBackgroundWorker(env)
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Post("/rates/update", env.HandleUpdateQuote)
	r.Get("/rates/job/{id}", env.HandleGetJobStatus)
	r.Get("/rates/latest/{pair}", env.HandleGetLatestQuote)
	r.Get("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "swagger.json")
	})

	http.ListenAndServe(":8080", r)
}
