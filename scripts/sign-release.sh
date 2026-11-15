#!/usr/bin/env bash
#
# Signs and publishes a draft release made by .github/workflows/release.yml.
#
#   scripts/sign-release.sh v0.1.0
#
# Downloads every asset of the draft and checks it against checksums.txt, so
# nothing gets signed that hasn't been checked, then signs checksums.txt with
# minisign, uploads the signature, and publishes the release once confirmed.
#
# The minisign secret key is read from $MINISIGN_KEY, or minisign's default
# ~/.minisign/minisign.key.
set -euo pipefail

usage() {
	echo "usage: $0 <tag>" >&2
	exit 2
}

[[ $# -eq 1 ]] || usage
tag=$1
key=${MINISIGN_KEY:-$HOME/.minisign/minisign.key}

missing=()
for cmd in gh minisign shasum; do
	command -v "$cmd" >/dev/null || missing+=("$cmd")
done
if [[ ${#missing[@]} -gt 0 ]]; then
	echo "missing: ${missing[*]}" >&2
	echo "install them with: brew install gh minisign" >&2
	exit 1
fi
if [[ ! -f $key ]]; then
	echo "no minisign secret key at $key (set MINISIGN_KEY)" >&2
	exit 1
fi

# gh infers the repo from the git checkout it's run in, and the downloads
# below run in a temp dir outside it, so resolve the repo once from the
# script's own checkout and name it in every gh call.
repo=$(cd "$(dirname "$0")/.." && gh repo view --json nameWithOwner --jq .nameWithOwner)

is_draft=$(gh release view "$tag" -R "$repo" --json isDraft --jq .isDraft)
if [[ $is_draft != true ]]; then
	echo "release $tag isn't a draft; refusing to re-sign a published release" >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

echo "Downloading the assets of $tag..."
gh release download "$tag" -R "$repo" --pattern checksums.txt
gh release download "$tag" -R "$repo" --skip-existing

# every asset must be listed, not just those that are listed must match
for f in *; do
	case $f in
	checksums.txt | checksums.txt.minisig) continue ;;
	esac
	if ! awk -v f="$f" '$2 == f || $2 == "*" f { found = 1 } END { exit !found }' checksums.txt; then
		echo "$f isn't in checksums.txt" >&2
		exit 1
	fi
done

echo "Checking checksums..."
shasum -a 256 -c checksums.txt

echo "Signing checksums.txt..."
minisign -S -s "$key" -m checksums.txt -t "ecLogger $tag checksums.txt"

gh release upload "$tag" -R "$repo" checksums.txt.minisig --clobber
echo "Uploaded checksums.txt.minisig to $tag."

read -r -p "Publish release $tag? [y/N] " answer
if [[ $answer =~ ^[Yy]$ ]]; then
	gh release edit "$tag" -R "$repo" --draft=false
	echo "Published $tag."
else
	echo "Left $tag as a draft. Publish it with: gh release edit $tag -R $repo --draft=false"
fi
