package handler

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type Service interface {
	Save(string, *time.Time) (string, error)
	Load(string) (string, error)
	LoadInfo(string) (model.Item, error)
	Close() error
}

func New(conf config.Server, storage Service) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(conf.ReadTimeout.Duration))

	h := handler{conf.Schema, conf.Prefix, conf.Err404, storage, conf.Token}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("welcome")) })
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

	c, err := h.storage.Save(uri.String(), expires)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "Create handler error")
	}

	u := url.URL{
		Scheme: h.schema,
		Host:   h.host,
		Path:   c}

	log.Infof("Generated link: %v, %v", u.String(), time.Now().Sub(startAt))

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
