FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
COPY runner ./runner

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./backend/cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./backend/cmd/migrate

RUN mkdir -p /out/artifacts/runner && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/artifacts/runner/runner-linux-amd64 ./runner/cmd/runner && \
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/artifacts/runner/runner-linux-arm64 ./runner/cmd/runner && \
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/artifacts/runner/runner-darwin-amd64 ./runner/cmd/runner && \
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/artifacts/runner/runner-darwin-arm64 ./runner/cmd/runner && \
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/artifacts/runner/runner-windows-amd64.exe ./runner/cmd/runner

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/artifacts /app/artifacts
COPY db/migrations /app/db/migrations

ENV BACKEND_ADDR=:8080
ENV MIGRATIONS_DIR=/app/db/migrations
ENV RUNNER_ARTIFACT_DIR=/app/artifacts/runner

EXPOSE 8080

CMD ["/app/server"]
