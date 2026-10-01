FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Output outside the source tree: ./bot is a Go package directory, so
# `go build -o bot` would place the binary *inside* it instead of creating a file.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/jamhouse-bot .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/jamhouse-bot /app/jamhouse-bot

ENV TZ=Europe/Kyiv
CMD ["/app/jamhouse-bot"]
