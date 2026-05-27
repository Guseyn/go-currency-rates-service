package structs

import "time"

type JobStatusResponse struct {
	ID           string    `json:"id"`
	CurrencyPair string    `json:"currency_pair"`
	Status       string    `json:"status"`
	Price        *float64  `json:"price,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
