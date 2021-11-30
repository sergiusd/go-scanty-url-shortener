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
		Handler:     handler.New(conf.Server, storageSrv).Handler,
		ReadTimeout: conf.Server.ReadTimeout.Duration,
		IdleTimeout: conf.Server.IdleTimeout.Duration,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// start server
	serverError := make(chan error)
	go func() {
		err := server.ListenAndServe(":" + conf.Server.Port)
		if err != nil {
			log.Printf("Catch server error: %v\n", err)
		}
		serverError <- err
	}()
	log.Printf("Server started on :%v, %v / %v",
		conf.Server.Port, conf.Server.ReadTimeout, conf.Server.IdleTimeout)

	// waiting http server error or Ctrl+C
	select {
	case <-serverError:
		_ = storageSrv.Close()
		// server already failed with error
		log.Println("Server stopped")
		return
	case <-stop:
		log.Println("Ctrl+C pressed")
		_ = storageSrv.Close()
		_ = server.Shutdown()
		<-serverError // waiting server shutdown
		log.Println("Server stopped")
		return
	}
}
