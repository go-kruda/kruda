package wing

import (
	"reflect"
	"testing"
)

func featherEqual(a, b Feather) bool { return reflect.DeepEqual(a, b) }

func TestFeatherDefaults(t *testing.T) {
	var f Feather
	f.defaults()
	want := Feather{
		Dispatch: Inline,
		Engine:   Epoll,
		Response: DirectWrite,
		Buffer:   Fixed,
		Conn:     KeepAlive,
	}
	if !featherEqual(f, want) {
		t.Errorf("defaults() = %+v, want %+v", f, want)
	}
}

func TestFeatherWith(t *testing.T) {
	// Override single axis from Arrow.
	f := Arrow.With(Buffer(Grow))
	if f.Buffer != Grow {
		t.Errorf("Buffer = %v, want Grow", f.Buffer)
	}
	// Other axes unchanged.
	if f.Dispatch != Arrow.Dispatch {
		t.Errorf("Dispatch = %v, want %v", f.Dispatch, Arrow.Dispatch)
	}
	if f.Engine != Arrow.Engine {
		t.Errorf("Engine = %v, want %v", f.Engine, Arrow.Engine)
	}
	if f.Response != Arrow.Response {
		t.Errorf("Response = %v, want %v", f.Response, Arrow.Response)
	}
	if f.Conn != Arrow.Conn {
		t.Errorf("Conn = %v, want %v", f.Conn, Arrow.Conn)
	}
}

func TestFeatherWithMultiple(t *testing.T) {
	f := Bolt.With(Dispatch(Pool), Buffer(Fixed), Conn(KeepAlive))
	want := Feather{
		Dispatch: Pool,
		Engine:   Epoll,
		Response: Direct,
		Buffer:   Fixed,
		Conn:     KeepAlive,
	}
	if !featherEqual(f, want) {
		t.Errorf("With(multiple) = %+v, want %+v", f, want)
	}
}

func TestFeatherWithDoesNotMutateOriginal(t *testing.T) {
	original := Arrow
	_ = Arrow.With(Buffer(Grow))
	if !featherEqual(Arrow, original) {
		t.Errorf("With() mutated original preset: got %+v, want %+v", Arrow, original)
	}
}

func TestPresetValues(t *testing.T) {
	tests := []struct {
		name string
		f    Feather
		want Feather
	}{
		{"Bolt", Bolt, Feather{Dispatch: Inline, Engine: Epoll, Response: Direct, Buffer: ZeroCopy, Conn: Pipeline}},
		{"Flash", Flash, Feather{Dispatch: Inline, Engine: Epoll, Response: Direct, Buffer: Fixed, Conn: KeepAlive}},
		{"Arrow", Arrow, Feather{Dispatch: Pool, Engine: Epoll, Response: DirectWrite, Buffer: Fixed, Conn: KeepAlive}},
		{"Hawk", Hawk, Feather{Dispatch: Pool, Engine: Epoll, Response: Writeback, Buffer: Grow, Conn: KeepAlive}},
		{"Glide", Glide, Feather{Dispatch: Persist, Engine: Epoll, Response: Chunked, Buffer: Stream, Conn: Upgrade}},
		{"Talon", Talon, Feather{Dispatch: Pool, Engine: IOURing, Response: Sendfile, Buffer: Registered, Conn: OneShot}},
		{"Soar", Soar, Feather{Dispatch: Spawn, Engine: Net, Response: Direct, Buffer: Grow, Conn: KeepAlive}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !featherEqual(tt.f, tt.want) {
				t.Errorf("%s = %+v, want %+v", tt.name, tt.f, tt.want)
			}
		})
	}
}

func TestStringers(t *testing.T) {
	// Dispatch
	if s := Inline.String(); s != "Inline" {
		t.Errorf("Inline.String() = %q", s)
	}
	if s := Pool.String(); s != "Pool" {
		t.Errorf("Pool.String() = %q", s)
	}
	if s := Spawn.String(); s != "Spawn" {
		t.Errorf("Spawn.String() = %q", s)
	}
	if s := Persist.String(); s != "Persist" {
		t.Errorf("Persist.String() = %q", s)
	}
	// Engine
	if s := Epoll.String(); s != "Epoll" {
		t.Errorf("Epoll.String() = %q", s)
	}
	if s := IOURing.String(); s != "IOURing" {
		t.Errorf("IOURing.String() = %q", s)
	}
	if s := Splice.String(); s != "Splice" {
		t.Errorf("Splice.String() = %q", s)
	}
	if s := Net.String(); s != "Net" {
		t.Errorf("Net.String() = %q", s)
	}
	// Response
	if s := Direct.String(); s != "Direct" {
		t.Errorf("Direct.String() = %q", s)
	}
	if s := Writeback.String(); s != "Writeback" {
		t.Errorf("Writeback.String() = %q", s)
	}
	if s := DirectWrite.String(); s != "DirectWrite" {
		t.Errorf("DirectWrite.String() = %q", s)
	}
	if s := Batch.String(); s != "Batch" {
		t.Errorf("Batch.String() = %q", s)
	}
	if s := Chunked.String(); s != "Chunked" {
		t.Errorf("Chunked.String() = %q", s)
	}
	if s := Sendfile.String(); s != "Sendfile" {
		t.Errorf("Sendfile.String() = %q", s)
	}
	// Buffer
	if s := Fixed.String(); s != "Fixed" {
		t.Errorf("Fixed.String() = %q", s)
	}
	if s := Grow.String(); s != "Grow" {
		t.Errorf("Grow.String() = %q", s)
	}
	if s := ZeroCopy.String(); s != "ZeroCopy" {
		t.Errorf("ZeroCopy.String() = %q", s)
	}
	if s := Stream.String(); s != "Stream" {
		t.Errorf("Stream.String() = %q", s)
	}
	if s := Registered.String(); s != "Registered" {
		t.Errorf("Registered.String() = %q", s)
	}
	// Conn
	if s := Pipeline.String(); s != "Pipeline" {
		t.Errorf("Pipeline.String() = %q", s)
	}
	if s := KeepAlive.String(); s != "KeepAlive" {
		t.Errorf("KeepAlive.String() = %q", s)
	}
	if s := OneShot.String(); s != "OneShot" {
		t.Errorf("OneShot.String() = %q", s)
	}
	if s := Upgrade.String(); s != "Upgrade" {
		t.Errorf("Upgrade.String() = %q", s)
	}
}

func TestUnknownStringers(t *testing.T) {
	if s := DispatchMode(99).String(); s != "Unknown" {
		t.Errorf("DispatchMode(99).String() = %q", s)
	}
	if s := EngineMode(99).String(); s != "Unknown" {
		t.Errorf("EngineMode(99).String() = %q", s)
	}
	if s := ResponseMode(99).String(); s != "Unknown" {
		t.Errorf("ResponseMode(99).String() = %q", s)
	}
	if s := BufferMode(99).String(); s != "Unknown" {
		t.Errorf("BufferMode(99).String() = %q", s)
	}
	if s := ConnMode(99).String(); s != "Unknown" {
		t.Errorf("ConnMode(99).String() = %q", s)
	}
}
