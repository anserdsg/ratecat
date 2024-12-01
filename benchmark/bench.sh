#!/bin/bash
program_name=$0

function print_usage {
    echo ""
    echo "Usage: $program_name server_addr [concurrency] [cmd_count]"
    echo ""
    echo "server_addr:  The server address for benchmarking"
    echo "concurrency:  The number of concurrency connections"
    echo "cmd_count:    The number of allow commands"
    echo ""
    echo "Example:"
    echo "$program_name 1000 1000"
    echo ""
    exit 1
}

# if less than two arguments supplied, display usage
if [  $# -le 2 ]; then
  print_usage
  exit 1
fi

if [[ ( $# == "--help") ||  $# == "-h" ]]; then
  print_usage
  exit 0
fi

server_addr=$1
concurrency=$2
cmd_count=$3

echo "Build server"
go build -tags=gc_opt -ldflags="-s -w" -o ./ratecat_bench_server ../cmd/ratecat

echo "Build benchmark client"
go build -ldflags="-s -w" -o ./ratecat_bench ./bench.go

export RC_ENV=production
export RC_LOG_LEVEL=warn

function go_bench() {
    echo "====================================================="
    export RC_MULTI_CORE=$1
    export RC_WORKER_POOL=$2
    echo "Benchmark RC_MULTI_CORE=${1} RC_WORKER_POOL=${2}"
    ./ratecat_bench_server 2>&1 &

    echo "Warm up server"
    sleep 1

    ./ratecat_bench --address ${server_addr} --concurrency ${concurrency} --cmd_count ${cmd_count}
    kill -9 $(jobs -rp)
    wait $(jobs -rp) 2>/dev/null
}

go_bench 1 1
go_bench 1 0
go_bench 0 1
go_bench 0 0

rm -f ./ratecat_bench_server ./ratecat_bench

