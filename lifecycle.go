package kruda

// HookFunc is a lifecycle hook function.
type HookFunc func(c *Ctx) error

// ErrorHookFunc is an error lifecycle hook function.
type ErrorHookFunc func(c *Ctx, err error)

// Hooks holds all lifecycle hook slices.
// H2 fix: removed BeforeHandle/AfterHandle — they were declared but never executed.
// They will be re-added in a future phase when per-route hooks are implemented.
type Hooks struct {
	OnRequest  []HookFunc
	OnResponse []HookFunc
	OnError    []ErrorHookFunc
	OnShutdown []func()
	OnParse    []func(c *Ctx, input any) error // fires after parse, before validate
}
