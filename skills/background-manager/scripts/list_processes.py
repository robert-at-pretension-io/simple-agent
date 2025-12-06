import os
import sys
import datetime

# Add current dir to sys.path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from manager_utils import load_state, save_state

def is_pid_running(pid):
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    else:
        return True

def main():
    state = load_state()
    if not state:
        print("No background processes found.")
        return

    # Update status
    dirty = False
    for pid_id, info in state.items():
        if info['status'] == 'running':
            if not is_pid_running(info['pid']):
                info['status'] = 'stopped'
                info['end_time'] = datetime.datetime.now().isoformat()
                dirty = True

    if dirty:
        save_state(state)

    # Print table
    # Columns: ID, PID, Status, Started, Command
    print(f"{'ID':<10} {'PID':<8} {'STATUS':<10} {'STARTED':<20} {'COMMAND'}")
    print("-" * 80)
    
    for pid_id, info in state.items():
        # Truncate command if too long
        cmd = info.get('command', '')
        if len(cmd) > 40:
            cmd = cmd[:37] + "..."
            
        start_time = info.get('start_time', '')
        # Simplify time format if possible, just take HH:MM:SS
        try:
            dt = datetime.datetime.fromisoformat(start_time)
            start_str = dt.strftime("%Y-%m-%d %H:%M:%S")
        except:
            start_str = start_time[:19]

        print(f"{pid_id:<10} {str(info.get('pid')):<8} {info.get('status'):<10} {start_str:<20} {cmd}")

if __name__ == "__main__":
    main()
