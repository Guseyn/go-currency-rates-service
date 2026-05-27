package structs

import "time"

type LatestQuoteResponse struct {
	Price     float64   `json:"price"`
	UpdatedAt time.Time `json:"updated_at"`
}
