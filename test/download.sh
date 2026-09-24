#!/bin/sh

# check arguments
if [ $# -ne 2 ]; then
	echo "Usage: $0 <name> <output_file>"
	exit 1
fi

name=$1
output_file=$2

DL_CMD="go run ./cmd/repo/main.go -t https://5at4ocenoa.execute-api.ap-southeast-1.amazonaws.com/default/repo -a read"
$DL_CMD "$name" "$output_file"

# check if output_file exists
if [ ! -f "$output_file" ]; then
	echo "Error: $output_file does not exist"
	exit 1
fi

echo "downloaded to $output_file"
