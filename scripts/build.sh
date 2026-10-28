#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")/.."
if [ -x .tools/go/bin/go ]; then relevo_go="$PWD/.tools/go/bin/go"; else relevo_go="$(command -v go)"; fi
cp THIRD_PARTY_NOTICES.txt internal/wininstall/third_party_notices.txt
"$relevo_go" test -race ./...
"$relevo_go" vet ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$relevo_go" vet ./...
mkdir -p dist
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$relevo_go" build -trimpath -ldflags='-s -w -H=windowsgui' -o dist/Relevo-Windows.exe ./cmd/relevo
python3 scripts/package.py
