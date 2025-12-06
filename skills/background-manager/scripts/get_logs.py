import argparse
import os
import sys
import subprocess

# Add current dir to sys.path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from manager_utils import load_state, get_password

def main():
    get_password()
    parser = argparse.ArgumentParser(description="Get logs for a background process")
    parser.add_argument("id", help="Process ID (or partial ID)")
    parser.add_argument("-n", "--lines", type=int, default=20, help="Number of lines to show from the end (default: 20). Set to 0 for all.")
    args = parser.parse_args()

    state = load_state()
    
    # Find process
    matches = [k for k in state.keys() if k.startswith(args.id)]
    if not matches:
        print(f"No process found with ID starting with '{args.id}'")
        sys.exit(1)
    if len(matches) > 1:
        print(f"Multiple processes found: {', '.join(matches)}")
        sys.exit(1)
        
    process_id = matches[0]
    info = state[process_id]
    log_file = info.get('log_file')
    
    if not log_file or not os.path.exists(log_file):
        print(f"Log file not found for process {process_id}")
        sys.exit(1)
        
    try:
        cmd = [
            'openssl', 'enc', '-d', '-aes-256-cbc', '-pbkdf2',
            '-pass', 'env:BG_PASSWORD',
            '-in', log_file
        ]
        
        print(f"--- Logs for {process_id} ---")
        
        # Use Popen to stream output instead of buffering in memory
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        
        if args.lines <= 0:
            # Print all lines directly
            for line in proc.stdout:
                print(line.decode('utf-8', errors='replace'), end='')
        else:
            from collections import deque
            # Use deque to keep only the last N lines in memory
            last_lines = deque(proc.stdout, maxlen=args.lines)
            for line in last_lines:
                print(line.decode('utf-8', errors='replace'), end='')

        proc.wait()
        
    except Exception as e:
        print(f"Error reading log file: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
