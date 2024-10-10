#!/usr/bin/env sh

# Assumes that /tmp is on a block device.

# Write a 5MB file, with 1MB/s limit should take 5 secs.
echo "Testing write." >&2
dd if=/dev/zero of=/tmp/testfile bs=1M count=5 oflag=direct

# Read a 5MB file, with 1MB/s limit should take 5 secs.
echo "Testing read." >&2
dd if=/tmp/testfile of=/dev/null

rm -f /tmp/testfile
