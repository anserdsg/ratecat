#!/bin/bash

# check arguments
if [ $# -ne 4 ]; then
    echo "Usage: $0 <git_repo_dir> <from_git_commit> <output_file> <credential>"
    exit 1
fi

repo_dir=$1
from_commit=$2
output_file=$3
credential=$4

# check if repo_dir exists
if [ ! -d "$repo_dir" ]; then
    echo "Error: $repo_dir does not exist"
    exit 1
fi

# Get absolute path of output file to ensure we can write to it after changing directory
# If realpath is not available, we might have issues with relative paths for output_file
if command -v realpath >/dev/null 2>&1; then
    output_file=$(realpath "$output_file")
else
    # Fallback if realpath is missing: assume output_file is relative to current dir if not starting with /
    if [[ "$output_file" != /* ]]; then
        output_file="$(pwd)/$output_file"
    fi
fi

temp_patch_dir=$(mktemp -d)
temp_compressed=$(mktemp)
temp_encrypted=$(mktemp)

cleanup() {
    rm -rf "$temp_patch_dir"
    rm -f "$temp_compressed" "$temp_encrypted"
}
trap cleanup EXIT

# Change to repo dir
cd "$repo_dir" || exit 1

# Generate patch
echo "Generating patches from $from_commit to HEAD in $repo_dir..."
if ! git format-patch -o "$temp_patch_dir" "$from_commit"..HEAD; then
    echo "Error: Failed to generate patch. Check if commit hash is valid."
    exit 1
fi

echo "Patches generated:"
ls -1 "$temp_patch_dir"

# Encoding process
# 1. Compress (tar + zstd)
tar -C "$temp_patch_dir" -cf - . | zstd -19 - > "$temp_compressed"

# 2. Encrypt
openssl enc -aes-256-cbc -salt -pbkdf2 -in "$temp_compressed" -out "$temp_encrypted" -k "$credential"

# 3. Base64 encode
base64 -w 0 "$temp_encrypted" > "$output_file"

echo "Patch packed to $output_file"
ls -alh "$output_file"
