#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")/.."
python3 scripts/make-icon.py
GOBIN="$PWD/.tools/bin" .tools/go/bin/go install github.com/akavel/rsrc@v0.10.2
.tools/bin/rsrc -arch amd64 -ico assets/relevo-monitor.ico -manifest assets/windows.manifest -o cmd/relevo/rsrc_windows_amd64.syso
