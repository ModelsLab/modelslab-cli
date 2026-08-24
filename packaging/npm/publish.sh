#!/usr/bin/env bash
#
# Publishes the built npm packages, safe to re-run.
#
# Two things the naive `for pkg in *; do npm publish; done` gets wrong, both of
# which bit the v0.1.2 release:
#
#   1. npm's spam heuristic. Publishing six similarly-named packages back to back
#      tripped it on the sixth with a 403 "Package name triggered spam
#      detection". It is rate-shaped, not permanent, so a paced retry clears it.
#
#   2. Re-running after a partial failure. npm refuses to republish a version
#      that already exists, so a retry of a half-finished release fails on the
#      packages that DID succeed and never reaches the ones that did not.
#
# Ordering still matters: platform packages before the entry package, which pins
# exact versions of all of them. Publishing the entry first leaves a window where
# every install fails.
set -euo pipefail

DIST="${1:?usage: publish.sh <dist/npm dir>}"
ENTRY="modelslab-cli"
MAX_ATTEMPTS=5

already_published() {
    local name="$1" version="$2"
    npm view "${name}@${version}" version >/dev/null 2>&1
}

publish_one() {
    local dir="$1"
    local name version attempt delay output
    name=$(node -p "require('${dir}/package.json').name")
    version=$(node -p "require('${dir}/package.json').version")

    if already_published "$name" "$version"; then
        echo "skip    ${name}@${version} (already on the registry)"
        return 0
    fi

    for attempt in $(seq 1 "$MAX_ATTEMPTS"); do
        if output=$(npm publish "$dir" --access public --provenance 2>&1); then
            echo "$output"
            echo "publish ${name}@${version}"
            return 0
        fi

        echo "$output"

        # A version that appeared between the check and the publish is a success
        # for our purposes — most likely a concurrent or retried run.
        if grep -qi "cannot publish over\|EPUBLISHCONFLICT" <<<"$output"; then
            echo "skip    ${name}@${version} (published concurrently)"
            return 0
        fi

        if ! grep -qi "spam detection\|429\|rate.limit" <<<"$output"; then
            echo "fatal   ${name}@${version}: not a retryable error" >&2
            return 1
        fi

        delay=$((attempt * 30))
        echo "retry   ${name}@${version} in ${delay}s (attempt ${attempt}/${MAX_ATTEMPTS}, spam/rate heuristic)" >&2
        sleep "$delay"
    done

    echo "fatal   ${name}@${version}: still refused after ${MAX_ATTEMPTS} attempts" >&2
    return 1
}

for dir in "${DIST}"/${ENTRY}-*; do
    [ -d "$dir" ] || continue
    publish_one "$dir"
    # Pace the platform packages. Publishing them as fast as the API allows is
    # what looks like spam in the first place.
    sleep 10
done

publish_one "${DIST}/${ENTRY}"

echo "npm publish complete"
