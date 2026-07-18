package kruda

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

var errShutdownLifecycleTransport = errors.New("shutdown lifecycle transport error")

type shutdownLifecycleTransport struct {
	service        *shutdownLifecycleService
	events         *[]string
	sawServiceLive bool
}

func (tr *shutdownLifecycleTransport) ListenAndServe(string, transport.Handler) error {
	return nil
}

func (tr *shutdownLifecycleTransport) Serve(net.Listener, transport.Handler) error {
	return nil
}

func (tr *shutdownLifecycleTransport) Shutdown(context.Context) error {
	*tr.events = append(*tr.events, "transport")
	tr.sawServiceLive = tr.service.live
	return errShutdownLifecycleTransport
}

type shutdownLifecycleService struct {
	live   bool
	events *[]string
}

func (s *shutdownLifecycleService) OnShutdown(context.Context) error {
	*s.events = append(*s.events, "container")
	s.live = false
	return nil
}

func TestPrivateShutdownLifecycleOrder(t *testing.T) {
	assertShutdownLifecycleOrder(t, (*App).shutdown)
}

func TestPublicShutdownLifecycleOrder(t *testing.T) {
	assertShutdownLifecycleOrder(t, func(app *App) error {
		return app.Shutdown(context.Background())
	})
}

func assertShutdownLifecycleOrder(t *testing.T, shutdown func(*App) error) {
	t.Helper()

	events := make([]string, 0, 3)
	service := &shutdownLifecycleService{live: true, events: &events}
	container := NewContainer()
	if err := container.Give(service); err != nil {
		t.Fatalf("register lifecycle service: %v", err)
	}

	tr := &shutdownLifecycleTransport{service: service, events: &events}
	app := New(WithContainer(container), WithTransport(tr))
	hookSawServiceStopped := false
	app.OnShutdown(func() {
		events = append(events, "hook")
		hookSawServiceStopped = !service.live
	})

	err := shutdown(app)
	if !errors.Is(err, errShutdownLifecycleTransport) {
		t.Fatalf("shutdown error = %v, want %v", err, errShutdownLifecycleTransport)
	}
	if !tr.sawServiceLive {
		t.Error("transport Shutdown should run while the lifecycle service is still live")
	}
	if service.live {
		t.Error("container lifecycle service should be stopped despite the transport error")
	}
	if !hookSawServiceStopped {
		t.Error("OnShutdown hook should run after the container lifecycle service stops")
	}
	if want := []string{"transport", "container", "hook"}; !slices.Equal(events, want) {
		t.Errorf("shutdown order = %v, want %v", events, want)
	}
}
