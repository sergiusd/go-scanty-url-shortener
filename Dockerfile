FROM golang:1.20.4-alpine3.16 as builder

WORKDIR /app

# for cache go mod depends
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go mod tidy && go build -ldflags '-w -extldflags "-static"' -o shortener ./cmd/shortener/main.go


FROM alpine:3.16
EXPOSE 8080
RUN adduser -D -H -h /app shortener && \
    mkdir -p /app  && \
    chown -R shortener:shortener /app
WORKDIR /app
USER shortener

COPY --chown=shortener --from=builder /app/shortener /app
COPY --chown=shortener --from=builder /app/config.json /app

CMD ["/app/shortener"]
