#!/usr/bin/env bash
WEEK=$1
if [ "x$WEEK" = "x" ]; then
    WEEK="4"
fi

echo "Find branches ${WEEK} week ago"
for k in $(git branch | sed /\*/d); do
    cond="--since='${WEEK} week ago'"
    result=`git log -1 "${cond}" -s $k`
    if [ -z "${result}" ]; then
        echo "Remove branch $k"
        git branch -D $k
    fi
done