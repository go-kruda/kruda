package kruda

func init() {
	bindgenGeneratedParse = bindGeneratedBinderInputAttested()
	bindgenGeneratedFactory = func(app *App, path string, handler func(*C[generatedBinderInput]) (*generatedBinderOutput, error)) HandlerFunc {
		return buildTypedHandlerWithValidation(app, "GET", path, handler, nil, bindGeneratedBinderInputAttested(), nil)
	}
}
