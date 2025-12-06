#!/bin/bash
# Executes the argument as a command.

if [ $# -eq 0 ]; then
  echo "Error: No command provided."
  exit 1
fi

COMMAND="$*"
echo "Executing: $COMMAND"
eval "$COMMAND"