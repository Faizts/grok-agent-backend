# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /grok-agent-server ./cmd/server

# Runtime stage
FROM alpine:3.20
LABEL io.grokagent.project="grokagent" io.grokagent.service="backend"

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /grok-agent-server .

EXPOSE 8080

CMD ["./grok-agent-server"]
