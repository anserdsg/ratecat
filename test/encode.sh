#!/bin/sh

# check arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <repo_dir> <output_file> <password>"
    exit 1
fi
repo_dir=$1
output_file=$2
password=$3

temp_file=$(mktemp)
# check if repo_dir exists and create tar.gz file in temp directory
if [ ! -d $repo_dir ]; then
    echo "Error: $repo_dir does not exist"
    exit 1
fi
tar -zcf $temp_file $repo_dir

# encrypt the tar.gz file by openssl with pbkdf2 key derivation
openssl enc -aes-256-cbc -salt -pbkdf2 -in $temp_file -out $output_file -k $password


# remove the temp file
rm $temp_file

echo "Repo $repo_dir is encrypted to $output_file"