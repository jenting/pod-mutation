FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o webhook .
ENTRYPOINT ["/webhook"]
