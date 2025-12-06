import argparse
import os
import sys
import signal
import datetime

# Add current dir to sys.path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from manager_utils import load_state, save_state

def main():
    parser = argparse.ArgumentParser(description="Stop a background process")
    parser.add_argument("id", help="Process ID (or partial ID)")
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
    pid = info.get('pid')
    
    if info.get('status') != 'running':
        print(f"Process {process_id} is already {info.get('status')}")
        return

    try:
        # Kill the process group
        os.killpg(pid, signal.SIGTERM)
        print(f"Stopped process {process_id} (PID {pid})")
        
        info['status'] = 'stopped'
        info['end_time'] = datetime.datetime.now().isoformat()
        save_state(state)
        
    except ProcessLookupError:
        print(f"Process {pid} not found (already dead?)")
        info['status'] = 'stopped'
        info['end_time'] = datetime.datetime.now().isoformat()
        save_state(state)
    except Exception as e:
        print(f"Error stopping process: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
