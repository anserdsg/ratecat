# Download bunlde and unpack
./test/download.sh re /tmp/repo.dat
./test/unpack-git-bundle.sh /tmp/repo.dat /tmp/bundles e3k5e3kv

# Pack patch
./test/pack-git-patch.sh ~/projects/eqp-hub commit /tmp/fpc.dat e3k5e3kv
./test/pack-git-patch.sh ~/projects/fdc-kernel-platform/ commit /tmp/fpc.dat e3k5e3kv


cat /tmp/fpc.dat |xclip -selection clipboard
