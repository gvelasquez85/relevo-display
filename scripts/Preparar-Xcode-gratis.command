#!/bin/bash
set -euo pipefail
archive="${1:-}"
if [ -z "$archive" ]; then echo "Uso: Preparar-Xcode-gratis.command /ruta/Xcode.xip [/ruta/destino]" >&2; exit 2; fi
relevo_project="$(cd -- "$(dirname -- "$0")/.." && pwd)"
destination="${2:-$relevo_project/.tools/xcode-full}"
python3 - "$archive" "$destination" <<'CHECK'
import pathlib,shutil,sys
archive=pathlib.Path(sys.argv[1]); destination=pathlib.Path(sys.argv[2])
if not archive.is_file():raise SystemExit('No se encontro el XIP oficial.')
parent=destination
while not parent.exists():parent=parent.parent
free=shutil.disk_usage(parent).free
if free < 35*2**30:raise SystemExit(f'Se requieren 35 GiB libres; disponibles {free/2**30:.1f} GiB. No se extrajo nada.')
if destination.exists() and any(destination.iterdir()):raise SystemExit('El destino ya contiene archivos; no se sobrescribira.')
CHECK
/usr/sbin/pkgutil --check-signature "$archive"
mkdir -p "$destination"
cd "$destination"
/usr/bin/xip --expand "$archive"
echo "Extraccion terminada. Pendiente comprobar arranque y firma Personal Team en Xcode."
