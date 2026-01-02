#!/bin/bash

set -ex

exec /headless-shell/run.sh &

sleep 1
exec /linksaver/linkserver -port 8080 -data data "$@"
