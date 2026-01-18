#!/bin/sh

if [ $# -ne 2 ]; then
	echo "Usage: $0 <filename> <chunk_size_mib>"
	exit 1
fi

file=$1
chunk_size_mib=$2

if [ ! -f "$file" ]; then
	echo "Error: $file does not exist"
	exit 1
fi

case $chunk_size_mib in
	''|*[!0-9]*)
		echo "Error: chunk_size_mib must be a positive integer"
		exit 1
		;;
esac

if [ "$chunk_size_mib" -le 0 ]; then
	echo "Error: chunk_size_mib must be greater than 0"
	exit 1
fi

chunk_bytes=$((chunk_size_mib * 1024 * 1024))
prefix="${file}.chunk_"

split -b "$chunk_bytes" -d --numeric-suffixes=1 --suffix-length=4 "$file" "$prefix"

echo "Split $file into chunks: ${prefix}0001..."
