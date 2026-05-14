#!/usr/bin/env bash
set -euo pipefail

OUTPUT_FLAG=""
while getopts "o:" opt; do
  case $opt in
    o)
      OUTPUT_FLAG="-o $OPTARG"
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
  esac
done

go build -tags netgo $OUTPUT_FLAG .
go test ./...
golangci-lint run ./...
