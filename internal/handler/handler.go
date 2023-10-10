package handler

import (
	"context"
	"encoding/json"
	"fmt"
	logger "github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
	log "github.com/sirupsen/logrus"
)

type Service interface {
	Save(string, *time.Time) (string, error)
	Load(string) (string, error)
	LoadInfo(string) (model.Item, error)
	Close() error
	Stat(ctx context.Context) (any, error)
}

func New(conf config.Server, storage Service) http.Handler {
	r := chi.NewRouter()

	routerLogger := log.New()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logger.Logger("router", routerLogger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(conf.ReadTimeout.Duration))

	h := handler{conf.Schema, conf.Prefix, conf.Err404, storage, conf.Token}
	r.Get("/health", h.health)
	r.Post("/", responseHandler(h.create))
	r.Get("/{shortLink}/info", responseHandler(h.info))
	r.Get("/{shortLink}", h.redirect)
	return r
}

type createRequest struct {
	URL     string  `json:"url"`
	Expires *string `json:"expires"`
}

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type handler struct {
	schema  string
	host    string
	err404  string
	storage Service
	token   string
}

func responseHandler(h func(r *http.Request) (interface{}, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, status, err := h(r)
		if err != nil {
			log.Errorf("Can't execute handler: %+v", err)
			data = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		err = json.NewEncoder(w).Encode(response{Data: data, Success: err == nil})
		if err != nil {
			log.Errorf("Can't create response to output: %+v", err)
		}
	}
}

func (h handler) health(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryStat := struct {
		Alloc      string `json:"alloc"`
		TotalAlloc string `json:"totalAlloc"`
		Sys        string `json:"sys"`
		NumGC      uint32 `json:"numGC"`
	}{
		Alloc:      fmt.Sprintf("%v MiB", m.Alloc/1024/1024),
		TotalAlloc: fmt.Sprintf("%v MiB", m.TotalAlloc/1024/1024),
		Sys:        fmt.Sprintf("%v MiB", m.Sys/1024/1024),
		NumGC:      m.NumGC,
	}
	storageStat, err := h.storage.Stat(r.Context())
	if err != nil {
		h.sendHtmlError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bytes, err := json.Marshal(struct {
		Memory       any `json:"memory"`
		NumGoroutine int `json:"numGoroutine"`
		Storage      any `json:"storage"`
	}{
		Memory:       memoryStat,
		NumGoroutine: runtime.NumGoroutine(),
		Storage:      storageStat,
	})
	if err != nil {
		h.sendHtmlError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func (h handler) create(r *http.Request) (interface{}, int, error) {
	startAt := time.Now()
	token := r.Header.Get("X-Token")
	if token != h.token {
		return nil, http.StatusForbidden, errors.New("Access denied")
	}

	var request createRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "Can't read body of request")
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, http.StatusBadRequest, errors.Wrap(err, "Unable to info JSON request body")
	}

	uri, err := url.ParseRequestURI(request.URL)

	if err != nil {
		return nil, http.StatusBadRequest, errors.New("Invalid url")
	}

	var expires *time.Time
	if request.Expires != nil {
		exp, err := time.Parse(time.RFC3339, *request.Expires)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("Invalid expiration date")
		}
		expires = &exp
	}

	startStorageAt := time.Now()
	c, err := h.storage.Save(uri.String(), expires)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "Create handler error")
	}
	durationStorage := time.Now().Sub(startStorageAt)

	u := url.URL{
		Scheme: h.schema,
		Host:   h.host,
		Path:   c,
	}

	duration := time.Now().Sub(startAt)
	log.Infof("Generated link: %v, duration: %v, storage: %v", u.String(), duration, durationStorage)

	return u.String(), http.StatusCreated, nil
}

func (h handler) info(r *http.Request) (interface{}, int, error) {
	code := chi.URLParam(r, "shortLink")

	item, err := h.storage.LoadInfo(code)
	if err != nil {
		if errors.Is(err, model.ErrNoLink) {
			return nil, http.StatusNotFound, fmt.Errorf("URL not found")
		}
		return nil, http.StatusInternalServerError, errors.Wrap(err, "Info handler error")
	}

	return item, http.StatusOK, nil
}

func (h handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "shortLink")

	uri, err := h.storage.Load(code)
	if err != nil {
		if h.err404 != "" {
			http.Redirect(w, r, h.err404, http.StatusMovedPermanently)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`<h1 style="margin-top: 150px; text-align: center; font-size: 72px;">Page not found</h1>`))
		}
		return
	}

	http.Redirect(w, r, uri, http.StatusMovedPermanently)
}

func (h handler) sendHtmlError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(code)
	w.Write([]byte(message))
}
