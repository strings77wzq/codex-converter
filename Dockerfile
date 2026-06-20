FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /codex-converter ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /codex-converter /usr/local/bin/
COPY config.example.toml /etc/codex-converter/config.toml
EXPOSE 8080
CMD ["codex-converter", "-config", "/etc/codex-converter/config.toml"]
