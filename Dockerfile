FROM golang:1.17-alpine as builder

WORKDIR /app

# for cache go mod depends
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o shortener ./cmd/shortener/main.go


FROM alpine
EXPOSE 8080 8081
RUN adduser -D -H -h /app shortener && \
    mkdir -p /app  && \
    chown -R shortener:shortener /app
WORKDIR /app
USER shortener

COPY --chown=shortener --from=builder /app/shortener /app
COPY --chown=shortener --from=builder /app/config.json /app

CMD ["/app/shortener", "/app/bin/config.json"]
