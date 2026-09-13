package kruda

func init() {
	bindgenGeneratedParse = bindGeneratedBinderInput
	bindgenGeneratedFactory = func(app *App, path string, handler func(*C[generatedBinderInput]) (*generatedBinderOutput, error)) HandlerFunc {
		return buildTypedHandlerWithBinder(app, "GET", path, handler, nil, bindGeneratedBinderInput)
	}
}
