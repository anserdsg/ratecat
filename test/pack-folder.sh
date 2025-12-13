#!/bin/bash

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <target_folder> <output_file> <credential>"
    exit 1
fi

target_folder=$1
output_file=$2
credential=$3

# check if target_folder exists
if [ ! -d "$target_folder" ]; then
    echo "Error: $target_folder does not exist"
    exit 1
fi

# Use realpath to handle relative paths and trailing slashes
abs_target_path=$(realpath "$target_folder")
parent_dir=$(dirname "$abs_target_path")
folder_name=$(basename "$abs_target_path")

temp_archive=$(mktemp)
temp_encrypted=$(mktemp)

cleanup() {
    rm -f "$temp_archive" "$temp_encrypted"
}
trap cleanup EXIT

# create tar.zstd file
# keep only one layer of folder means we change to parent dir and tar the folder
tar -cf - -C "$parent_dir" "$folder_name" | zstd -19 - > "$temp_archive"

# encrypt and base64
openssl enc -aes-256-cbc -salt -pbkdf2 -in "$temp_archive" -out "$temp_encrypted" -k "$credential"
base64 -w 0 "$temp_encrypted" > "$output_file"

echo "Folder $target_folder packed and encrypted to $output_file"
ls -alh "$output_file"
