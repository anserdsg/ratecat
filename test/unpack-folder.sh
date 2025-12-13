#!/bin/bash

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <input_file> <output_folder> <credential>"
    exit 1
fi

input_file=$1
output_folder=$2
credential=$3

# check if input_file exists
if [ ! -f "$input_file" ]; then
    echo "Error: $input_file does not exist"
    exit 1
fi

# create output_folder if it does not exist
if [ ! -d "$output_folder" ]; then
    mkdir -p "$output_folder"
fi

temp_encrypted=$(mktemp)
temp_archive=$(mktemp)

cleanup() {
    rm -f "$temp_encrypted" "$temp_archive"
}
trap cleanup EXIT

# decode base64
base64 -d "$input_file" > "$temp_encrypted"

# decrypt
openssl enc -d -aes-256-cbc -salt -pbkdf2 -in "$temp_encrypted" -out "$temp_archive" -k "$credential"
if [ $? -ne 0 ]; then
    echo "Error: Decryption failed. Wrong credential?"
    exit 1
fi

# decompress and untar
# zstd -d to stdout, pipe to tar extract
zstd -d -c "$temp_archive" | tar -xf - -C "$output_folder"

echo "File $input_file unpacked to $output_folder"
