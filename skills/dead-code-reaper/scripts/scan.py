#!/usr/bin/env python3
import os
import re
import subprocess

def find_exported_funcs():
    exported = {}
    # regex for 'func Name('
    pattern = re.compile(r'^func ([A-Z][a-zA-Z0-9_]*)\(')
    
    for root, dirs, files in os.walk("."):
        if "vendor" in root or ".git" in root: continue
        for file in files:
            if file.endswith(".go") and not file.endswith("_test.go"):
                path = os.path.join(root, file)
                with open(path, 'r') as f:
                    for line in f:
                        match = pattern.match(line)
                        if match:
                            func_name = match.group(1)
                            exported[func_name] = path
    return exported

def check_usage(name, file_path):
    # simplistic grep check
    cmd = ["grep", "-r", "--include=*.go", name, "."]
    result = subprocess.run(cmd, capture_output=True, text=True)
    # count occurrences. If only 1 (definition), it's unused.
    lines = result.stdout.strip().split('\n')
    return len(lines)

print("Scanning for unused exported functions...")
funcs = find_exported_funcs()
for name, path in funcs.items():
    count = check_usage(name, path)
    if count <= 1:
        print(f"POTENTIAL DEAD CODE: {name} in {path}")