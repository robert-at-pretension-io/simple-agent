#!/usr/bin/env python3
import sys

def check_complexity(file_path):
    MAX_DEPTH = 5
    try:
        with open(file_path, 'r') as f:
            lines = f.readlines()
            
        max_detected = 0
        for i, line in enumerate(lines):
            stripped = line.lstrip()
            if not stripped or stripped.startswith("//"): continue
            
            # count tabs or 4-spaces
            indent = len(line) - len(stripped)
            # assume 1 tab = 4 spaces
            depth = indent // 4
            if line.startswith("\t"):
                 depth = indent 
            
            if depth > MAX_DEPTH:
                print(f"WARNING: High complexity in {file_path}:{i+1}. Depth: {depth}")
                return
                
    except Exception as e:
        pass # ignore errors

if len(sys.argv) > 1:
    check_complexity(sys.argv[1])