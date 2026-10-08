#!/bin/sh
set -eu

# Explicit deployment secrets take precedence. Otherwise generate once and persist
# independently of application credentials and the source checkout.
if [ -z "${SEARXNG_SECRET:-}" ]; then
    secret_file=/var/lib/grokagent-searxng/secret
    umask 077
    mkdir -p "$(dirname "$secret_file")"
    if [ ! -s "$secret_file" ]; then
        secret=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
        [ "${#secret}" -eq 64 ] || { echo "Failed to generate SearXNG secret" >&2; exit 1; }
        temporary=$(mktemp "${secret_file}.XXXXXX")
        printf '%s\n' "$secret" > "$temporary"
        mv "$temporary" "$secret_file"
    fi
    SEARXNG_SECRET=$(cat "$secret_file")
    export SEARXNG_SECRET
fi

exec /usr/local/searxng/entrypoint.sh "$@"
