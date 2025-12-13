#!/bin/bash
# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <input_file> <output_file> <password>"
    exit 1
fi
in_file=$1
out_file=$2
password=$3

CURDIR=`pwd`

temp_file=$(mktemp)
temp_encoded_file=$(mktemp)

zstd -19 ${in_file} -f -o $temp_file
openssl enc -aes-256-cbc -salt -pbkdf2 -in $temp_file -out $temp_encoded_file -k $password
base64 -w 0 $temp_encoded_file > $out_file
ls -alh $out_file

rm $temp_file $temp_encoded_file

cd $CURDIR
