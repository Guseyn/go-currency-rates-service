-- migrations/000001_init_schema.up.sql

CREATE TABLE IF NOT EXISTS update_jobs (
    id UUID PRIMARY KEY,
    currency_pair VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL,
    price NUMERIC,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS latest_prices (
    currency_pair VARCHAR(10) PRIMARY KEY,
    price NUMERIC NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);