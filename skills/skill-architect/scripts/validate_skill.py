#!/usr/bin/env python3
import sys
import os
import re

def parse_frontmatter(content):
    if not content.startswith("---"):
        return None, "File must start with '---'"
    
    try:
        end_idx = content.index("---", 3)
    except ValueError:
        return None, "Missing closing '---' for frontmatter"
    
    yaml_block = content[3:end_idx]
    data = {}
    for line in yaml_block.split('\n'):
        line = line.strip()
        if not line or line.startswith("#"): continue
        if ":" in line:
            key, val = line.split(":", 1)
            data[key.strip()] = val.strip()
    
    return data, None

def validate_skill(path):
    print(f"Validating skill at: {path}")
    
    skill_md = os.path.join(path, "SKILL.md")
    if not os.path.exists(skill_md):
        print("FAIL: SKILL.md not found")
        return False
        
    with open(skill_md, 'r') as f:
        content = f.read()
        
    data, err = parse_frontmatter(content)
    if err:
        print(f"FAIL: Invalid Frontmatter - {err}")
        return False
        
    # Check Name
    name = data.get("name", "")
    if not name:
        print("FAIL: 'name' field is missing")
        return False
    
    if not re.match(r'^[a-z0-9-]+$', name):
        print(f"FAIL: Invalid name '{name}'. Must be lowercase, numbers, and hyphens only.")
        return False
        
    if len(name) > 64:
        print("FAIL: Name is too long (>64 chars)")
        return False
        
    # Check Description
    desc = data.get("description", "")
    if not desc:
        print("FAIL: 'description' field is missing")
        return False
        
    if len(desc) > 1024:
        print("FAIL: Description is too long (>1024 chars)")
        return False
        
    print("PASS: Skill structure is valid.")
    return True

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: validate_skill.py <path>")
        sys.exit(1)
        
    path = sys.argv[1]
    if not validate_skill(path):
        sys.exit(1)
