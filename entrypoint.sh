#!/bin/sh
set -e

GITHUB_TOKEN="${INPUT_GITHUB_TOKEN:-$GITHUB_TOKEN}"
GITVERSE_TOKEN="${INPUT_GITVERSE_TOKEN}"
SOURCE_OWNER="${INPUT_SOURCE_OWNER:-}"
GITVERSE_OWNER="${INPUT_GITVERSE_OWNER:-}"
REPOS="${INPUT_REPOS}"
TIMEOUT="${INPUT_TIMEOUT_MINUTES:-30}"

if [ -z "${GITVERSE_TOKEN}" ]; then
  echo "ERROR: gitverse_token is required"
  exit 1
fi

if [ -z "${GITHUB_TOKEN}" ]; then
  echo "ERROR: github_token is required (set input or GITHUB_TOKEN env var)"
  exit 1
fi

CONFIG_PATH="${CONFIG_PATH:-/tmp/gh-mirror-config.yaml}"
export CONFIG_PATH

# Generate config.yaml
{
  echo "platforms:"
  echo "  github:"
  echo "    token: \"${GITHUB_TOKEN}\""
  echo "    url: \"https://github.com\""
  if [ -n "${SOURCE_OWNER}" ]; then
    echo "    owner: \"${SOURCE_OWNER}\""
  fi
  echo "  gitverse:"
  echo "    token: \"${GITVERSE_TOKEN}\""
  echo "    api_url: \"https://api.gitverse.ru\""
  echo "    url: \"https://gitverse.ru\""
  if [ -n "${GITVERSE_OWNER}" ]; then
    echo "    owner: \"${GITVERSE_OWNER}\""
  fi
  echo ""
  echo "source: github"
  echo "destinations:"
  echo "  - gitverse"
  echo ""
  echo "sync:"
  echo "  timeout_minutes: ${TIMEOUT}"
} > "${CONFIG_PATH}"

echo "Config written to ${CONFIG_PATH}"

if [ -n "${REPOS}" ]; then
  echo "Syncing repositories: ${REPOS}"
  for repo in $(echo "${REPOS}" | tr ',' ' '); do
    echo "--- Syncing ${repo} ---"
    ./mirror sync "${repo}"
  done
else
  echo "Syncing all repositories..."
  ./mirror sync
fi

echo "Done."
