FROM golang:1.20 as builder
WORKDIR /app
COPY . .
RUN cd cmd/sankarea && go build -o sankarea main.go

FROM debian:bullseye-slim
WORKDIR /app
COPY --from=builder /app/cmd/sankarea/sankarea .
COPY config ./config
COPY data ./data
CMD ["./sankarea"]
