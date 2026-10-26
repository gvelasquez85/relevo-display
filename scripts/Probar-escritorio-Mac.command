#!/bin/bash
set -euo pipefail
trial_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
exec "$trial_dir/Probar-en-Mac.command" --mode desktop
