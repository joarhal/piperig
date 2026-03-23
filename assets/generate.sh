#!/bin/bash
# Generate PNG screenshots from terminal HTML files.
# Requires: puppeteer (npx puppeteer browsers install chrome)
#
# Usage:
#   ./assets/generate.sh              # all terminals
#   ./assets/generate.sh banner       # single terminal

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TERMINALS_DIR="$SCRIPT_DIR/terminals"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

generate() {
  local name="$1"
  local html="$TERMINALS_DIR/${name}.html"
  local png="$SCRIPT_DIR/${name}.png"

  if [ ! -f "$html" ]; then
    echo "error: $html not found" >&2
    exit 1
  fi

  echo "generating $png"

  cd /tmp && node --input-type=module <<SCRIPT
import puppeteer from 'puppeteer';
const browser = await puppeteer.launch();
const page = await browser.newPage();
await page.setViewport({ width: 1200, height: 900 });
await page.goto('file://${html}');
const el = await page.\$('.terminal');
await el.screenshot({ path: '${png}', omitBackground: true });
await browser.close();
SCRIPT
}

if [ $# -gt 0 ]; then
  generate "$1"
else
  for html in "$TERMINALS_DIR"/*.html; do
    name="$(basename "$html" .html)"
    generate "$name"
  done
fi

echo "done"
