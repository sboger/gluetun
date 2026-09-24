#!/usr/bin/env bash
# ci-gate-remote.sh — runs on the gate host (e.g. eco2) as root.
# Orchestrated by ci-gate.sh. Pulls a specific commit of the Gluetun fork over HTTPS,
# then runs the repo's own Docker build stages exactly as the GitHub Actions `verify`
# job does (lint / mocks / test / integration / xcompile / final).
#
# To be CI-faithful it intentionally runs the evergreen Docker targets rather than a
# locally-installed Go toolchain — every stage carries its own pinned toolchain and
# needs NET_ADMIN + /dev/net/tun for the Linux-kernel tests.
#
# Env:  GATE_DIR    absolute path of the (re)usable git work tree on the host
#       GATE_REMOTE HTTPS clone URL of the fork
# Args: <sha> [stage ...]
#       stage ∈ lint mocks test integration xcompile final  (default: all except final)

set -euo pipefail

: "${GATE_DIR:?set GATE_DIR}"
: "${GATE_REMOTE:?set GATE_REMOTE}"
sha="${1:?usage: ci-gate-remote.sh <sha> [stages...]}"
shift
stages="${*:-lint mocks test integration xcompile}"

mkdir -p "$GATE_DIR"
cd "$GATE_DIR"
export GIT_SSH_COMMAND="${GIT_SSH_COMMAND:-ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes}"
if [ ! -d .git ]; then
  echo ">> cloning $GATE_REMOTE"
  git clone --quiet "$GATE_REMOTE" .
fi

echo ">> syncing to $sha"
git fetch --all --tags --prune --quiet || git fetch origin --quiet
if ! git cat-file -e "$sha^{commit}" 2>/dev/null; then
  git fetch origin --quiet "$sha"
fi
git checkout --detach --quiet "$sha"
echo ">> now at $(git rev-parse --short HEAD) ($(git log -1 --format=%s))"

for stage in $stages; do
  echo "========== STAGE: $stage =========="
  case "$stage" in
    lint)        docker build --target lint . ;;
    mocks)       docker build --target mocks . ;;
    test)        docker build --target test -t gate-test .
                 touch coverage.txt
                 docker run --rm --cap-add=NET_ADMIN --device /dev/net/tun \
                   -v "$PWD/coverage.txt:/tmp/gobuild/coverage.txt" gate-test ;;
    integration) docker build --target test -t gate-test .
                 docker run --rm --entrypoint go gate-test test -tags=integration ./internal/restrictednet ;;
    xcompile)    docker build --target xcompile . ;;
    final)       docker build -t gate-final . ;;
    *) echo "unknown stage: $stage" >&2; exit 2 ;;
  esac
  echo "========== PASSED: $stage =========="
done
echo "ALL_GATES_PASSED:$stages"
