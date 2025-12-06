import argparse
import os
import sys

# Add current dir to sys.path to import manager_utils
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from manager_utils import load_state, get_password

def main():
    # Ensure password is present
    get_password()

    parser = argparse.ArgumentParser(description="Send input to a background process")
    parser.add_argument("id", help="Process ID (or partial ID)")
    parser.add_argument("text", help="Text to send")
    parser.add_argument("-n", "--no-newline", action="store_true", help="Do not append newline")
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
    input_file = info.get('input_file')
    
    if not input_file or not os.path.exists(input_file):
        print(f"Input interface not found for process {process_id}")
        sys.exit(1)
        
    # Write to FIFO
    text_to_send = args.text + ("" if args.no_newline else "\n")
    try:
        # Open non-blocking to avoid hanging if no reader (process dead)
        fd = os.open(input_file, os.O_WRONLY | os.O_NONBLOCK)
        with os.fdopen(fd, "w") as f:
            f.write(text_to_send)
            print(f"Sent input to {process_id}")
    except OSError as e:
        if e.errno == 6: # ENXIO: No such device or address (no reader)
            print(f"Error: Process {process_id} is not listening (it may be stopped or crashed).")
            sys.exit(1)
        else:
            raise e

if __name__ == "__main__":
    main()