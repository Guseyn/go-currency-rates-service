package channel

import (
	"context"
	"log"
	"time"

	"github.com/Guseyn/go-currency-rates-service.git/endpoints"
)

func StartBackgroundWorker(e *endpoints.Env) {
	if e == nil || e.DB == nil {
		log.Println("Background worker structural startup error: env or DB client is nil")
		return
	}
	for task := range e.TaskQueue {
		log.Printf("[Worker] Picked up job %s for currency pair: %s", task.JobID, task.CurrencyPair)
		err := ProcessCurrencyUpdate(e, task)
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
