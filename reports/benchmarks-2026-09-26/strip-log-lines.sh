#!/usr/bin/env bash
# Removes log lines that a benchmark wrote to stderr (merged into the raw output) and
# rejoins a benchmark name with its result line when a log line split them.
# Measured values are not changed.
exec awk '
  /^[0-9][0-9][0-9][0-9]\/[0-9][0-9]\/[0-9][0-9] [0-9:]+ / { next }
  /^Benchmark/ && !/ns\/op/ { sub(/[ \t]*[0-9][0-9][0-9][0-9]\/[0-9][0-9]\/[0-9][0-9] .*$/, ""); held = $0; next }
  held != "" && /^[ \t]*[0-9]+[ \t].*ns\/op/ { sub(/^[ \t]+/, ""); print held "\t" $0; held = ""; next }
  { print }
'
