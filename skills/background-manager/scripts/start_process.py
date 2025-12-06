import argparse
import subprocess
import os
import sys
import time
import uuid
import datetime
import shlex

# Add current dir to sys.path to import manager_utils
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from manager_utils import load_state, save_state, DATA_DIR, get_password

def main():
    # Ensure password is present before starting
    get_password()

    parser = argparse.ArgumentParser(description="Start a background process")
    parser.add_argument("command", nargs="+", help="The command to run")
    args = parser.parse_args()

    command_list = args.command
    # If the user provided a single string with spaces but didn't quote it properly in the shell call, 
    # it might come as multiple args. If they provided a complex command string, we might want to use shell=True 
    # or shlex.split. 
    # Generally, for security and predictability, executing the list directly is better if possible.
    # But often users want to run "ls -la | grep foo", which requires shell=True.
    
    # Let's try to detect if it's a complex command.
    full_command_str = " ".join(command_list)
    
    process_id = str(uuid.uuid4())[:8]
    logs_dir = os.path.join(DATA_DIR, 'logs')
    inputs_dir = os.path.join(DATA_DIR, 'inputs')
    os.makedirs(logs_dir, exist_ok=True)
    os.makedirs(inputs_dir, exist_ok=True)
    log_file_path = os.path.join(logs_dir, f"{process_id}.log.enc")
    input_file_path = os.path.join(inputs_dir, f"{process_id}.in")
    
    # Use process_supervisor.py to handle PTY and logging
    supervisor_script = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'process_supervisor.py')
    
    # We pass the command as a single argument to the supervisor
    supervisor_cmd = [
        sys.executable,
        supervisor_script,
        full_command_str,
        log_file_path,
        input_file_path
    ]
    
    try:
        proc = subprocess.Popen(
            supervisor_cmd,
            shell=False, # We are executing python directly
            start_new_session=True,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            cwd=os.getcwd(),
            env=os.environ.copy() # Ensure BG_PASSWORD is passed
        )
            
        pid = proc.pid
        
        # Wait briefly to ensure supervisor starts successfully (e.g. openssl check)
        time.sleep(0.5)
        if proc.poll() is not None:
            print(f"Failed to start process (Exit Code: {proc.returncode}). Check BG_PASSWORD.")
            sys.exit(1)
        
        state = load_state()
        state[process_id] = {
            "id": process_id,
            "pid": pid,
            "command": full_command_str,
            "start_time": datetime.datetime.now().isoformat(),
            "status": "running",
            "log_file": log_file_path,
            "input_file": input_file_path,
            "cwd": os.getcwd()
        }
        save_state(state)
        
        print(f"Started process {process_id} (PID {pid})")
        print(f"Logs: {log_file_path}")
        
    except Exception as e:
        print(f"Failed to start process: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
