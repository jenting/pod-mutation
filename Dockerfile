FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go build -o webhook .
ENTRYPOINT ["/webhook"]
