#!/bin/sh

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <input_file> <output_file> <password>"
    exit 1
fi

input_file=$1
output_file=$2
password=$3

# check if input_file exists
if [ ! -f $input_file ]; then
    echo "Error: $input_file does not exist"
    exit 1
fi

# decrypt the input file by openssl with pbkdf2 key derivation
openssl enc -d -aes-256-cbc -salt -pbkdf2 -in $input_file -out $output_file -k $password

echo "File $input_file is decrypted to $output_file"