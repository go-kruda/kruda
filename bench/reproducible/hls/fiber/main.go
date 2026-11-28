package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var headers = []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "X-Origin-Request", "Cache-Control", "ETag", "Last-Modified"}
var pool = sync.Pool{New: func() any { b := make([]byte, 64<<10); return &b }}
var client = &http.Client{
	Transport:     &http.Transport{MaxIdleConns: 1024, MaxIdleConnsPerHost: 1024, IdleConnTimeout: 65 * time.Second, DisableCompression: true},
	Timeout:       8 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func upstream(origin, method, path, byteRange string) (*http.Response, error) {
	req, err := http.NewRequest(method, origin+path, nil)
	if err != nil {
		return nil, err
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	return client.Do(req)
}

type readerOnly struct{ io.Reader }
type writerOnly struct{ io.Writer }

func main() {
	if len(os.Args) != 4 {
		log.Fatal("mode address origin required")
	}
	mode, addr, origin := os.Args[1], os.Args[2], os.Args[3]
	if mode == "fiber" {
		app := fiber.New(fiber.Config{IdleTimeout: 65 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 8 * time.Second, DisableDefaultContentType: true})
		app.All("/media/:name", func(c fiber.Ctx) error {
			response, err := upstream(origin, c.Method(), c.Path(), c.Get("Range"))
			if err != nil {
				return c.SendStatus(http.StatusBadGateway)
			}
			for _, name := range headers {
				if v := response.Header.Get(name); v != "" {
					c.Set(name, v)
				}
			}
			c.Status(response.StatusCode)
			if c.Method() == "HEAD" {
				response.Body.Close()
				return nil
			}
			if response.ContentLength < 0 {
				response.Body.Close()
				return c.SendStatus(http.StatusBadGateway)
			}
			return c.SendStream(response.Body, int(response.ContentLength))
		})
		log.Fatal(app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}))
	}
	if mode != "pooled" {
		log.Fatal("bad mode")
	}
	server := &http.Server{Addr: addr, IdleTimeout: 65 * time.Second, ReadHeaderTimeout: 8 * time.Second, WriteTimeout: 8 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := upstream(origin, r.Method, r.URL.EscapedPath(), r.Header.Get("Range"))
		if err != nil {
			http.Error(w, "origin", 502)
			return
		}
		defer response.Body.Close()
		for _, name := range headers {
			if v := response.Header.Get(name); v != "" {
				w.Header().Set(name, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		if r.Method == "HEAD" {
			return
		}
		buf := pool.Get().(*[]byte)
		defer pool.Put(buf)
		n, err := io.CopyBuffer(writerOnly{w}, readerOnly{response.Body}, *buf)
		if err != nil || n != response.ContentLength {
			log.Print("partial copy: ", strconv.FormatInt(n, 10))
			panic(http.ErrAbortHandler)
		}
	})}
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.Serve(ln))
}
