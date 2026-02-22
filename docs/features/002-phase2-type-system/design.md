# Phase 2 — Type System & Validation: Design

> Detailed design to be written when Phase 1 is complete.
> See spec Sections 7, 8 for full API signatures and implementation details.

## Key Design Points (from spec)

### Pre-compiled Input Parser
```go
type inputParser struct {
    paramFields []fieldParser   // fields with `param:` tag
    queryFields []fieldParser   // fields with `query:` tag
    hasBody     bool            // true if any field has `json:` tag
}
```
- Built once per type at `Post[In, Out]()` registration time
- String→type converters pre-selected per field kind
- Zero reflection at request time

### Validation Engine
- Pre-compiled: parse validate tags once → build validator chain
- Returns `ValidationError` with structured field errors
- Custom validators registered globally or per-app

### Structured Validation Errors (NEW)

```go
// ValidationError — implements error + json.Marshaler
type ValidationError struct {
    Errors []FieldError `json:"errors"`
}

type FieldError struct {
    Field   string `json:"field"`   // json tag name or struct field name
    Rule    string `json:"rule"`    // "required", "min", "email", etc.
    Param   string `json:"param"`   // "18" for min=18
    Message string `json:"message"` // "Age must be at least 18"
    Value   string `json:"value"`   // rejected value (stringified)
}

func (e *ValidationError) Error() string { ... }
func (e *ValidationError) MarshalJSON() ([]byte, error) { ... }
```

**Message generation pipeline:**
1. Check `message:"..."` struct tag → use if present (supports i18n)
2. Check `app.Validator().Messages()` overrides → use if rule match
3. Fall back to built-in English templates:
   ```go
   var defaultMessages = map[string]string{
       "required": "%s is required",
       "min":      "%s must be at least %s",
       "max":      "%s must be at most %s",
       "email":    "%s must be a valid email address",
   }
   ```

**Integration with error handler:**
```go
// In default ErrorHandler:
if ve, ok := err.(*ValidationError); ok {
    return c.Status(422).JSON(Map{
        "code":    422,
        "message": "Validation failed",
        "errors":  ve.Errors,
    })
}
```

### Input Parsing Pipeline (CRITICAL — order matters)

Mixed struct parsing (param + query + body) is complex. The pipeline must follow this exact order:

```
Step 1: Set defaults        → apply `default:"1"` tag values to struct fields
Step 2: Parse JSON body     → unmarshal body into struct (overwrites defaults for body fields)
Step 3: Parse query params  → set fields with `query:` tag from URL query string
Step 4: Parse path params   → set fields with `param:` tag from route params (highest priority)
Step 5: Validate            → run validators on merged struct (all sources combined)
```

**Why this order:**
- Defaults FIRST — so they're only used when no value is provided
- Body SECOND — most data comes from body
- Query THIRD — can override body for things like pagination
- Params LAST — path params are the most specific (e.g. `:id` is always from URL)
- Validate LAST — always after all sources are merged

**Example:**
```go
type UpdateUserReq struct {
    ID    string `param:"id"`                            // from /users/:id
    Page  int    `query:"page" default:"1"`              // from ?page=2
    Name  string `json:"name" validate:"required,min=2"` // from body
    Email string `json:"email" validate:"required,email"` // from body
}
```

### Sonic Integration
- Build tag gated: `sonic` for CGO, `std` for pure Go
- Same interface as encoding/json — drop-in
