#!/usr/bin/env bash

if [ -n "$1" ]; then
    echo "PREVIOUS_VERSION=$1"
    exit 0
fi

set -e

prev=$(gh release list --exclude-drafts --exclude-pre-releases --limit 2 --json tagName --jq '.[1].tagName')
printf "PREVIOUS_VERSION=%s\n" "${prev}"
