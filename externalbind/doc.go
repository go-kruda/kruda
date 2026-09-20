// Package externalbind proves that bindgen output works outside package
// kruda: the committed generated file references only the public API, the
// attested factory engages cross-package, and stale shapes decline.
//
// Binder-only inputs (no validate tags) are supported. Inputs with
// validatable rules are rejected for external packages until the exported
// validator descriptor lands; see the generator note.
package externalbind
