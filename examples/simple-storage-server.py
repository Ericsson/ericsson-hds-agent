import json
import os
import socket
import sys

HOST = ''   # Symbolic name meaning all available interfaces
PORT = 9090 # Arbitrary non-privileged port
BUFF_SIZE = 1024 #Input buffer for ingesting HDS Agent, can adjust if needed.
 
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

# reuse existing socket
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
 
#Bind socket to local host and port
try:
    s.bind((HOST, PORT))
except socket.error as msg:
    print('Bind failed. Error Code : ' + str(msg[0]) + ' Message ' + msg[1])
    sys.exit()
     
#Start listening on socket
s.listen(10)

announce = '\nServer waiting to receive Ericsson HDS Agent data on port ' + str(PORT) +'(Ctrl-C to quit):\n'
print(announce)


def get_json(str):
    try:
        return json.loads(str)
    except ValueError as e:
        return None


def write_file(filename, content):
    if os.path.exists(filename) and filename.endswith('.metric'):
        mode = 'a'
    else:
        mode = 'w'
    with open(filename, mode) as text_file:
        text_file.write(content)


def write_buffer_to_file(data_buffer, write_part_chunk):
    lines = []
    income_lines = data_buffer.split("\n")
    for (i, line) in enumerate(income_lines):
        if not write_part_chunk and i == len(income_lines) - 1 and line != '':
            lines.append(line)
            continue
        if line.startswith('!'):
            continue
        if line.startswith(':=:'):
            words = line.split()
            if len(words) > 3 and ( words[0] == ":=:header" or words[0] == ":=:metadata"):
                if words[1] == "cpu":
                    write_file(words[1]+"-"+ip_port+".metric", line + '\n')
            elif len(words) > 2:
                if len(words[0]) > 3 and words[0][3:] == "cpu":
                    write_file( words[0][3:]+"-"+ip_port+".metric", line + '\n')
            continue
        obj = get_json(line)
        if obj is not None:
            if obj['Type'] == "inventory.all":
                write_file("inventory-"+ip_port+".all", line)
            continue

        lines.append(line)
    return '\n'.join(lines)


# Keep talking with the client
while 1:
    try:
        # Wait to accept a connection - blocking call
        conn, addr = s.accept()
        ip_port = addr[0] + ':' + str(addr[1])
        print('Connected with ' + ip_port)
         
        # Infinite loop so that function does not terminate and thread does not end.
        data_buffer = ""
        while True:
            #Receiving from client
            chunk = conn.recv(BUFF_SIZE)
            #Check the data if the sending side stops
            if len(chunk) is 0:
                write_buffer_to_file(data_buffer, True)
                break
            decoded = chunk.decode()
            print(decoded)
            data_buffer += decoded
            data_buffer = write_buffer_to_file(data_buffer, False)

    # Handle exiting via control-C
    except KeyboardInterrupt:
        print('\nStopping server')
        try:
            sys.exit(0)
        except SystemExit:
            os._exit(0)
     
# Free up the connection
conn.close()

# Free up the port
s.close()
