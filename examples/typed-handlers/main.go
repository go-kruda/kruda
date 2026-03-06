// Example: Typed Handlers with C[T]
//
// This example demonstrates Kruda's typed handler system using C[T] — the
// generic typed context. Typed handlers auto-parse request inputs from
// multiple sources (path params, query strings, JSON body) into a single
// struct, then auto-validate using struct tags.
//
// Key concepts:
//   - C[T] typed context with auto-parsed c.In field
//   - `param` tag — binds from URL path parameters (:id, :slug)
//   - `query` tag — binds from query string (?page=1&limit=10)
//   - `json`  tag — binds from JSON request body
//   - `validate` tag — auto-validation (required, min, max, email, etc.)
//   - Multiple input sources combined in one struct
//
// Endpoints:
//
//	GET    /products/:id          — path param binding
//	GET    /products              — query param binding (search, pagination)
//	POST   /products              — JSON body binding with validation
//	PUT    /products/:id          — combined: path param + JSON body
//	GET    /products/:id/reviews  — nested path params + query params
//
// Run:
//
//	go run -tags kruda_stdjson ./examples/typed-handlers/
//
// Test:
//
//	curl http://localhost:3000/products/42
//	curl "http://localhost:3000/products?search=phone&page=1&limit=5"
//	curl -X POST http://localhost:3000/products -H 'Content-Type: application/json' \
//	     -d '{"name":"Laptop","price":999.99,"category":"electronics"}'
//	curl -X PUT http://localhost:3000/products/42 -H 'Content-Type: application/json' \
//	     -d '{"name":"Updated Laptop","price":1099.99}'
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain model
// ---------------------------------------------------------------------------

// Product is the domain entity returned in API responses.
type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
}

