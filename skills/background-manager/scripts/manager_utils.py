import json
import os
import fcntl
import subprocess
import sys

# Path to the state file
DATA_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), 'data')
STATE_FILE = os.path.join(DATA_DIR, 'processes.json.enc')
LOCK_FILE = os.path.join(DATA_DIR, 'processes.lock')

def get_password():
    password = os.environ.get('BG_PASSWORD')
    if not password:
        print("Error: BG_PASSWORD environment variable is not set.", file=sys.stderr)
        sys.exit(1)
    return password

def load_state():
    get_password() # Ensure password is set

    # Create lock file if it doesn't exist
    if not os.path.exists(LOCK_FILE):
        with open(LOCK_FILE, 'w') as f:
            pass

    if not os.path.exists(STATE_FILE):
        return {}

    try:
        with open(LOCK_FILE, 'r') as lockf:
            fcntl.flock(lockf, fcntl.LOCK_SH)
            try:
                # Decrypt using openssl
                cmd = [
                    'openssl', 'enc', '-d', '-aes-256-cbc', '-pbkdf2',
                    '-pass', 'env:BG_PASSWORD',
                    '-in', STATE_FILE
                ]
                result = subprocess.run(cmd, check=True, capture_output=True)
                return json.loads(result.stdout)
            finally:
                fcntl.flock(lockf, fcntl.LOCK_UN)
    except subprocess.CalledProcessError as e:
        print(f"Error decrypting state (wrong password?): {e.stderr.decode()}", file=sys.stderr)
        sys.exit(1)
    except (json.JSONDecodeError, IOError) as e:
        print(f"Error reading state: {e}", file=sys.stderr)
        return {}

def save_state(state):
    get_password() # Ensure password is set
    
    # Create lock file if it doesn't exist
    if not os.path.exists(LOCK_FILE):
        with open(LOCK_FILE, 'w') as f:
            pass

    try:
        with open(LOCK_FILE, 'w') as lockf:
            fcntl.flock(lockf, fcntl.LOCK_EX)
            try:
                json_bytes = json.dumps(state, indent=4).encode('utf-8')
                cmd = [
                    'openssl', 'enc', '-e', '-aes-256-cbc', '-pbkdf2',
                    '-pass', 'env:BG_PASSWORD',
                    '-out', STATE_FILE
                ]
                subprocess.run(cmd, input=json_bytes, check=True, capture_output=True)
            finally:
                fcntl.flock(lockf, fcntl.LOCK_UN)
    except IOError as e:
        print(f"Error saving state: {e}")
    except subprocess.CalledProcessError as e:
        print(f"Error encrypting state: {e.stderr.decode()}", file=sys.stderr)

