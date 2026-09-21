package kruda

func init() {
	bindgenGeneratedParse = bindGeneratedBinderInputAttested()
	bindgenGeneratedFactory = func(app *App, path string, handler func(*C[generatedBinderInput]) (*generatedBinderOutput, error)) HandlerFunc {
		return buildTypedHandlerWithBinder(app, "GET", path, handler, nil, bindGeneratedBinderInputAttested(), nil)
	}
}