// Review represents a product review.
type Review struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Author    string `json:"author"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
}

// ---------------------------------------------------------------------------
// In-memory store
// ---------------------------------------------------------------------------

type Store struct {
	mu       sync.RWMutex
	products map[string]Product
	reviews  map[string][]Review
	seq      int
}

func NewStore() *Store {
	s := &Store{
		products: make(map[string]Product),
		reviews:  make(map[string][]Review),
	}
	// Seed some data
	s.products["1"] = Product{ID: "1", Name: "Laptop", Price: 999.99, Category: "electronics"}
	s.products["2"] = Product{ID: "2", Name: "Go Book", Price: 39.99, Category: "books"}
	s.products["3"] = Product{ID: "3", Name: "Keyboard", Price: 79.99, Category: "electronics"}
	s.reviews["1"] = []Review{
		{ID: "r1", ProductID: "1", Author: "Alice", Rating: 5, Comment: "Great laptop!"},
		{ID: "r2", ProductID: "1", Author: "Bob", Rating: 4, Comment: "Good value"},
	}
	s.seq = 3
	return s
}

// ---------------------------------------------------------------------------
// Typed handler input structs
//
// Each struct defines what the handler expects. Tags tell Kruda where to
// parse each field from:
//   - `param:"name"` → URL path parameter
//   - `query:"name"` → query string parameter
//   - `json:"name"`  → JSON request body field
//   - `validate:"rules"` → validation rules
// ---------------------------------------------------------------------------

// GetProductInput binds the :id path parameter.
// Used by GET /products/:id — Kruda extracts "42" from /products/42.
type GetProductInput struct {
	ID string `param:"id" validate:"required"`
}

// ListProductsInput binds query string parameters for search and pagination.
// Used by GET /products?search=phone&page=1&limit=5
type ListProductsInput struct {
	Search   string `query:"search"`                 // optional search term
	Category string `query:"category"`               // optional category filter
	Page     int    `query:"page" validate:"min=1"`  // page number (min 1)
	Limit    int    `query:"limit" validate:"min=1"` // items per page (min 1)
}

// CreateProductInput binds JSON body fields with validation.
// Used by POST /products — all fields parsed from request body.
type CreateProductInput struct {
	Name     string  `json:"name" validate:"required,min=2,max=100"`    // product name (2-100 chars)
	Price    float64 `json:"price" validate:"required"`                 // price (required)
	Category string  `json:"category" validate:"required,min=2,max=50"` // category (2-50 chars)
}

// UpdateProductInput combines path param + JSON body.
// Kruda parses ID from the URL and Name/Price from the JSON body.
type UpdateProductInput struct {
	ID       string  `param:"id" validate:"required"`        // from URL: /products/:id
	Name     string  `json:"name" validate:"required,min=2"` // from body
	Price    float64 `json:"price" validate:"required"`      // from body
	Category string  `json:"category"`                       // from body (optional on update)
}

// GetReviewsInput combines path params + query params.
// Used by GET /products/:id/reviews?sort=rating&limit=10
type GetReviewsInput struct {
	ProductID string `param:"id" validate:"required"` // from URL
	Sort      string `query:"sort"`                   // optional: "rating" or "date"
	Limit     int    `query:"limit" validate:"min=1"` // items per page
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// ProductListResponse wraps a list of products with pagination metadata.
type ProductListResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

// ReviewListResponse wraps a list of reviews.
type ReviewListResponse struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	store := NewStore()

	// Create app with validation enabled.
	// NewValidator() provides built-in rules: required, min, max, email, etc.
	app := kruda.New(
		kruda.WithValidator(kruda.NewValidator()),
	)

	// Global middleware
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())

	// -----------------------------------------------------------------------
	// Endpoint 1: GET /products/:id — Path parameter binding
	//
	// C[GetProductInput] auto-extracts :id from the URL path.
	// Access the parsed value via c.In.ID.
	// -----------------------------------------------------------------------
	kruda.Get[GetProductInput, Product](app, "/products/:id", func(c *kruda.C[GetProductInput]) (*Product, error) {
		product, ok := store.Get(c.Context(), c.In.ID)
		if !ok {
			return nil, kruda.NotFound("product not found")
		}
		return &product, nil
	})

	// -----------------------------------------------------------------------
	// Endpoint 2: GET /products — Query parameter binding
	//
	// C[ListProductsInput] auto-extracts search, category, page, limit
	// from the query string. Validation ensures page >= 1 and limit >= 1.
	// -----------------------------------------------------------------------
	kruda.Get[ListProductsInput, ProductListResponse](app, "/products", func(c *kruda.C[ListProductsInput]) (*ProductListResponse, error) {
		// Apply defaults for optional pagination params
		page := c.In.Page
		if page == 0 {
			page = 1
		}
		limit := c.In.Limit
		if limit == 0 {
			limit = 10
		}

		products := store.List(c.Context(), c.In.Search, c.In.Category)

		// Simple pagination
		start := (page - 1) * limit
		if start > len(products) {
			start = len(products)
		}
		end := start + limit
		if end > len(products) {
			end = len(products)
		}

		return &ProductListResponse{
			Products: products[start:end],
			Total:    len(products),
			Page:     page,
			Limit:    limit,
		}, nil
	})

	// -----------------------------------------------------------------------
	// Endpoint 3: POST /products — JSON body binding with validation
	//
	// C[CreateProductInput] auto-parses the JSON body. Validation runs
	// automatically — if name is empty or price is missing, Kruda returns
	// a 422 Unprocessable Entity with field-level error details.
	// -----------------------------------------------------------------------
	kruda.Post[CreateProductInput, Product](app, "/products", func(c *kruda.C[CreateProductInput]) (*Product, error) {
		product := store.Create(c.Context(), Product{
			Name:     c.In.Name,
			Price:    c.In.Price,
			Category: c.In.Category,
		})
		c.Status(201) // Created
		return &product, nil
	})

	// -----------------------------------------------------------------------
	// Endpoint 4: PUT /products/:id — Combined: path param + JSON body
	//
	// C[UpdateProductInput] parses ID from the URL path AND Name/Price
	// from the JSON body — all in one struct. This is the power of C[T]:
	// multiple input sources unified into a single typed struct.
	// -----------------------------------------------------------------------
	kruda.Put[UpdateProductInput, Product](app, "/products/:id", func(c *kruda.C[UpdateProductInput]) (*Product, error) {
		product, ok := store.Update(c.Context(), c.In.ID, Product{
			Name:     c.In.Name,
			Price:    c.In.Price,
			Category: c.In.Category,
		})
		if !ok {
			return nil, kruda.NotFound("product not found")
		}
		return &product, nil
	})

	// -----------------------------------------------------------------------
	// Endpoint 5: GET /products/:id/reviews — Nested params + query
	//
	// C[GetReviewsInput] extracts :id from the path and sort/limit from
	// the query string. Demonstrates combining path params with query
	// params in a single typed struct.
	// -----------------------------------------------------------------------
	kruda.Get[GetReviewsInput, ReviewListResponse](app, "/products/:id/reviews", func(c *kruda.C[GetReviewsInput]) (*ReviewListResponse, error) {
		reviews := store.GetReviews(c.Context(), c.In.ProductID)

		limit := c.In.Limit
		if limit == 0 {
			limit = 20
		}
		if limit > len(reviews) {
			limit = len(reviews)
		}

		return &ReviewListResponse{
			Reviews: reviews[:limit],
			Total:   len(reviews),
		}, nil
	})

	// Start the server
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}

// ---------------------------------------------------------------------------
// Store methods
// ---------------------------------------------------------------------------

func (s *Store) Get(_ context.Context, id string) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *Store) List(_ context.Context, search, category string) []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		if search != "" && !contains(p.Name, search) {
			continue
		}
		if category != "" && p.Category != category {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (s *Store) Create(_ context.Context, p Product) Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	p.ID = fmt.Sprintf("%d", s.seq)
	s.products[p.ID] = p
	return p
}

func (s *Store) Update(_ context.Context, id string, p Product) (Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.products[id]
	if !ok {
		return Product{}, false
	}
	p.ID = id
	if p.Category == "" {
		p.Category = existing.Category
	}
	s.products[id] = p
	return p, true
}

func (s *Store) GetReviews(_ context.Context, productID string) []Review {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reviews[productID]
}

// contains is a simple case-insensitive substring check.
func contains(s, substr string) bool {
	// Simple ASCII lowercase comparison
	sl := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sl[i] = c
	}
	subl := make([]byte, len(substr))
	for i := range substr {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		subl[i] = c
	}
	return len(subl) == 0 || indexOf(sl, subl) >= 0
}

func indexOf(s, sub []byte) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
