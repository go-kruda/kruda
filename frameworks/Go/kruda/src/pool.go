package main

import "sync"

// Tiered buffer pools sized for TFB response distribution.
// Small: single World JSON (~50 bytes), Medium: ≤20 queries (~1KB), Large: up to 500 queries (~25KB).
// Pools store *[]byte to avoid interface allocation on Get/Put.
var (
	smallPool  = sync.Pool{New: func() any { b := make([]byte, 0, 1024); return &b }}
	mediumPool = sync.Pool{New: func() any { b := make([]byte, 0, 8192); return &b }}
	largePool  = sync.Pool{New: func() any { b := make([]byte, 0, 32768); return &b }}

	// worldSlicePool provides reusable []World slices for multi-query handlers.
	worldSlicePool = sync.Pool{New: func() any { s := make([]World, 0, 500); return &s }}

	// fortuneSlicePool provides reusable []Fortune slices for the fortunes handler.
	fortuneSlicePool = sync.Pool{New: func() any { s := make([]Fortune, 0, 16); return &s }}
)

// maxPoolBufferCap is the maximum buffer capacity allowed back into a pool.
// Buffers that grew beyond this are discarded to prevent pool bloat.
const maxPoolBufferCap = 65536

// GetBuffer returns a pooled buffer pointer appropriate for the expected size.
// Tier selection: ≤1024 → small, ≤8192 → medium, else → large.
func GetBuffer(expectedSize int) *[]byte {
	var bp *[]byte
	switch {
	case expectedSize <= 1024:
		bp = smallPool.Get().(*[]byte)
	case expectedSize <= 8192:
		bp = mediumPool.Get().(*[]byte)
	default:
		bp = largePool.Get().(*[]byte)
	}
	*bp = (*bp)[:0] // reset length, keep capacity
	return bp
}

// PutBuffer returns a buffer to the appropriate pool based on its capacity.
// Buffers with capacity > 64KB are discarded to prevent pool bloat.
func PutBuffer(buf *[]byte) {
	if buf == nil {
		return
	}
	c := cap(*buf)
	if c > maxPoolBufferCap {
		return // discard oversized buffers
	}
	*buf = (*buf)[:0]
	switch {
	case c <= 1024:
		smallPool.Put(buf)
	case c <= 8192:
		mediumPool.Put(buf)
	default:
		largePool.Put(buf)
	}
}

// GetWorldSlice returns a pooled []World slice with length 0 and capacity 500.
func GetWorldSlice() *[]World {
	sp := worldSlicePool.Get().(*[]World)
	*sp = (*sp)[:0]
	return sp
}

// PutWorldSlice returns a []World slice to the pool.
func PutWorldSlice(s *[]World) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	worldSlicePool.Put(s)
}

// int32 slice pool for updates handler UNNEST parameters.
var int32SlicePool = sync.Pool{New: func() any { s := make([]int32, 0, 500); return &s }}

// GetInt32Slice returns a pooled []int32 slice with length 0 and capacity 500.
func GetInt32Slice() *[]int32 {
	sp := int32SlicePool.Get().(*[]int32)
	*sp = (*sp)[:0]
	return sp
}

// PutInt32Slice returns a []int32 slice to the pool.
func PutInt32Slice(s *[]int32) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	int32SlicePool.Put(s)
}

// GetFortuneSlice returns a pooled []Fortune slice with length 0.
func GetFortuneSlice() *[]Fortune {
	sp := fortuneSlicePool.Get().(*[]Fortune)
	*sp = (*sp)[:0]
	return sp
}

// PutFortuneSlice returns a []Fortune slice to the pool.
func PutFortuneSlice(s *[]Fortune) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	fortuneSlicePool.Put(s)
}

// bitmapPool provides reusable [10001]bool bitmaps for unique ID generation.
// A [10001]bool (10KB) is cheaper than a map for 10,000 possible IDs.
var bitmapPool = sync.Pool{New: func() any { return new([10001]bool) }}

// GetBitmap returns a zeroed [10001]bool from the pool.
func GetBitmap() *[10001]bool {
	return bitmapPool.Get().(*[10001]bool)
}

// PutBitmap returns a bitmap to the pool. Caller must have already cleared used entries.
func PutBitmap(b *[10001]bool) {
	bitmapPool.Put(b)
}
