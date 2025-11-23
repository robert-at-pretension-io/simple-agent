import subprocess
import re
import sys

# Patterns to check
PATTERNS = {
    "AWS Access Key": r"AKIA[0-9A-Z]{16}",
    "Google API Key": r"AIza[0-9A-Za-z\\-_]{35}",
    "Slack Token": r"xox[baprs]-([0-9a-zA-Z]{10,48})?",
    "Private Key": r"-----BEGIN .*PRIVATE KEY-----",
    "Generic Secret Assignment": r"(?i)(password|secret|api_key|access_token)\s*[:=]\s*['\"][A-Za-z0-9_\-]{16,}['\"]"
}

def get_staged_files():
    try:
        result = subprocess.run(
            ["git", "diff", "--name-only", "--cached"],
            capture_output=True, text=True, check=True
        )
        return [f for f in result.stdout.splitlines() if f.strip()]
    except subprocess.CalledProcessError:
        return []

def scan_file(filepath):
    issues = []
    try:
        with open(filepath, "r", encoding="utf-8") as f:
            content = f.read()
            for name, pattern in PATTERNS.items():
                if re.search(pattern, content):
                    issues.append(name)
    except UnicodeDecodeError:
        # Skip binary files
        pass
    except FileNotFoundError:
        pass
    return issues

def main():
    files = get_staged_files()
    if not files:
        sys.exit(0)

    failed = False
    print("🔎 Scanning staged files for secrets...")

    for file in files:
        # Skip the script itself if it ends up being scanned (unlikely but good practice)
        if "check_secrets.py" in file:
            continue
            
        issues = scan_file(file)
        if issues:
            print(f"❌ Potential secrets found in {file}:")
            for issue in issues:
                print(f"   - {issue}")
            failed = True

    if failed:
        print("Please verify these are not real secrets. If they are placeholders, ignore this.")
        sys.exit(1)
    
    sys.exit(0)

if __name__ == "__main__":
    main()