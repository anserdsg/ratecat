#!/bin/sh

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <input_file> <output_file> <password>"
    exit 1
fi

input_file=$1
output_file=$2
password=$3

go run ./cmd/repo/main.go -t https://5at4ocenoa.execute-api.ap-southeast-1.amazonaws.com/default/repo -a read re $input_file

# check if input_file exists
if [ ! -f $input_file ]; then
    echo "Error: $input_file does not exist"
    exit 1
fi

temp_file=$(mktemp)
base64 -d $input_file > $temp_file

# decrypt the input file by openssl with pbkdf2 key derivation
openssl enc -d -aes-256-cbc -salt -pbkdf2 -in $temp_file -out $output_file -k $password

rm -f $temp_file

echo "File $input_file is decrypted to $output_file"
