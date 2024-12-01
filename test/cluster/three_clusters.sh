#!/usr/bin/env bash
set -euo pipefail

if ! [[ "$0" =~ test/cluster/three_clusters.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

echo "Build ratecat server"
make build-ratecat

# function kill_background_jobs() {
#   echo "Killing all background jobs"
#   kill -9 $(jobs -rp)
#   wait $(jobs -rp) 2>/dev/null
#   echo "Done"
# }
# trap kill_background_jobs SIGINT SIGQUIT SIGTERM


export RC_ENV="development"
export RC_LOG_LEVEL="debug"
export RC_MULTI_CORE="true"
export RC_WORK_POOL="true"
export RC_METRIC_ENABLED="false"
export RC_CLUSTER_ENABLED="true"

RC_API_LISTEN_URL="tcp://0.0.0.0:4117" \
RC_CLUSTER_URL="tcp://node1@127.0.0.1:4118" \
RC_CLUSTER_PEER_URLS="tcp://node2@127.0.0.1:4218,tcp://node3@127.0.0.1:4318" \
RC_CLUSTER_STATE_DIR="/tmp/rc1" \
./ratecat &

RC_API_LISTEN_URL="tcp://0.0.0.0:4217" \
RC_CLUSTER_URL="tcp://node2@127.0.0.1:4218" \
RC_CLUSTER_PEER_URLS="tcp://node1@127.0.0.1:4118,tcp://node3@127.0.0.1:4318" \
RC_CLUSTER_STATE_DIR="/tmp/rc2" \
./ratecat &

RC_API_LISTEN_URL="tcp://0.0.0.0:4317" \
RC_CLUSTER_URL="tcp://node3@127.0.0.1:4318" \
RC_CLUSTER_PEER_URLS="tcp://node1@127.0.0.1:4118,tcp://node2@127.0.0.1:4218" \
RC_CLUSTER_STATE_DIR="/tmp/rc3" \
./ratecat




