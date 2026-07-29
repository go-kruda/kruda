package kruda

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var maxSizeRuleRe = regexp.MustCompile(`max_size=([0-9]+(?:[kKmMgG][bB])?)`)

// TestDocumentedMaxSizeIsReachable guards a limit that reads as if it works and
// cannot. BodyLimit is enforced by the transport before the handler runs, so a
// request larger than it is answered 413 and never reaches validation. Every
// sample in the tree documented `max_size=5mb` against the 4MB default, which
// means the rule could not fire for any request that got far enough to be
// validated — measured end to end: a 4.5MB upload returns 413, not the 422 the
// docs describe.
//
// A sample is acceptable if its max_size fits under the default BodyLimit, or
// if the same file raises WithBodyLimit alongside it.
func TestDocumentedMaxSizeIsReachable(t *testing.T) {
	files := []string{
		"upload.go",
		"validation.go",
		"docs/guide/security.md",
		"README.md",
	}
	defaultLimit := int64(New().config.BodyLimit)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(b)
		raises := strings.Contains(body, "WithBodyLimit")
		for _, m := range maxSizeRuleRe.FindAllStringSubmatch(body, -1) {
			n, err := parseSize(m[1])
			if err != nil {
				t.Errorf("%s: max_size=%s is not a size parseSize accepts, so the rule always fails", f, m[1])
				continue
			}
			if n > defaultLimit && !raises {
				t.Errorf("%s documents max_size=%s (%d bytes) above the default BodyLimit (%d bytes) without raising WithBodyLimit — the rule can never fire; such a request is rejected 413 before validation",
					f, m[1], n, defaultLimit)
			}
		}
	}
}
