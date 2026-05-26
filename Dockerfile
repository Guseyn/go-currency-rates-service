FROM golang:1.26-alpine

WORKDIR /app

# Copy the go.mod and go.sum files to install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your source code
COPY . .

# Build the Go app
RUN go build -o main .

# Expose the port your server listens on
EXPOSE 8080

# Run the app
CMD ["./main"]