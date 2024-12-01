#!/usr/bin/env bash

set -euo pipefail

# shopt -s globstar

if ! [[ "$0" =~ scripts/proto_gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

PROTO_MODULE_PATH=$(go list -f '{{.Dir}}' "github.com/gogo/protobuf/proto")
PROTO_ROOT=$(realpath ${PROTO_MODULE_PATH}/..)
PROTO_INCLUDE_PATH=.:${PROTO_ROOT}:${PROTO_ROOT}/protobuf
PROTO_OUT="plugins=grpc,\
Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types"
PROTO_TEMP_DIR=$(mktemp -d)

function gen_protobuf() {
  base_dir=${1}
  proto_name=${2}
  protoc -I=${PROTO_INCLUDE_PATH} --gogofaster_out=${PROTO_OUT}:${PROTO_TEMP_DIR} ./${base_dir}/${proto_name}.proto
  if [ -f ${PROTO_TEMP_DIR}/github.com/anserdsg/ratecat/v1/${base_dir}/${proto_name}.pb.go ]; then
    mv ${PROTO_TEMP_DIR}/github.com/anserdsg/ratecat/v1/${base_dir}/${proto_name}.pb.go ./${base_dir}/${proto_name}.pb.go
    echo "./${base_dir}/${proto_name}.pb.go generated"
  else
    exit 1
  fi
}

gen_protobuf "proto" "ratecat_api_v1"
gen_protobuf "internal/cluster/pb" "ratecat_cluster_v1"

rm -rf ${PROTO_TEMP_DIR}
