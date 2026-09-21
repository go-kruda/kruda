// Package externalbind proves that bindgen output works outside package
// kruda: the committed generated file references only the public API, the
// attested plan engages cross-package through WithGeneratedPlan, and stale
// shapes decline.
//
// Validated inputs are supported. The generated validator factory attests
// against exported ValidatorDescriptors; inputs whose rules the generator
// cannot compile (custom overrides, unsupported rules, dive, omitempty)
// decline the factory and the route keeps generic validation.
package externalbind
