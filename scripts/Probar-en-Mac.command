#!/bin/bash
set -uo pipefail
trial_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
trial_bin="$trial_dir/relevo-macos-arm64"
if [ ! -x "$trial_bin" ]; then
  trial_bin="$trial_dir/../dist/relevo-macos-arm64"
fi
if [ ! -x "$trial_bin" ]; then
  echo "No se encuentra el ejecutable Relevo Display junto al lanzador."
  read -r -p "Pulsa Enter para cerrar. "
  exit 1
fi
trial_restore=0
if /usr/bin/pgrep -f '^/Applications/Yam Display.app/Contents/MacOS/Yam Display$' >/dev/null; then
  trial_restore=1
fi
trial_cleanup() {
  if [ "$trial_restore" -eq 1 ]; then
    /usr/bin/open -a '/Applications/Yam Display.app'
  fi
}
trap trial_cleanup EXIT
if [ "$trial_restore" -eq 1 ]; then
  /usr/bin/osascript -e 'tell application "Yam Display" to quit' || exit 1
  /bin/sleep 2
fi
echo "Prueba de 20 segundos. Mantén Yam Display abierto en el iPad."
"$trial_bin" --duration 20s --fps 5 "$@"
trial_status=$?
trial_cleanup
trap - EXIT
echo "Resultado: $trial_status. El servidor original se ha vuelto a abrir si estaba activo."
read -r -p "Pulsa Enter para cerrar. "
exit "$trial_status"
