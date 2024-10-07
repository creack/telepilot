#!/usr/bin/env sh

# Write a 60MB file, with 1MB/s limit should take 1min.
dd if=/dev/zero of=/tmp/testfile bs=1M count=60 oflag=direct
