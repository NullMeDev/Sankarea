FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /newsbot ./cmd/newsbot

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /newsbot /newsbot
COPY data/state.json /data/state.json
ENTRYPOINT ["/newsbot"]
