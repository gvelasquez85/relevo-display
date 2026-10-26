#!/usr/bin/env python3
"""Deprecated split packager: use the single Windows release pipeline."""
from pathlib import Path
import runpy
runpy.run_path(str(Path(__file__).with_name("package.py")),run_name="__main__")
