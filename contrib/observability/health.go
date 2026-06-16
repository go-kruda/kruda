package observability

import (
	"time"

	"github.com/go-kruda/kruda"
)

// mountHealthRoutes registers /livez, /readyz, and the /health alias.
// /livez is cheap and dependency-free; /readyz and /health run the core
// HealthHandler (which discovers DI HealthCheckers and degrades to ok).
// The readiness timeout sits below the typical k8s probe timeout (1s) so a
// slow dependency fails the probe deterministically instead of flapping.
func mountHealthRoutes(app *kruda.App, r resolved) {
	if !r.healthOn {
		return
	}
	app.Get(r.livenessPath, func(c *kruda.Ctx) error {
		return c.Status(200).JSON(kruda.Map{"status": "ok"})
	})
	readiness := kruda.HealthHandler(kruda.WithHealthTimeout(900 * time.Millisecond))
	app.Get(r.readinessPath, readiness)
	if r.healthPath != "" && r.healthPath != r.readinessPath {
		app.Get(r.healthPath, readiness)
	}
}
