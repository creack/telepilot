#!/usr/bin/env bash

# Allocate more and more memory for a minute.
# As we use cgroups v2, we may not get OOM but the process memory
# usage should be capped as the set limit.
allocate_memory() {
  declare -a mem_array
  for i in {0..600}; do
    # Allocate 10 MB chunks of memory and store in an array to keep it from being garbage collected.
    mem_array+=("$(head -c 10M /dev/zero | tr '\0' 'a')")
    sleep 0.1
  done
}

allocate_memory
