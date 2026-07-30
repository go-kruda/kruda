package json

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// sonicConstraintPin is the sonic release whose build constraint sonic_accel.go
// mirrors. Bump it only after re-reading that constraint.
const sonicConstraintPin = "v1.15.0"

// TestSonicConstraintMirrorIsCurrent fails when the sonic dependency moves.
//
// sonic_accel.go copies sonic's own build constraint by hand, because sonic
// exposes no way to ask whether its accelerated implementation compiled. A sonic
// release that adds support for a newer Go version makes that copy wrong in the
// direction that silently costs speed: Kruda would report and select
// encoding/json's path while sonic was in fact accelerating. Pinning the version
// forces a human to re-check the constraint on every bump.
func TestSonicConstraintMirrorIsCurrent(t *testing.T) {
	b, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	m := regexp.MustCompile(`github\.com/bytedance/sonic (v[0-9][^\s]*)`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("sonic not found in go.mod")
	}
	if m[1] != sonicConstraintPin {
		t.Fatalf("sonic moved to %s but sonic_accel.go still mirrors the constraint from %s.\n"+
			"Re-read that release's sonic.go build tag, update sonic_accel.go and sonic_noaccel.go to match, "+
			"then bump sonicConstraintPin.", m[1], sonicConstraintPin)
	}
}

// TestActiveEngineMatchesThisBuild pins the relationship the response path now
// depends on: ActiveEngine and EngineIsStdlib must agree with each other and with
// what this build can actually do.
func TestActiveEngineMatchesThisBuild(t *testing.T) {
	if got := ActiveEngine(); got != "sonic" && got != "encoding/json" {
		t.Fatalf("ActiveEngine() = %q, want sonic or encoding/json", got)
	}
	if (ActiveEngine() == "encoding/json") != EngineIsStdlib {
		t.Errorf("ActiveEngine()=%q disagrees with EngineIsStdlib=%v", ActiveEngine(), EngineIsStdlib)
	}

	// On an architecture sonic has no assembly for, the default build must report
	// the fallback rather than the tag.
	accelArch := runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
	if EncoderName == "sonic" && !accelArch && !EngineIsStdlib {
		t.Errorf("GOARCH=%s cannot be accelerated by sonic, but EngineIsStdlib is false", runtime.GOARCH)
	}
}

// TestEncoderNameIsDocumentedAsTheTag guards the distinction that caused the bug:
// EncoderName names what the tag selected, ActiveEngine names what runs. Anything
// choosing behaviour must use the latter.
func TestEncoderNameIsDocumentedAsTheTag(t *testing.T) {
	b, err := os.ReadFile("sonic.go")
	if err != nil {
		t.Skip("sonic.go not in this build")
	}
	if !strings.Contains(string(b), "not necessarily") {
		t.Error("EncoderName's doc no longer warns that it is not necessarily the engine doing the work")
	}
}
