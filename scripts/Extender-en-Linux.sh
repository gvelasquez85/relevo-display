#!/bin/bash
set -euo pipefail
trial_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
case "$(uname -m)" in
  x86_64) trial_arch=amd64 ;;
  aarch64|arm64) trial_arch=arm64 ;;
  *) echo "Arquitectura no soportada"; exit 1 ;;
esac
exec "$trial_dir/relevo-linux-$trial_arch" --mode extend --duration 60s --fps 5 "$@"
