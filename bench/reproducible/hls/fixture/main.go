package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-kruda/kruda"
)

const segmentBytes = 3_000_000

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: bench origin ADDRESS | wing ADDRESS ORIGIN | nethttp ADDRESS ORIGIN | load URL VIEWERS WARM_CYCLES MEASURE_CYCLES [close]")
	}
	mode, addr := os.Args[1], os.Args[2]
	switch mode {
	case "origin":
		runOrigin(addr)
	case "wing", "nethttp":
		runProxy(mode, addr, os.Args[3])
	case "load":
		runLoad(addr, mustInt(os.Args[3]), mustInt(os.Args[4]), mustInt(os.Args[5]), len(os.Args) > 6)
	default:
		log.Fatal("unknown mode: ", mode)
	}
}

func mustInt(text string) int {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 {
		log.Fatal("expected positive integer: ", text)
	}
	return n
}

func runOrigin(addr string) {
	fixtures := make([][]byte, 8)
	random := rand.NewChaCha8([32]byte{11, 22, 26})
	for i := range fixtures {
		fixtures[i] = make([]byte, segmentBytes)
		random.Read(fixtures[i])
	}
	var requests atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, requests.Load())
	})
	mux.HandleFunc("GET /media/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if len(name) != 4 || name[1:] != ".ts" || name[0] < '0' || name[0] > '7' {
			http.NotFound(w, r)
			return
		}
		seq := requests.Add(1)
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("ETag", fmt.Sprintf("\"fixture-%c\"", name[0]))
		w.Header().Set("X-Origin-Request", strconv.FormatInt(seq, 10))
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(fixtures[name[0]-'0']))
	})
	server := http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func runProxy(mode, addr, origin string) {
	transport := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	var app *kruda.App
	if mode == "wing" {
		app = kruda.New(kruda.Wing())
	} else {
		app = kruda.New(kruda.NetHTTP())
	}
	stream := func(c *kruda.Ctx) error {
		name := c.Param("name")
		for _, char := range name {
			if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '.') {
				return c.Status(http.StatusBadRequest).Text("bad name")
			}
		}
		req, err := http.NewRequestWithContext(c.Context(), c.Method(), origin+"/media/"+name, nil)
		if err != nil {
			return c.Status(http.StatusBadGateway).Text("origin request")
		}
		if value := c.Header("Range"); value != "" {
			req.Header.Set("Range", value)
		}
		res, err := client.Do(req)
		if err != nil {
			return c.Status(http.StatusBadGateway).Text("origin response")
		}
		defer res.Body.Close()
		for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "X-Origin-Request"} {
			if value := res.Header.Get(name); value != "" {
				c.SetHeader(name, value)
			}
		}
		if c.Method() == http.MethodHead {
			return c.Status(res.StatusCode).SendBytes(nil)
		}
		return c.Status(res.StatusCode).Stream(res.Body)
	}
	streamPreset := kruda.Stream
	if mode == "wing" {
		streamPreset = kruda.Stream.With(kruda.StreamKeepAlive())
	}
	app.Get("/media/:name", stream, streamPreset)
	app.Head("/media/:name", stream, kruda.Stream)
	log.Fatal(app.Listen(addr))
}

func printJSON(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		log.Fatal(err)
	}
}
