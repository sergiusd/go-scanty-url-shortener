package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bluele/gcache"
	logger "github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sergiusd/go-scanty-url-shortener/internal/base62"
	"github.com/sergiusd/go-scanty-url-shortener/internal/metrics"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
	log "github.com/sirupsen/logrus"
)

type IService interface {
	Save(string, *time.Time) (string, error)
	Load(id uint64) (string, error)
	LoadInfo(string) (model.Item, error)
	Close() error
	Stat(ctx context.Context) (any, error)
}

type ICache interface {
	Set(key, value any) error
	Get(key any) (any, error)
	HitRate() float64
	HitCount() uint64
	MissCount() uint64
	LookupCount() uint64
}

func New(conf config.Server, storage IService, cache ICache) http.Handler {
	r := chi.NewRouter()

	routerLogger := log.New()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logger.Logger("router", routerLogger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(conf.ReadTimeout.Duration))

	prometheusHandler := promhttp.Handler()

	h := handler{
		schema:  conf.Schema,
		host:    conf.Prefix,
		err404:  conf.Err404,
		storage: storage,
		token:   conf.Token,
		cache:   cache,
	}
	r.Get("/health", h.health)
	r.Post("/", responseHandler(h.create))
	r.Get("/{shortLink}/info", responseHandler(h.info))
	r.Get("/{shortLink}", h.redirect)
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		prometheusHandler.ServeHTTP(w, r)
	})
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
	storage IService
	token   string
	cache   ICache
}

type health struct {
	Memory       healthMemory `json:"memory"`
	Cache        healthCache  `json:"cache"`
	NumGoroutine int          `json:"numGoroutine"`
	Storage      any          `json:"storage"`
}

type healthMemory struct {
	Alloc      string `json:"alloc"`
	TotalAlloc string `json:"totalAlloc"`
	Sys        string `json:"sys"`
	NumGC      uint32 `json:"numGC"`
}

type healthCache struct {
	LookupCount uint64 `json:"lookupCount"`
	HitCount    uint64 `json:"hitCount"`
	MissCount   uint64 `json:"missCount"`
	HitRate     string `json:"hitRate"`
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

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryStat := healthMemory{
		Alloc:      fmt.Sprintf("%v MiB", m.Alloc/1024/1024),
		TotalAlloc: fmt.Sprintf("%v MiB", m.TotalAlloc/1024/1024),
		Sys:        fmt.Sprintf("%v MiB", m.Sys/1024/1024),
		NumGC:      m.NumGC,
	}
	cacheStat := healthCache{
		LookupCount: h.cache.LookupCount(),
		HitCount:    h.cache.HitCount(),
		MissCount:   h.cache.MissCount(),
		HitRate:     fmt.Sprintf("%.4f%%", h.cache.HitRate()*100),
	}
	storageStat, err := h.storage.Stat(r.Context())
	if err != nil {
		h.sendHtmlError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bytes, err := json.Marshal(health{
		Memory:       memoryStat,
		Cache:        cacheStat,
		NumGoroutine: runtime.NumGoroutine(),
		Storage:      storageStat,
	})
	if err != nil {
		h.sendHtmlError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(bytes)
}

func (h *handler) create(r *http.Request) (interface{}, int, error) {
	metricStop := metrics.StartHistogramTimer(metrics.GenerateHistogram)
	defer metricStop()

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

func (h *handler) info(r *http.Request) (interface{}, int, error) {
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

func (h *handler) redirect(w http.ResponseWriter, r *http.Request) {
	metricStop := metrics.StartHistogramFactoryTimer()
	useCache := false
	defer func() {
		histogram := metrics.ViewNoCacheHistogram
		if useCache {
			histogram = metrics.ViewCacheHistogram
		}
		metricStop(histogram)
	}()

	code := chi.URLParam(r, "shortLink")

	decodedId, err := base62.Decode(code)
	if err != nil {
		h.sendHtmlError(
			w,
			`<h1 style="margin-top: 150px; text-align: center; font-size: 72px;">Can't decode code</h1>`,
			http.StatusInternalServerError,
		)
	}

	uri, useCache, err := h.getUriById(decodedId)

	if err != nil {
		if h.err404 != "" {
			http.Redirect(w, r, h.err404, http.StatusMovedPermanently)
		} else {
			h.sendHtmlError(
				w,
				`<h1 style="margin-top: 150px; text-align: center; font-size: 72px;">Page not found</h1>`,
				http.StatusNotFound,
			)
		}
		return
	}

	http.Redirect(w, r, uri, http.StatusMovedPermanently)
}

func (h *handler) getUriById(id uint64) (string, bool, error) {
	cachedUri, err := h.cache.Get(id)
	if err == nil {
		return cachedUri.(string), true, nil
	}
	if !errors.Is(err, gcache.KeyNotFoundError) {
		log.Errorf("Error on get long url from cache for %v: %v", id, err)
	}
	uri, err := h.storage.Load(id)
	if err != nil {
		return "", false, errors.Wrap(err, "Can't get uri from storage")
	}
	if err := h.cache.Set(id, uri); err != nil {
		log.Errorf("Error on set long url to cache for %v: %v", id, err)
	}
	return uri, false, err
}

func (h *handler) sendHtmlError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(message))
}
