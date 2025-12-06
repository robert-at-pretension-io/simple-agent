import os
import sys
import pty
import select
import subprocess
import time
import signal

def main():
    # Args: command, log_file, input_fifo
    if len(sys.argv) < 4:
        sys.exit(1)
        
    command = sys.argv[1]
    log_file = sys.argv[2]
    input_fifo = sys.argv[3]
    
    # 1. Setup Logging Subprocess (OpenSSL)
    # We rely on inherited env BG_PASSWORD
    openssl_cmd = [
        'openssl', 'enc', '-e', '-aes-256-cbc', '-pbkdf2', 
        '-pass', 'env:BG_PASSWORD', '-out', log_file
    ]
    # Buffering=0 for immediate write to the pipe
    logger_proc = subprocess.Popen(openssl_cmd, stdin=subprocess.PIPE, bufsize=0)
    
    # 2. Setup Input FIFO
    if not os.path.exists(input_fifo):
        os.mkfifo(input_fifo)
    
    # Open RW to avoid EOF when writer closes. 
    # O_NONBLOCK prevents blocking on open, though O_RDWR usually doesn't block.
    fifo_fd = os.open(input_fifo, os.O_RDWR | os.O_NONBLOCK)
    
    # 3. Fork PTY
    # pty.fork returns (pid, fd). In child, pid=0. In parent, pid=child_pid, fd=master_fd.
    pid, master_fd = pty.fork()
    
    if pid == 0:
        # CHILD: Execute the command
        # We use sh -c to allow complex command strings
        sys.stdout.flush()
        # Restore signals if needed? Python usually handles this.
        os.execvp('sh', ['sh', '-c', command])
    else:
        # PARENT: Supervisor Loop
        try:
            while True:
                # Check if logger is still alive
                if logger_proc.poll() is not None:
                    break
                
                # Select on Master (output) and FIFO (input)
                rlist, _, _ = select.select([master_fd, fifo_fd], [], [], 0.5)
                
                if master_fd in rlist:
                    try:
                        data = os.read(master_fd, 4096)
                        if not data:
                            break # EOF
                        logger_proc.stdin.write(data)
                        logger_proc.stdin.flush()
                    except OSError:
                        break # Child closed
                
                if fifo_fd in rlist:
                    try:
                        data = os.read(fifo_fd, 4096)
                        if data:
                            os.write(master_fd, data)
                    except OSError:
                        pass 
                
                # Check child status
                wpid, status = os.waitpid(pid, os.WNOHANG)
                if wpid != 0:
                    break
                    
        except Exception as e:
            pass
        finally:
            if logger_proc.stdin: logger_proc.stdin.close()
            logger_proc.wait()
            try: os.close(fifo_fd)
            except: pass
            try: os.close(master_fd)
            except: pass

if __name__ == "__main__":
    main()