#!/bin/bash
if [ -f "remember.txt" ]; then
    cat remember.txt
else
    echo "No memory file found."
fi
