#!/bin/sh

# check arguments
if [ $# -ne 4 ]; then
    echo "Usage: $0 <prefix> <chunk_num> <output_file> <password>"
    exit 1
fi

prefix=$1
chunk_num=$2
output_file=$3
password=$4

DL_CMD="go run ./cmd/repo/main.go -t https://5at4ocenoa.execute-api.ap-southeast-1.amazonaws.com/default/repo -a read"

temp_dir=$(mktemp -d)
input_file=$(mktemp)

# download chunks
i=1
while [ $i -le $chunk_num ]; do
    chunk_key="${prefix}[${i}]"
    chunk_file="$temp_dir/chunk_$i"

    $DL_CMD "$chunk_key" "$chunk_file"

    if [ ! -f "$chunk_file" ]; then
        echo "Error: chunk $i ($chunk_key) does not exist"
        exit 1
    fi

    cat "$chunk_file" >> "$input_file"

    i=$((i + 1))
done

temp_file=$(mktemp)
base64 -d "$input_file" > "$temp_file"

# decrypt the input file by openssl with pbkdf2 key derivation
openssl enc -d -aes-256-cbc -salt -pbkdf2 -in "$temp_file" -out "$output_file" -k "$password"

rm -f "$temp_file" "$input_file"
rm -rf "$temp_dir"

echo "File $output_file was reconstructed and decrypted"
