#!/usr/bin/env bash
set -euo pipefail

if ! [[ "$0" =~ test/cluster/start_cluster.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

if [ "x$1" = "x" ]; then
  echo "Usage: `basename $0` endpoint"
  exit 1
fi

if [ $1 -lt 1 ]; then
  echo "endpoint must be > 0"
  exit 1
fi



api_port=$((4117 + ($1 - 1) * 100))
raft_port=$((4118 + ($1 - 1) * 100))

echo "Build ratecat server"
make build-ratecat

export RC_ENV="production"
export RC_LOG_LEVEL="info"
export RC_MULTI_CORE="true"
export RC_WORK_POOL="true"
export RC_METRIC_ENABLED="false"
export RC_CLUSTER_ENABLED="true"
export RC_API_LISTEN_URL="tcp://0.0.0.0:${api_port}"
export RC_API_ADVERTISE_URL="tcp://127.0.0.01:${api_port}"
export RC_CLUSTER_URL="tcp://node${1}@127.0.0.1:${raft_port}"
export RC_CLUSTER_STATE_DIR="/tmp/rc${1}"

shift 1
peers=$@
peers_env=""
for peer in $peers; do
  if [ $peer -lt 1 ]; then
    echo "peer number must be > 0"
    exit 1
  fi
  raft_port=$((4118 + ($peer - 1) * 100))
  peers_env="${peers_env},tcp://node${peer}@127.0.0.1:${raft_port}"
done
peers_env=${peers_env#,*}
export RC_CLUSTER_PEER_URLS=${peers_env}

./ratecat