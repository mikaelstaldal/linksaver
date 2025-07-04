#!/bin/bash

set -ex

exec /headless-shell/run.sh &

exec /linksaver/linkserver -port 8080
