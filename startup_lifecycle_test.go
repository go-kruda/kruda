package kruda

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

var errStartupTransportDone = errors.New("startup transport done")

type startupProbeTransport struct {
	t           *testing.T
	serveCalled bool
	onServe     func(handler transport.Handler) error
}

func (tr *startupProbeTransport) ListenAndServe(_ string, _ transport.Handler) error {
	return errors.New("ListenAndServe should not be called by App.Listen")
}

func (tr *startupProbeTransport) Serve(ln net.Listener, handler transport.Handler) error {
	tr.serveCalled = true
	if tr.onServe != nil {
		if err := tr.onServe(handler); err != nil {
			_ = ln.Close()
			return err
		}
	}
	_ = ln.Close()
	return errStartupTransportDone
}

func (tr *startupProbeTransport) Shutdown(context.Context) error {
	return nil
}

func TestListenUsesFullCompileState(t *testing.T) {
	tr := &startupProbeTransport{
		t: t,
		onServe: func(handler transport.Handler) error {
			app, ok := handler.(*App)
			if !ok {
				return errors.New("handler is not *App")
			}
			if !app.hasLifecycle {
				return errors.New("Listen should compute full compile lifecycle state before serving")
			}
			return nil
		},
	}
	app := New(WithTransport(tr))
	app.OnRequest(func(c *Ctx) error { return nil })
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })

	err := app.Listen("127.0.0.1:0")
	if !errors.Is(err, errStartupTransportDone) {
		t.Fatalf("Listen error = %v, want %v", err, errStartupTransportDone)
	}
	if !tr.serveCalled {
		t.Fatal("transport Serve was not called")
	}
}

type startupLifecycleService struct {
	initCalled bool
}

func (s *startupLifecycleService) OnInit(context.Context) error {
	s.initCalled = true
	return nil
}

func TestListenStartsContainerBeforeServing(t *testing.T) {
	svc := &startupLifecycleService{}
	container := NewContainer()
	if err := container.Give(svc); err != nil {
		t.Fatal(err)
	}

	tr := &startupProbeTransport{
		t: t,
		onServe: func(transport.Handler) error {
			if !svc.initCalled {
				return errors.New("container OnInit should run before transport Serve")
			}
			return nil
		},
	}
	app := New(WithContainer(container), WithTransport(tr))
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })

	err := app.Listen("127.0.0.1:0")
	if !errors.Is(err, errStartupTransportDone) {
		t.Fatalf("Listen error = %v, want %v", err, errStartupTransportDone)
	}
}

type failingStartupLifecycleService struct{}

func (s *failingStartupLifecycleService) OnInit(context.Context) error {
	return errors.New("init failed")
}

func TestListenReturnsContainerStartErrorBeforeServing(t *testing.T) {
	container := NewContainer()
	if err := container.Give(&failingStartupLifecycleService{}); err != nil {
		t.Fatal(err)
	}

	tr := &startupProbeTransport{t: t}
	app := New(WithContainer(container), WithTransport(tr))
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })

	err := app.Listen("127.0.0.1:0")
	if err == nil || !strings.Contains(err.Error(), "init failed") {
		t.Fatalf("Listen should return container startup error, got %v", err)
	}
	if tr.serveCalled {
		t.Fatal("transport Serve should not be called when container startup fails")
	}
}
