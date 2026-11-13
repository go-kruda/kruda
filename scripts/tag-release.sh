#!/usr/bin/env bash
# Tag a Kruda release the only way release.yml accepts it.
#
#   scripts/tag-release.sh vX.Y.Z [--set contrib/ws=vA.B.C]... [--push]
#
# Without --push it checks everything and prints the tags it would create.
# Nested modules that changed since their last tag get a patch bump unless
# --set gives their version; unchanged modules are not retagged.
set -euo pipefail

ok() { printf "\033[32m ✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m ✗\033[0m %s\n" "$*" >&2; exit 1; }
section() { printf "\n\033[1m== %s ==\033[0m\n" "$*"; }

SEMVER='^v[0-9]+\.[0-9]+\.[0-9]+$'
CORE=""
PUSH=0
SETS=()
while (($#)); do
  case $1 in
    --push) PUSH=1 ;;
    --set) [[ $# -ge 2 ]] || fail "--set needs <dir>=<version>"; SETS+=("$2"); shift ;;
    -*) fail "unknown flag $1" ;;
    *) [[ -z $CORE ]] || fail "one core version only"; CORE=$1 ;;
  esac
  shift
done
[[ $CORE =~ $SEMVER ]] || fail "usage: scripts/tag-release.sh vX.Y.Z [--set <dir>=vA.B.C]... [--push]"

cd "$(git rev-parse --show-toplevel)"

