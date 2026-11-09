#!/usr/bin/env bash
# Exercises scripts/tag-release.sh against a throwaway repository, with gh and
# curl stubbed, so its release guards are checked without touching GitHub.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/tag-release.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

pass=0
check() { printf "  ok %s\n" "$1"; pass=$((pass + 1)); }
die() { printf "FAIL %s\n" "$*" >&2; exit 1; }

mkdir -p "$TMP/bin"
cat >"$TMP/bin/gh" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "run list")
    wf=""
    while (($#)); do [[ $1 == --workflow ]] && wf=$2; shift; done
    case $wf in
      test.yml | benchmark.yml) echo "${FAKE_GREEN:-1}" ;;
      release.yml) echo "${FAKE_RUN_ID:-}" ;;
    esac ;;
  "run watch") exit "${FAKE_WATCH_EXIT:-0}" ;;
  "release view") echo "${FAKE_AUTHOR:-github-actions[bot]}" ;;
  *) echo "unexpected: gh $*" >&2; exit 2 ;;
esac
EOF
cat >"$TMP/bin/curl" <<'EOF'
#!/usr/bin/env bash
echo "{\"Version\":\"x\",\"Origin\":{\"VCS\":\"git\",\"Hash\":\"$(git -C "$FAKE_ORIGIN" rev-parse main)\"}}"
EOF
chmod +x "$TMP/bin/gh" "$TMP/bin/curl"
export PATH="$TMP/bin:$PATH" TAG_RELEASE_POLL=0 TAG_RELEASE_WAIT=2
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com

export FAKE_ORIGIN="$TMP/origin.git"
git init -q --bare -b main "$FAKE_ORIGIN"
# Log how many refs each push carries.
printf '#!/usr/bin/env bash\nwc -l | tr -d " " >>"%s/pushes"\n' "$TMP" >"$FAKE_ORIGIN/hooks/pre-receive"
chmod +x "$FAKE_ORIGIN/hooks/pre-receive"

git clone -q "$FAKE_ORIGIN" "$TMP/work" 2>/dev/null
cd "$TMP/work"
for d in . cmd/kruda contrib/a contrib/b; do mkdir -p "$d" && echo "module m/$d" >"$d/go.mod"; done
printf '# Changelog\n\n## [1.0.0]\n' >CHANGELOG.md
git add -A && git commit -qm one
for t in v1.0.0 cmd/kruda/v1.0.0 contrib/a/v1.2.0 contrib/b/v1.0.0; do git tag -a "$t" -m "$t"; done
git push -q origin main --tags

echo "// a" >>contrib/a/go.mod
printf '# Changelog\n\n## [1.0.1]\n\n## [1.0.0]\n' >CHANGELOG.md
git commit -qam two && git push -q origin main
: >"$TMP/pushes"

run() { "$SCRIPT" "$@" >"$TMP/out" 2>&1; }
expect_fail() {
  local want=$1; shift
  if run "$@"; then die "expected failure for: $*"; fi
  grep -q -- "$want" "$TMP/out" || { cat "$TMP/out"; die "missing '$want' for: $*"; }
}

FAKE_GREEN=0 expect_fail "no successful test.yml run" v1.0.1
check "refuses to tag before main is green"

expect_fail "not newer than v1.0.0" v1.0.0
check "refuses a core version that is not newer"

expect_fail "no '## \[1.0.2\]' section" v1.0.2
check "refuses a version without a CHANGELOG section"

expect_fail "unchanged since contrib/b/v1.0.0" v1.0.1 --set contrib/b=v1.1.0
check "refuses --set on an unchanged module"

run v1.0.1 || { cat "$TMP/out"; die "dry run failed"; }
grep -q "contrib/a/v1.2.1" "$TMP/out" && grep -q "^  v1.0.1$" "$TMP/out" || die "dry run plan wrong"
! grep -qE "contrib/b/v|cmd/kruda/v1.0.1" "$TMP/out" || die "dry run retags unchanged modules"
[[ -z $(git tag -l v1.0.1) && ! -s $TMP/pushes ]] || die "dry run created or pushed tags"
check "dry run bumps only changed modules and touches nothing"

run v1.0.1 --set contrib/a=v1.3.0 || die "--set dry run failed"
grep -q "contrib/a/v1.3.0" "$TMP/out" || die "--set ignored"
check "--set overrides the patch bump"

FAKE_RUN_ID="" expect_fail "release.yml did not start" v1.0.1 --push
check "fails loudly when release.yml does not start"
[[ $(cat "$TMP/pushes") == $'1\n1' ]] || die "pushes were: $(tr '\n' ' ' <"$TMP/pushes")"
check "pushes nested tags, then the core tag alone"

echo "// a2" >>contrib/a/go.mod
printf '# Changelog\n\n## [1.0.2]\n' >CHANGELOG.md
git commit -qam three && git push -q origin main

FAKE_RUN_ID=7 FAKE_AUTHOR=someone expect_fail "created by someone, not release.yml" v1.0.2 --push
check "rejects a GitHub release not made by release.yml"

echo "// a3" >>contrib/a/go.mod
printf '# Changelog\n\n## [1.0.3]\n' >CHANGELOG.md
git commit -qam four && git push -q origin main
FAKE_RUN_ID=8 run v1.0.3 --push || { cat "$TMP/out"; die "push run failed"; }
grep -q "proxy serves every tag" "$TMP/out" || die "push run did not verify the proxy"
[[ $(git -C "$FAKE_ORIGIN" rev-parse 'v1.0.3^{commit}') == $(git -C "$FAKE_ORIGIN" rev-parse main) ]] || die "core tag not on main"
check "full push run tags main and verifies release and proxy"

echo "tag-release: $pass checks passed"
