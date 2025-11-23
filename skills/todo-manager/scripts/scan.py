#!/usr/bin/env python3
import os
import sys

def scan_directory(root_dir):
    todo_keywords = ["TODO", "FIXME"]
    ignore_dirs = {".git", "node_modules", "skills", "vendor", "__pycache__"}
    
    found_any = False

    for root, dirs, files in os.walk(root_dir):
        # Modify dirs in-place to skip ignored directories
        dirs[:] = [d for d in dirs if d not in ignore_dirs]
        
        for file in files:
            # Skip binary files or weird extensions if needed, but for now just read all
            file_path = os.path.join(root, file)
            
            try:
                with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                file_matches = []
                for i, line in enumerate(lines):
                    for keyword in todo_keywords:
                        if keyword in line:
                            # Clean up the line for display
                            content = line.strip()
                            # Highlight the keyword (simple uppercase check usually works)
                            file_matches.append((i + 1, keyword, content))
                            break # Avoid double counting if both are on same line
                
                if file_matches:
                    found_any = True
                    print(f"\n📄 {file_path}")
                    for line_num, keyword, content in file_matches:
                        # format:  Line 10: [TODO] fix this
                        # Using standard color codes if we wanted, but keeping it simple for now
                        print(f"   Line {line_num}: {content}")
                        
            except Exception as e:
                # Ignore read errors (permissions, etc)
                continue

    if not found_any:
        print("No TODOs or FIXMEs found! 🎉")

if __name__ == "__main__":
    scan_directory(".")