latest_tag() { git tag -l "${1}v*" | sed "s#^$1##" | grep -E "$SEMVER" | sort -V | tail -1; }
bump_patch() { local v=${1#v}; echo "v${v%.*}.$((${v##*.} + 1))"; }
newer() { [[ $1 != "$2" && $(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1) == "$1" ]]; }
set_for() {
  local s
  for s in ${SETS[@]+"${SETS[@]}"}; do [[ ${s%%=*} == "$1" ]] && { echo "${s#*=}"; return; }; done
  return 0
}
green_runs() {
  gh run list --workflow "$1" --branch main --commit "$SHA" --status success \
    --limit 1 --json databaseId --jq length
}
# A dead connection must fail instead of hanging: ssh keepalive, and abort a stalled https transfer.
export GIT_SSH_COMMAND=${GIT_SSH_COMMAND:-ssh -o ConnectTimeout=15 -o ServerAliveInterval=10 -o ServerAliveCountMax=3}
git_net() { git -c http.lowSpeedLimit=1000 -c http.lowSpeedTime=30 "$@"; }

section "commit"
git_net fetch --quiet origin main --tags
SHA=$(git rev-parse origin/main)
ok "origin/main is $SHA"

section "core tag $CORE"
LAST_CORE=$(latest_tag "")
newer "$CORE" "$LAST_CORE" || fail "$CORE is not newer than $LAST_CORE"
[[ -n $(git diff --name-only "$LAST_CORE" "$SHA") ]] || fail "nothing changed since $LAST_CORE"
# A here-string, not a pipe: grep -q stops at the first match and pipefail would report git show's SIGPIPE.
grep -q "^## \[${CORE#v}\]" <<<"$(git show "$SHA:CHANGELOG.md")" || fail "CHANGELOG.md has no '## [${CORE#v}]' section"
ok "$CORE follows $LAST_CORE and has a CHANGELOG section"

section "main is green for $SHA"
# release.yml rejects a tag without these, and the tag cannot be taken back.
for wf in test.yml benchmark.yml; do
  n=$(green_runs "$wf") || fail "gh run list failed for $wf; check the network and gh auth, then rerun"
  ((n >= 1)) || fail "no successful $wf run on main for $SHA yet; wait, then rerun"
  ok "$wf succeeded"
done

section "nested modules"
TAGS=()
MSGS=()
for dir in $(git ls-tree -r --name-only "$SHA" | grep -E '^(cmd/kruda|contrib/[^/]+)/go.mod$' | xargs -n1 dirname); do
  last=$(latest_tag "$dir/")
  want=$(set_for "$dir")
  if [[ -n $last ]] && git diff --quiet "$dir/$last" "$SHA" -- "$dir"; then
    [[ -z $want ]] || fail "--set $dir: unchanged since $dir/$last"
    echo "  $dir unchanged since $last"
    continue
  fi
  [[ -n $want || -n $last ]] || fail "$dir has no tag yet; pass --set $dir=vA.B.C"
  next=${want:-$(bump_patch "$last")}
  [[ $next =~ $SEMVER ]] || fail "$dir: bad version $next"
  [[ -z $last ]] || newer "$next" "$last" || fail "$dir: $next is not newer than $last"
  TAGS+=("$dir/$next")
  MSGS+=("$dir $next")
done
for s in ${SETS[@]+"${SETS[@]}"}; do
  [[ ${s%%=*} =~ ^(cmd/kruda|contrib/[^/]+)$ ]] && git cat-file -e "$SHA:${s%%=*}/go.mod" 2>/dev/null ||
    fail "--set ${s%%=*}: not a nested module"
done
TAGS+=("$CORE")
MSGS+=("Release $CORE")

remote=$(git_net ls-remote --tags origin | awk '{print $2}')
for t in "${TAGS[@]}"; do
  git rev-parse -q --verify "refs/tags/$t" >/dev/null && fail "tag $t already exists locally"
  grep -qx "refs/tags/$t" <<<"$remote" && fail "tag $t already exists on origin"
  echo "  $t"
done
ok "${#TAGS[@]} new tags on $SHA"

section "release security scan"
# The advisory database changes between PR CI and tagging; v1.8.3 was tagged
# hours after a new advisory and release.yml stopped at this scan.
GO_SCAN=$(awk '/^  security:/{s=1} s && /go-version:/{gsub(/[^0-9.]/,"",$2); print $2; exit}' <<<"$(git show "$SHA:.github/workflows/release.yml")")
[[ -n $GO_SCAN ]] || fail "cannot read the security job's go-version from release.yml"
SCAN_DIR=$(mktemp -d)
trap 'rm -rf "$SCAN_DIR"' EXIT
git archive "$SHA" | tar -x -C "$SCAN_DIR"
if ! (cd "$SCAN_DIR" && GOTOOLCHAIN=go$GO_SCAN scripts/govulncheck-modules.sh --local-core) >"$SCAN_DIR.log" 2>&1; then
  tail -20 "$SCAN_DIR.log" >&2
  fail "release security scan failed with go$GO_SCAN (full log: $SCAN_DIR.log); release.yml would stop here"
fi
rm -f "$SCAN_DIR.log"
ok "govulncheck-modules.sh --local-core is clean with go$GO_SCAN"

if ((PUSH == 0)); then
  echo
  echo "Dry run. Rerun with --push to create and push these tags."
  exit 0
fi

section "push"
for i in "${!TAGS[@]}"; do git tag -a "${TAGS[$i]}" -m "${MSGS[$i]}" "$SHA"; done
if ((${#TAGS[@]} > 1)); then
  NESTED=("${TAGS[@]:0:${#TAGS[@]}-1}")
  git_net push --atomic origin "${NESTED[@]/#/refs/tags/}"
  ok "pushed ${#NESTED[@]} nested module tags"
fi
# GitHub sends no push event when more than three tags arrive in one push,
# so the core tag goes alone or release.yml never runs.
git_net push origin "refs/tags/$CORE"
ok "pushed $CORE on its own"

section "release.yml"
RUN=""
for _ in $(seq 1 "${TAG_RELEASE_WAIT:-30}"); do
  RUN=$(gh run list --workflow release.yml --event push --branch "$CORE" --limit 1 --json databaseId --jq '.[0].databaseId // empty')
  [[ -n $RUN ]] && break
  sleep "${TAG_RELEASE_POLL:-10}"
done
[[ -n $RUN ]] || fail "release.yml did not start for $CORE; find out why. Do not create the GitHub release by hand"
gh run watch "$RUN" --exit-status >/dev/null || fail "release.yml run $RUN failed; do not create the GitHub release by hand"
ok "release.yml run $RUN succeeded"
author=$(gh release view "$CORE" --json author --jq .author.login)
[[ $author == "github-actions[bot]" ]] || fail "release $CORE was created by $author, not release.yml"
ok "GitHub release created by $author"

section "Go proxy"
for t in "${TAGS[@]}"; do
  dir=${t%/*}; ver=${t##*/}
  mod=github.com/go-kruda/kruda
  [[ $dir == "$t" ]] || mod=$mod/$dir
  hash=""
  for _ in 1 2 3 4 5 6; do
    hash=$(curl -fsS "https://proxy.golang.org/$mod/@v/$ver.info" 2>/dev/null | sed -n 's/.*"Hash":"\([0-9a-f]*\)".*/\1/p') || true
    [[ -n $hash ]] && break
    sleep "${TAG_RELEASE_POLL:-10}"
  done
  [[ $hash == "$SHA" ]] || fail "proxy serves $mod@$ver at '${hash:-nothing}', want $SHA"
  echo "  $mod@$ver"
done
ok "proxy serves every tag at $SHA"
