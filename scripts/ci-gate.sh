#!/usr/bin/env bash
# ci-gate.sh — local driver that runs the GitHub-Actions `verify` gate for this repo
# on a remote Linux gate host (default: eco2) that has Docker + a usable tun device.
#
# It:
#   1. resolves the ref to a commit SHA,
#   2. pushes that branch to the fork (origin) so the gate host can fetch it,
#   3. ships ci-gate-remote.sh to the host and runs it with GATE_DIR / GATE_REMOTE,
#      which pulls the SHA and runs the repo's Docker stages (lint/mocks/test/integration/xcompile).
#
# Usage:  scripts/ci-gate.sh [REF] [STAGE ...]
#   REF    git ref to gate (default: current branch HEAD)
#   STAGE  lint mocks test integration xcompile final  (default: lint mocks test integration xcompile)
#
# Env overrides:
#   GATE_HOST     ssh alias/host (default: eco2)
#   GATE_DIR      work tree path on the host (default: /root/gluetun-gate)
#   GATE_REMOTE   clone URL used by the host (default: this repo's origin remote, i.e. your ssh fork URL)
#
# CI-faithful: the Green gate runs the repo's evergreen Docker targets, each with a
# pinned toolchain, exactly like .github/workflows/ci.yml `verify`. No Go needed on the host.

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

GATE_HOST="${GATE_HOST:-eco2}"
GATE_DIR="${GATE_DIR:-/root/gluetun-gate}"
GATE_REMOTE="${GATE_REMOTE:-$(git remote get-url origin)}"

REF="${1:-$(git branch --show-current)}"
shift || true
STAGES="$*"
[ -z "$STAGES" ] && STAGES="lint mocks test integration xcompile"

sha="$(git rev-parse --verify "${REF}^{commit}")"

# Ensure the ref's branch is on the fork so the gate host can fetch the exact commit.
if git show-ref --verify --quiet "refs/heads/$REF" 2>/dev/null; then
  echo ">> pushing '$REF' to origin"
  git push origin "$REF"
fi

echo ">> gating $sha on $GATE_HOST ${GATE_DIR}"
scp -q scripts/ci-gate-remote.sh "$GATE_HOST:/tmp/ci-gate-remote.sh"
ssh "$GATE_HOST" "GATE_DIR='$GATE_DIR' GATE_REMOTE='$GATE_REMOTE' bash /tmp/ci-gate-remote.sh '$sha' $STAGES"
