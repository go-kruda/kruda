package kruda

const compositionTypedValidationExpected = true

func compositionHandler5(app *App, handler func(*C[composition5Input]) (*composition5Input, error)) HandlerFunc {
	return buildTypedHandlerWithValidation(app, "GET", "/users/:id", handler, nil, bindComposition5InputAttested(), makeBindComposition5InputValidator)
}

func compositionValidationEligible5(v *Validator) bool {
	if v == nil {
		return false
	}
	return makeBindComposition5InputValidator(buildValidators[composition5Input](v)) != nil
}

func compositionHandler10(app *App, handler func(*C[composition10Input]) (*composition10Input, error)) HandlerFunc {
	return buildTypedHandlerWithValidation(app, "GET", "/users/:id", handler, nil, bindComposition10InputAttested(), makeBindComposition10InputValidator)
}

func compositionValidationEligible10(v *Validator) bool {
	if v == nil {
		return false
	}
	return makeBindComposition10InputValidator(buildValidators[composition10Input](v)) != nil
}

func compositionHandler30(app *App, handler func(*C[composition30Input]) (*composition30Input, error)) HandlerFunc {
	return buildTypedHandlerWithValidation(app, "GET", "/users/:id", handler, nil, bindComposition30InputAttested(), makeBindComposition30InputValidator)
}

func compositionValidationEligible30(v *Validator) bool {
	if v == nil {
		return false
	}
	return makeBindComposition30InputValidator(buildValidators[composition30Input](v)) != nil
}
