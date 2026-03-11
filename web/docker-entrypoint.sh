#!/bin/sh
# Write runtime config from API_URL env (used by Cloud Run). Then start the app.
set -e
API_URL="${API_URL:-http://localhost:8080}"
# Escape for JSON: backslash and double-quote
escaped=$(printf '%s' "$API_URL" | sed 's/\\/\\\\/g; s/"/\\"/g')
printf '{"apiUrl":"%s"}\n' "$escaped" > /app/public/config.json
exec node server.js
