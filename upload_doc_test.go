package kruda

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	maxSizeRuleRe   = regexp.MustCompile(`max_size=([0-9]+(?:[kKmMgG][bB])?)`)
	withBodyLimitRe = regexp.MustCompile(`WithBodyLimit\(([^)]*)\)`)
	shiftExprRe     = regexp.MustCompile(`^(\d+)\s*<<\s*(\d+)$`)
	mulExprRe       = regexp.MustCompile(`^\d+(?:\s*\*\s*\d+)*$`)
)

// evalByteExpr evaluates the literal forms a WithBodyLimit sample realistically
// uses: 8<<20, 8 * 1024 * 1024, 4194304. Anything else — a named constant, a
// variable, an API table's WithBodyLimit(bytes) — returns an error, and the
// caller skips it rather than crediting the file with a limit it never states.
func evalByteExpr(expr string) (int64, error) {
	e := strings.TrimSpace(expr)
	if m := shiftExprRe.FindStringSubmatch(e); m != nil {
		base, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return 0, err
		}
		shift, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return 0, err
		}
		if shift > 62 {
			return 0, fmt.Errorf("shift %d out of range", shift)
		}
		return base << uint(shift), nil
	}
	if mulExprRe.MatchString(e) {
		total := int64(1)
		for _, part := range strings.Split(e, "*") {
			n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil {
				return 0, err
			}
			total *= n
		}
		return total, nil
	}
	return 0, fmt.Errorf("unrecognized byte expression %q", e)
}

// effectiveBodyLimit is the largest BodyLimit a file's samples establish, or the
// framework default when a file sets none. Taking the largest is deliberate: it
// is the most generous reading, so a max_size this test still rejects is
// unreachable under every configuration the file shows.
//
// Checking the value rather than the mere presence of WithBodyLimit matters: the
// first version of this guard passed any file that mentioned the option at all,
// so a sample raising the limit to 1MB while documenting max_size=5mb would have
// sailed through it.
//
// Mentions that are prose rather than samples — an API table's
// WithBodyLimit(bytes) — carry no value and are skipped. Granularity is the
// file, not the individual snippet: a file that raises the limit anywhere is
// credited for it everywhere, which is the limit of what a text scan can claim.
func effectiveBodyLimit(body string, def int64) int64 {
	limit := def
	for _, m := range withBodyLimitRe.FindAllStringSubmatch(body, -1) {
		n, err := evalByteExpr(m[1])
		if err != nil {
			continue
		}
		if n > limit {
			limit = n
		}
	}
	return limit
}

// shippedFilesMentioning returns the tracked documentation and source carrying a
// needle. Discovery rather than a hard-coded list: the first version of this
// guard named four files, and so never checked cmd/kruda/cmd_mcp.go — which
// carried the very sample that prompted writing it.
//
// Tracked files only. Untracked working-tree material (local design notes under
// .kiro, a stale draft) is not what anyone ships or copies, and failing on it
// would make the guard noise the next person silences.
func shippedFilesMentioning(t *testing.T, needle string) []string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git to tell shipped files from local ones")
	}
	out, err := exec.Command("git", "ls-files", "-z", "*.go", "*.md", "*.tmpl").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	var found []string
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		// The changelog records the defect verbatim; tests are not samples a
		// user copies, and this one states the defect in its own comments.
		if path == "CHANGELOG.md" || strings.HasSuffix(path, "_test.go") {
			continue
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		if strings.Contains(string(b), needle) {
			found = append(found, path)
		}
	}
	return found
}

// TestDocumentedMaxSizeIsReachable guards a limit that reads as if it works and
// cannot. BodyLimit is enforced by the transport before the handler runs, so a
// request larger than it is answered 413 and never reaches validation. Every
// upload sample in the tree documented max_size=5mb against the 4MB default,
// which means the rule could not fire for any request that got far enough to be
// validated — measured end to end, a 4.5MB upload returns 413, not the 422 the
// validation docs describe.
//
// A sample passes when its max_size fits under the BodyLimit its own file
// establishes, whether that is the framework default or one the file raises.
func TestDocumentedMaxSizeIsReachable(t *testing.T) {
	defaultLimit := int64(New().config.BodyLimit)

	files := shippedFilesMentioning(t, "max_size=")
	if len(files) == 0 {
		t.Fatal("found no files documenting max_size=; the guard is scanning the wrong tree")
	}
	t.Logf("checking %d file(s): %s", len(files), strings.Join(files, ", "))

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(b)

		limit := effectiveBodyLimit(body, defaultLimit)

		for _, m := range maxSizeRuleRe.FindAllStringSubmatch(body, -1) {
			n, err := parseSize(m[1])
			if err != nil {
				t.Errorf("%s: max_size=%s is not a size parseSize accepts, so the rule always fails", f, m[1])
				continue
			}
			if n > limit {
				t.Errorf("%s documents max_size=%s (%d bytes) above the BodyLimit its samples establish (%d bytes) — "+
					"the rule can never fire, because a request that large is rejected 413 before validation runs",
					f, m[1], n, limit)
			}
		}
	}
}
