package main

import (
	"context"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/handler"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage"
)

func main() {

	// read configuration
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	logLevel, err := log.ParseLevel(conf.LogLevel)
	if err != nil {
		log.Fatalln(err)
	}
	log.SetLevel(logLevel)
	log.Infof("Log level: %s", conf.LogLevel)

	// connect to storage service
	storageSrv, err := storage.New(conf.Storage)
	if err != nil {
		log.Fatalln(err)
	}

	// configure http server
	server := &http.Server{
		Addr:    ":" + conf.Server.Port,
		Handler: handler.New(conf.Server, storageSrv),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// start server
	serverError := make(chan error)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Errorf("Catch server error: %v", err)
		}
		serverError <- err
	}()
	log.Infof("Server started on :%v, %v / %v",
		conf.Server.Port, conf.Server.ReadTimeout, conf.Server.IdleTimeout)

	// waiting http server error or Ctrl+C
	select {
	case <-serverError:
		_ = storageSrv.Close()
		// server already failed with error
		log.Infoln("Server stopped")
		return
	case <-stop:
		log.Infoln("Ctrl+C pressed")
		_ = storageSrv.Close()
		_ = server.Shutdown(context.Background())
		<-serverError // waiting server shutdown
		log.Infoln("Server stopped")
		return
	}
}
