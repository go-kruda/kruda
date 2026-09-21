package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/binder"
)

type fieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param"`
	Message string `json:"message"`
	Value   string `json:"value"`
}

type errorResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Errors  []fieldError `json:"errors,omitempty"`
}

func failure(field, rule, param string, value any) fieldError {
	bound := "least"
	if rule == "max" {
		bound = "most"
	}
	return fieldError{field, rule, param, field + " must be at " + bound + " " + param, fmt.Sprint(value)}
}

type input5 struct {
	ID     int64   `query:"id" uri:"id"`
	Page   int     `query:"page"`
	Active bool    `query:"active"`
	Score  float64 `query:"score"`
	Name   string  `query:"name"`
}

func handle5(c fiber.Ctx) error {
	in := input5{ID: 7, Page: 1, Active: true, Score: 1.5, Name: "guest"}
	if err := c.Bind().Query(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	if err := c.Bind().URI(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	in.Name = strings.TrimSpace(in.Name)
	var errs []fieldError
	if !(in.ID >= 1) {
		errs = append(errs, failure("id", "min", "1", in.ID))
	}
	if !(in.Page >= 1) {
		errs = append(errs, failure("page", "min", "1", in.Page))
	}
	if !(in.Page <= 1000) {
		errs = append(errs, failure("page", "max", "1000", in.Page))
	}
	if !(in.Score >= 0) {
		errs = append(errs, failure("score", "min", "0", in.Score))
	}
	if !(in.Score <= 100) {
		errs = append(errs, failure("score", "max", "100", in.Score))
	}
	if !(len(in.Name) >= 1) {
		errs = append(errs, failure("name", "min", "1", in.Name))
	}
	if !(len(in.Name) <= 64) {
		errs = append(errs, failure("name", "max", "64", in.Name))
	}
	if len(errs) != 0 {
		return c.Status(422).JSON(errorResponse{422, "Validation failed", errs}, "application/json; charset=utf-8")
	}
	return c.JSON(&in, "application/json; charset=utf-8")
}

type input10 struct {
	ID      int64   `query:"id" uri:"id"`
	Page    int     `query:"page"`
	Active  bool    `query:"active"`
	Score   float64 `query:"score"`
	Name    string  `query:"name"`
	ID2     int64   `query:"id2"`
	Page2   int     `query:"page2"`
	Active2 bool    `query:"active2"`
	Score2  float64 `query:"score2"`
	Name2   string  `query:"name2"`
}

func handle10(c fiber.Ctx) error {
	in := input10{ID: 7, Page: 1, Active: true, Score: 1.5, Name: "guest", ID2: 7, Page2: 1, Active2: true, Score2: 1.5, Name2: "guest"}
	if err := c.Bind().Query(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	if err := c.Bind().URI(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Name2 = strings.TrimSpace(in.Name2)
	var errs []fieldError
	if !(in.ID >= 1) {
		errs = append(errs, failure("id", "min", "1", in.ID))
	}
	if !(in.Page >= 1) {
		errs = append(errs, failure("page", "min", "1", in.Page))
	}
	if !(in.Page <= 1000) {
		errs = append(errs, failure("page", "max", "1000", in.Page))
	}
	if !(in.Score >= 0) {
		errs = append(errs, failure("score", "min", "0", in.Score))
	}
	if !(in.Score <= 100) {
		errs = append(errs, failure("score", "max", "100", in.Score))
	}
	if !(len(in.Name) >= 1) {
		errs = append(errs, failure("name", "min", "1", in.Name))
	}
	if !(len(in.Name) <= 64) {
		errs = append(errs, failure("name", "max", "64", in.Name))
	}
	if !(in.ID2 >= 1) {
		errs = append(errs, failure("id2", "min", "1", in.ID2))
	}
	if !(in.Page2 >= 1) {
		errs = append(errs, failure("page2", "min", "1", in.Page2))
	}
	if !(in.Page2 <= 1000) {
		errs = append(errs, failure("page2", "max", "1000", in.Page2))
	}
	if !(in.Score2 >= 0) {
		errs = append(errs, failure("score2", "min", "0", in.Score2))
	}
	if !(in.Score2 <= 100) {
		errs = append(errs, failure("score2", "max", "100", in.Score2))
	}
	if !(len(in.Name2) >= 1) {
		errs = append(errs, failure("name2", "min", "1", in.Name2))
	}
	if !(len(in.Name2) <= 64) {
		errs = append(errs, failure("name2", "max", "64", in.Name2))
	}
	if len(errs) != 0 {
		return c.Status(422).JSON(errorResponse{422, "Validation failed", errs}, "application/json; charset=utf-8")
	}
	return c.JSON(&in, "application/json; charset=utf-8")
}

type input30 struct {
	ID      int64   `query:"id" uri:"id"`
	Page    int     `query:"page"`
	Active  bool    `query:"active"`
	Score   float64 `query:"score"`
	Name    string  `query:"name"`
	ID2     int64   `query:"id2"`
	Page2   int     `query:"page2"`
	Active2 bool    `query:"active2"`
	Score2  float64 `query:"score2"`
	Name2   string  `query:"name2"`
	ID3     int64   `query:"id3"`
	Page3   int     `query:"page3"`
	Active3 bool    `query:"active3"`
	Score3  float64 `query:"score3"`
	Name3   string  `query:"name3"`
	ID4     int64   `query:"id4"`
	Page4   int     `query:"page4"`
	Active4 bool    `query:"active4"`
	Score4  float64 `query:"score4"`
	Name4   string  `query:"name4"`
	ID5     int64   `query:"id5"`
	Page5   int     `query:"page5"`
	Active5 bool    `query:"active5"`
	Score5  float64 `query:"score5"`
	Name5   string  `query:"name5"`
	ID6     int64   `query:"id6"`
	Page6   int     `query:"page6"`
	Active6 bool    `query:"active6"`
	Score6  float64 `query:"score6"`
	Name6   string  `query:"name6"`
}

func handle30(c fiber.Ctx) error {
	in := input30{ID: 7, Page: 1, Active: true, Score: 1.5, Name: "guest", ID2: 7, Page2: 1, Active2: true, Score2: 1.5, Name2: "guest", ID3: 7, Page3: 1, Active3: true, Score3: 1.5, Name3: "guest", ID4: 7, Page4: 1, Active4: true, Score4: 1.5, Name4: "guest", ID5: 7, Page5: 1, Active5: true, Score5: 1.5, Name5: "guest", ID6: 7, Page6: 1, Active6: true, Score6: 1.5, Name6: "guest"}
	if err := c.Bind().Query(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	if err := c.Bind().URI(&in); err != nil {
		return c.Status(400).JSON(errorResponse{Code: 400, Message: err.Error()}, "application/json; charset=utf-8")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Name2 = strings.TrimSpace(in.Name2)
	in.Name3 = strings.TrimSpace(in.Name3)
	in.Name4 = strings.TrimSpace(in.Name4)
	in.Name5 = strings.TrimSpace(in.Name5)
	in.Name6 = strings.TrimSpace(in.Name6)
	var errs []fieldError
	if !(in.ID >= 1) {
		errs = append(errs, failure("id", "min", "1", in.ID))
	}
	if !(in.Page >= 1) {
		errs = append(errs, failure("page", "min", "1", in.Page))
	}
	if !(in.Page <= 1000) {
		errs = append(errs, failure("page", "max", "1000", in.Page))
	}
	if !(in.Score >= 0) {
		errs = append(errs, failure("score", "min", "0", in.Score))
	}
	if !(in.Score <= 100) {
		errs = append(errs, failure("score", "max", "100", in.Score))
	}
	if !(len(in.Name) >= 1) {
		errs = append(errs, failure("name", "min", "1", in.Name))
	}
	if !(len(in.Name) <= 64) {
		errs = append(errs, failure("name", "max", "64", in.Name))
	}
	if !(in.ID2 >= 1) {
		errs = append(errs, failure("id2", "min", "1", in.ID2))
	}
	if !(in.Page2 >= 1) {
		errs = append(errs, failure("page2", "min", "1", in.Page2))
	}
	if !(in.Page2 <= 1000) {
		errs = append(errs, failure("page2", "max", "1000", in.Page2))
	}
	if !(in.Score2 >= 0) {
		errs = append(errs, failure("score2", "min", "0", in.Score2))
	}
	if !(in.Score2 <= 100) {
		errs = append(errs, failure("score2", "max", "100", in.Score2))
	}
	if !(len(in.Name2) >= 1) {
		errs = append(errs, failure("name2", "min", "1", in.Name2))
	}
	if !(len(in.Name2) <= 64) {
		errs = append(errs, failure("name2", "max", "64", in.Name2))
	}
	if !(in.ID3 >= 1) {
		errs = append(errs, failure("id3", "min", "1", in.ID3))
	}
	if !(in.Page3 >= 1) {
		errs = append(errs, failure("page3", "min", "1", in.Page3))
	}
	if !(in.Page3 <= 1000) {
		errs = append(errs, failure("page3", "max", "1000", in.Page3))
	}
	if !(in.Score3 >= 0) {
		errs = append(errs, failure("score3", "min", "0", in.Score3))
	}
	if !(in.Score3 <= 100) {
		errs = append(errs, failure("score3", "max", "100", in.Score3))
	}
	if !(len(in.Name3) >= 1) {
		errs = append(errs, failure("name3", "min", "1", in.Name3))
	}
	if !(len(in.Name3) <= 64) {
		errs = append(errs, failure("name3", "max", "64", in.Name3))
	}
	if !(in.ID4 >= 1) {
		errs = append(errs, failure("id4", "min", "1", in.ID4))
	}
	if !(in.Page4 >= 1) {
		errs = append(errs, failure("page4", "min", "1", in.Page4))
	}
	if !(in.Page4 <= 1000) {
		errs = append(errs, failure("page4", "max", "1000", in.Page4))
	}
	if !(in.Score4 >= 0) {
		errs = append(errs, failure("score4", "min", "0", in.Score4))
	}
	if !(in.Score4 <= 100) {
		errs = append(errs, failure("score4", "max", "100", in.Score4))
	}
	if !(len(in.Name4) >= 1) {
		errs = append(errs, failure("name4", "min", "1", in.Name4))
	}
	if !(len(in.Name4) <= 64) {
		errs = append(errs, failure("name4", "max", "64", in.Name4))
	}
	if !(in.ID5 >= 1) {
		errs = append(errs, failure("id5", "min", "1", in.ID5))
	}
	if !(in.Page5 >= 1) {
		errs = append(errs, failure("page5", "min", "1", in.Page5))
	}
	if !(in.Page5 <= 1000) {
		errs = append(errs, failure("page5", "max", "1000", in.Page5))
	}
	if !(in.Score5 >= 0) {
		errs = append(errs, failure("score5", "min", "0", in.Score5))
	}
	if !(in.Score5 <= 100) {
		errs = append(errs, failure("score5", "max", "100", in.Score5))
	}
	if !(len(in.Name5) >= 1) {
		errs = append(errs, failure("name5", "min", "1", in.Name5))
	}
	if !(len(in.Name5) <= 64) {
		errs = append(errs, failure("name5", "max", "64", in.Name5))
	}
	if !(in.ID6 >= 1) {
		errs = append(errs, failure("id6", "min", "1", in.ID6))
	}
	if !(in.Page6 >= 1) {
		errs = append(errs, failure("page6", "min", "1", in.Page6))
	}
	if !(in.Page6 <= 1000) {
		errs = append(errs, failure("page6", "max", "1000", in.Page6))
	}
	if !(in.Score6 >= 0) {
		errs = append(errs, failure("score6", "min", "0", in.Score6))
	}
	if !(in.Score6 <= 100) {
		errs = append(errs, failure("score6", "max", "100", in.Score6))
	}
	if !(len(in.Name6) >= 1) {
		errs = append(errs, failure("name6", "min", "1", in.Name6))
	}
	if !(len(in.Name6) <= 64) {
		errs = append(errs, failure("name6", "max", "64", in.Name6))
	}
	if len(errs) != 0 {
		return c.Status(422).JSON(errorResponse{422, "Validation failed", errs}, "application/json; charset=utf-8")
	}
	return c.JSON(&in, "application/json; charset=utf-8")
}

func main() {
	fields := os.Getenv("BENCH_FIELDS")
	if fields != "5" && fields != "10" && fields != "30" {
		panic("BENCH_FIELDS must be 5, 10 or 30")
	}
	workersText := os.Getenv("BENCH_WORKERS")
	if workersText != "4" && workersText != "8" {
		panic("BENCH_WORKERS must be 4 or 8")
	}
	workers, _ := strconv.Atoi(workersText)
	if runtime.GOMAXPROCS(0) != workers {
		panic("GOMAXPROCS must match BENCH_WORKERS")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "18373"
	}
	binder.SetParserDecoder(binder.ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: false})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("request-source", "benchmark")
		return c.Next()
	}, func(c fiber.Ctx) error {
		c.Set("X-Benchmark", "bindgen")
		return c.Next()
	})
	switch fields {
	case "5":
		app.Get("/users/:id", handle5)
	case "10":
		app.Get("/users/:id", handle10)
	case "30":
		app.Get("/users/:id", handle30)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr := "127.0.0.1:" + port
	fmt.Printf("fiber=%s fields=%s workers=%d gomaxprocs=%d addr=%s\n", fiber.Version, fields, workers, runtime.GOMAXPROCS(0), addr)
	if err := app.Listen(addr, fiber.ListenConfig{
		DisableStartupMessage: true,
		GracefulContext:       ctx,
		ShutdownTimeout:       5 * time.Second,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
