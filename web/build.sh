#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p public
SHA=$(git -C .. rev-parse --short HEAD)
node -e "const fs=require('fs');const s=process.argv[1];fs.writeFileSync('src/lib/version.generated.ts','export const BUILD_VERSION = \"'+s+'\";\n');fs.writeFileSync('public/version.json',JSON.stringify({version:s}));" "$SHA"
npx vite build
