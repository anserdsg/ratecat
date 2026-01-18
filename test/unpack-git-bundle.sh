#!/bin/bash

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <packed_file> <output_dir> <credential>"
    exit 1
fi

packed_file=$1
output_dir=$2
credential=$3

# check if packed_file exists
if [ ! -f "$packed_file" ]; then
    echo "Error: $packed_file does not exist"
    exit 1
fi

# create output_dir if it does not exist
if [ ! -d "$output_dir" ]; then
    mkdir -p "$output_dir"
else
    # remove existing patch files
    find "$output_dir" -maxdepth 1 -name "*.patch" -type f -delete
fi

temp_encrypted=$(mktemp)
temp_compressed=$(mktemp)

cleanup() {
    rm -f "$temp_encrypted" "$temp_compressed"
}
trap cleanup EXIT

# 1. Base64 decode
base64 -d "$packed_file" > "$temp_encrypted"

# 2. Decrypt
openssl enc -d -aes-256-cbc -salt -pbkdf2 -in "$temp_encrypted" -out "$temp_compressed" -k "$credential"
if [ $? -ne 0 ]; then
    echo "Error: Decryption failed. Wrong credential?"
    exit 1
fi

# 3. Decompress and untar
# zstd -d to stdout, pipe to tar extract
zstd -d -c "$temp_compressed" | tar -xf - -C "$output_dir"

echo "Patches unpacked to $output_dir:"
ls -a1h "$output_dir"
