#! /bin/bash
set -e

if [[ -z "${SEEDER}" ]]; then
  # set SEEDER var from ARGs if env var is empty
  SEEDER=$1
  # if ARG and ENV empty Error out
  [ -n "$SEEDER" ] || ( echo "FATAL: must provide SEEDER variable as ENV or Arg" ; exit 1 ; )
fi

# this IP will be then read by golang using os.env
export SEEDER
ginkgo -r --randomizeAllSpecs --randomizeSuites --failFast --cover --trace --race --progress -v
