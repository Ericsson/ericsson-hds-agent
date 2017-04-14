import socket
import sys
import getopt
from elasticsearch import Elasticsearch


def get_node_id(data):
    for line in data.splitlines():
        words = line.split(" ")
        if ":=:cpu" in words[0]:
            print "nodeid :", words[1]
            return words[1]
    return ""


def create_index(elastic_client, index_name):
    print('Check if the index %s exists in the database' % (index_name))
    if elastic_client.indices.exists(index_name):
        print('The index exists in the elastic database')
    else:
        print('The index does not exist, creating the index: ' + index_name)
        request_body = {
            "settings": {
                "number_of_shards": 1,
                "number_of_replicas": 1
            },
            "mappings": {
                "_default_": {
                    "_all":       {"enabled": False},
                    "properties": {
                        "timestamp":    {"type": "date", "format": "epoch_second", "doc_values": True},
                        "cpu_usage":     {"type": "float", "doc_values": True, "index": "no"},
                        "process_change":      {"type": "float", "doc_values": True, "index": "no"},
                        "node_id":      {"type": "keyword", "doc_values": True}
                    }
                }
            }
        }
        elastic_client.indices.create(index=index_name, body=request_body)
        print('The index was created in the elastic database')


def cpu_data_clean(data):
    # raw_data is a list of cpu metric data from list 0 to 13 are :=:header cpu
    # node_id frequency #timestamp cpu.user cpu.nice
    # cpu.system cpu.idle cpu.iowait cpu.irq cpu.softirq cpu.steal cpu.guest
    # from -7 to end are intr ctxt btime processes procs_running procs_blocked
    # softirq
    cpu_info = map(int, data[3:13])
    non_idle = cpu_info[1] + cpu_info[2] + cpu_info[3] + \
        cpu_info[6] + cpu_info[7] + cpu_info[8] + cpu_info[9]
    idle = cpu_info[4] + cpu_info[5]
    process = data[-4]
    return non_idle, idle, int(process)


def send_data_elastic_server(elastic_client, data, index_name, node_id, prev_status):
    # Get data from agent
    # prev_status stores (cpu_nonIdle_time, cpu_idle_time,cpu_process)
    # data format is ":=:header cpu 75c3f9ae7c5204796c20ba65b5cb85a9 10
    # #timestamp cpu.user cpu.nice cpu.system cpu.idle cpu.iowait cpu.irq
    # cpu.softirq cpu.steal cpu.guest cpu*.user cpu*.nice cpu*.system
    # cpu*.idle cpu*.iowait cpu*.irq cpu*.softirq cpu*.steal cpu*.guest
    # intr ctxt btime processes procs_running procs_blocked softirq"

    non_idle, idle, process = 0, 0, 0
    for line in data.splitlines():
        if ":=:cpu" in line:
            words = line.split(" ")
            if len(words) < 20:
                return non_idle, idle, process
            elif prev_status == (0, 0, 0):
                print "Taking data for the first time"
                return cpu_data_clean(words)
            else:
                non_idle, idle, process = cpu_data_clean(words)
                break

    total = non_idle + idle
    prev_total = prev_status[0] + prev_status[1]
    usage = float(non_idle - prev_status[0]) / \
        float(total - prev_total) * 100.00
    process_change = process - prev_status[2]
    count = elastic_client.count(index=index_name)
    max_doc_id = count['count']
    max_doc_id += 1

    print('Adding data with the document id:', max_doc_id)
    request_body = {
        "timestamp": words[3],
        "cpu_usage": usage,
        "process_change": process_change,
        "node_id": node_id
    }
    result = elastic_client.create(
        index=index_name, doc_type="cpu", id=max_doc_id, body=request_body,refresh='true')
    print('Result is:', result)
    return non_idle, idle, process


def main(argv):

    es_host = 'http://localhost:9200'
    host = ''
    port = 9090
    node_id = ""
    try:
        opts, args = getopt.getopt(
            argv[1:], "he:", ["help", "es=", "port=", "host="])

    except getopt.GetoptError:
        print argv[0], '-e <elasticsearch server> or --es=<elasticsearch server> --host=<host address> --port=<port>'
        print "default port: 9090 default host: \"\""
        sys.exit(2)
    for opt, arg in opts:
        if opt == '-h':
            print argv[0], '-e <elasticsearch server> or --es=<elasticsearch server>'
            print 'default elasticsearch server is localhost:9200'
            print '--host=<host address> --port=<port>'
            print "default port: 9090"
            print "default host: \"\""
            sys.exit()
        elif opt in ("-e", "--es"):
            es_host = arg
            print 'Elasticsearch server is ', es_host
        elif opt in ("--port"):
            port = int(arg)
            print "port is ", port
        elif opt in ("host"):
            host = arg
            print "host is ", host
    # create ES client
    es = Elasticsearch(hosts=[es_host], timeout=60)

    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        s.bind((host, port))
    except socket.error as msg:
        print('Bind failed. Error Code : ' +
              str(msg[0]) + ' Message ' + msg[1])
        sys.exit()
    s.listen(10)

    announce = '\nHDSagent Elastic Search forwarding server waiting to receive Ericsson HDS Agent data on port ' + \
        str(port) + '(Ctrl-C to quit):\n'
    print(announce)
    while True:
        try:
            conn, addr = s.accept()
            print('Connected with ' + addr[0] + ':' + str(addr[1]))
            prev_status = (0, 0, 0)
            while True:
                # based on different machine, agent would send out different amount of data
                # change the buffer size for recv(buff) or using some algorithm for different 
                # usage
                data = conn.recv(8192)
                if len(data) > 0:
                    node_id = get_node_id(data)
                    if node_id == "":
                        continue
                    index_name = 'hdsagent-' + "cpu" + '-' + node_id
                    create_index(es, index_name)
                    # Send data to elastic server
                    # prev_status stores (cpu_nonIdle_time,
                    # cpu_idle_time,cpu_process)

                    prev_status = send_data_elastic_server(
                        es, data, index_name, node_id, prev_status)
                else:
                    print('No data collected, cannot create index')
                    conn.close()
                    break

        except KeyboardInterrupt:
            s.close()
            print('\nStopping server')
            sys.exit(0)


if __name__ == "__main__":
    main(sys.argv)
