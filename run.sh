#!/bin/bash

set -ex

exec /headless-shell/run.sh &

sleep 1
exec /mylinks/mylinks -port 8080 -data data "$@"
