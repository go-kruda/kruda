package main

// World represents a row from the TFB "world" table.
// IDs and RandomNumbers are in the range [1, 10000].
type World struct {
	ID           int32 `json:"id"`
	RandomNumber int32 `json:"randomNumber"`
}

// Fortune represents a row from the TFB "fortune" table.
// Messages may contain HTML special characters and UTF-8 multibyte characters.
type Fortune struct {
	ID      int32  `json:"id"`
	Message string `json:"message"`
}

// Pre-computed content-type byte slices for zero-alloc SendBytesWithType calls.
// Using []byte avoids string→[]byte conversion in SetContentTypeBytes on the fasthttp path.
var (
	ctJSON = []byte("application/json")
	ctText = []byte("text/plain")
	ctHTML = []byte("text/html; charset=utf-8")
)

// Pre-allocated response constants for zero-allocation handlers.
var jsonResponse = []byte(`{"message":"Hello, World!"}`)
var textResponse = []byte("Hello, World!")
