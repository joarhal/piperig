#!/bin/bash
# Fails FAIL_COUNT times, then succeeds.
# Uses a counter file to track attempts.
COUNTER_FILE="${COUNTER_FILE:-/tmp/flaky_counter}"
if [ ! -f "$COUNTER_FILE" ]; then
  echo 0 > "$COUNTER_FILE"
fi
COUNT=$(cat "$COUNTER_FILE")
COUNT=$((COUNT + 1))
echo $COUNT > "$COUNTER_FILE"
if [ "$COUNT" -le "${FAIL_COUNT:-0}" ]; then
  exit 1
fi
exit 0
