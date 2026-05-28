package endpoints

import (
	"github.com/Guseyn/go-currency-rates-service.git/structs"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Env struct {
	DB        *pgxpool.Pool
	TaskQueue chan structs.Task
}
