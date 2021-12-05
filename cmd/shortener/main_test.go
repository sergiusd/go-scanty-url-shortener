package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
)

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func Test_App(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	conf, err := config.FromFileAndEnv(wd + "/../../config.json")
	if err != nil {
		t.Fatal(err)
	}

	prefix := time.Now().Unix()
	gorutineCount := 50
	itemCount := 100
	resultList := make([]string, gorutineCount*itemCount)

	getIndex := func(g int, i int) int {
		return g*itemCount + i
	}
	getUrl := func(g int, i int) string {
		return fmt.Sprintf("https://ya.ru/?q=%d.%d.%d", prefix, g, i)
	}

	wg := &sync.WaitGroup{}
	for g := 0; g < gorutineCount; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < itemCount; i++ {
				shortUrl, err := createRequest(conf.Server.Port, conf.Server.Token, getUrl(g, i))
				if err != nil {
					panic(err)
				}
				resultList[getIndex(g, i)] = shortUrl
			}
		}(g)
	}
	wg.Wait()
	for g := 0; g < gorutineCount; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < itemCount; i++ {
				shortUrl := resultList[getIndex(g, i)]
				longUrl, err := redirectRequest(shortUrl)
				if err != nil {
					panic(err)
				}
				assert.Equal(t, getUrl(g, i), longUrl, "short = %v", shortUrl)
			}
		}(g)
	}
	wg.Wait()
}

func createRequest(port string, token string, url string) (string, error) {
	client := &http.Client{}
	r := bytes.NewReader([]byte(`{"url": "` + url + `"}`))
	req, _ := http.NewRequest("POST", "http://localhost:"+port+"/", r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	// Read body
	b, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	// Unmarshal
	var msg response
	err = json.Unmarshal(b, &msg)
	if err != nil {
		return "", err
	}
	if !msg.Success {
		return "", errors.New(msg.Data.(string))
	}
	return msg.Data.(string), nil
}

func redirectRequest(url string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 301 {
		return "", errors.New(fmt.Sprintf("Not redirect status code %v", resp.StatusCode))
	}
	return resp.Header.Get("Location"), nil
}
