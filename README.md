# Simple URL shortener on Golang + ( Redis | PostgreSQL )

## Configuration
See `config.json` and `config/config.go`. Environment variable overwrited json data.
For development environment you might use `config.local.json`

## Handlers

Create new shortest link:

    curl -d '{"url": "http://ya.ru"}' \
         -H "Content-Type: application/json" \
         -H "X-Token: changeme" \
         localhost:8080
    {"success":true,"data":"http://localhost:8080/O8KEZlAseeb"}

    or
    {"success":false,"data":"Could not store in database: dial tcp: lookup 127.0.0.1 on 192.168.1.1:53: no such host"}

Information about the item:

    curl localhost:8081/O8KEZlAseeb/info
    {"success":true,"data":{"id":894385949183117216,"url":"http://ya.ru","expires":null,"visits":0}}


Redirect short link to original:

    curl localhost:8080/O8KEZlAseeb -v
    ...
    < HTTP/1.1 301 Moved Permanently
    < Location: http://ya.ru/

## Build

    docker build -t shortener:last .

## Run

    # Go + Redis
    docker-compose -f docker-compose-redis.yml up --build

    # Go + Postresql
    docker-compose -f docker-compose-psql.yml up --build