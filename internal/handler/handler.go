package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type Service interface {
	Save(string, *time.Time) (string, error)
	Load(string) (string, error)
	LoadInfo(string) (model.Item, error)
	Close() error
}

func New(conf config.Server, storage Service) *router.Router {
	r := router.New()
	h := handler{conf.Schema, conf.Prefix, conf.Err404, storage, conf.Token}
	r.POST("/", responseHandler(h.create))
	r.GET("/{shortLink}/info", responseHandler(h.info))
	r.GET("/{shortLink}", h.redirect)
	return r
}

var createRequest struct {
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

func responseHandler(h func(ctx *fasthttp.RequestCtx) (interface{}, int, error)) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		data, status, err := h(ctx)
		if err != nil {
			data = err.Error()
		}
		ctx.Response.Header.Set("Content-Type", "application/json")
		ctx.Response.SetStatusCode(status)
		err = json.NewEncoder(ctx.Response.BodyWriter()).Encode(response{Data: data, Success: err == nil})
		if err != nil {
			log.Printf("could not create response to output: %v", err)
		}
	}
}

func (h handler) create(ctx *fasthttp.RequestCtx) (interface{}, int, error) {
	token := ctx.Request.Header.Peek("X-Token")
	if string(token) != h.token {
		return nil, http.StatusForbidden, errors.New("Access denied")
	}

	if err := json.Unmarshal(ctx.PostBody(), &createRequest); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("Unable to info JSON request body: %v", err)
	}

	uri, err := url.ParseRequestURI(createRequest.URL)

	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("Invalid url")
	}

	var expires *time.Time
	if createRequest.Expires != nil {
		exp, err := time.Parse(time.RFC3339, *createRequest.Expires)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("Invalid expiration date")
		}
		expires = &exp
	}

	c, err := h.storage.Save(uri.String(), expires)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("Could not store in database: %v", err)
	}

	u := url.URL{
		Scheme: h.schema,
		Host:   h.host,
		Path:   c}

	log.Printf("Generated link: %v \n", u.String())

	return u.String(), http.StatusCreated, nil
}

func (h handler) info(ctx *fasthttp.RequestCtx) (interface{}, int, error) {
	code := ctx.UserValue("shortLink").(string)

	item, err := h.storage.LoadInfo(code)
	if err != nil {
		log.Printf("Info handler error: %v", err)
		return nil, http.StatusNotFound, fmt.Errorf("URL not found")
	}

	return item, http.StatusOK, nil
}

func (h handler) redirect(ctx *fasthttp.RequestCtx) {
	code := ctx.UserValue("shortLink").(string)

	uri, err := h.storage.Load(code)
	if err != nil {
		if h.err404 != "" {
			ctx.Redirect(h.err404, http.StatusMovedPermanently)
		} else {
			ctx.Response.Header.Set("Content-Type", "text/html")
			ctx.Response.SetStatusCode(http.StatusNotFound)
			fmt.Fprintf(
				ctx.Response.BodyWriter(),
				`<h1 style="margin-top: 150px; text-align: center; font-size: 72px;">Page not found</h1>`,
			)
		}
		return
	}

	ctx.Redirect(uri, http.StatusMovedPermanently)
}
