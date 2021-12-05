package main

import (
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/handler"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage"
)

func main() {

	// read configuration
	conf, err := config.FromFileAndEnv("./config.json")
	if err != nil {
		log.Fatalln(err)
	}

	logLevel, err := log.ParseLevel(conf.LogLevel)
	if err != nil {
		log.Fatalln(err)
	}
	log.SetLevel(logLevel)
	log.Infof("Log level: %s\n", conf.LogLevel)

	// connect to storage service
	storageSrv, err := storage.New(conf.Storage)
	if err != nil {
		log.Fatalln(err)
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
			log.Errorf("Catch server error: %v\n", err)
		}
		serverError <- err
	}()
	log.Infof("Server started on :%v, %v / %v\n",
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
		_ = server.Shutdown()
		<-serverError // waiting server shutdown
		log.Infoln("Server stopped")
		return
	}
}
