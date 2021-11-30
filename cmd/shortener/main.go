package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/valyala/fasthttp"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/handler"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage"
)

func main() {

	// read configuration
	conf, err := config.FromFileAndEnv("./config.json")
	if err != nil {
		log.Fatal(err)
	}

	// connect to storage service
	storageSrv, err := storage.New(conf.Storage)
	if err != nil {
		log.Fatal(err)
	}

	// configure http server
	server := &fasthttp.Server{
		Handler: handler.New(conf.Server, storageSrv).Handler,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// start server
	serverError := make(chan error, 1)
	isShuttingDown := false
	go func() {
		err := server.ListenAndServe(":" + conf.Server.Port)
		if isShuttingDown {
			serverError <- err
		}
	}()
	log.Printf("Server started on :%v", conf.Server.Port)

	// waiting http server error or Ctrl+C
loop:
	select {
	case err := <-serverError:
		log.Println(err)
		break loop
	case <-stop:
		log.Println("Ctrl+C pressed")
		break loop
	}
	isShuttingDown = true

	// shutdown
	_ = storageSrv.Close()
	_ = server.Shutdown()
	log.Println("Server stopped")
}
