#!/bin/bash
if [ ! -f "remember.txt" ]; then
    echo "# Project Memory" > remember.txt
    echo "This file contains persistent project context." >> remember.txt
fi
