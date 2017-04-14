The python script `cpu_to_es_example.py` shows how one can use the data collected by `ericsson-hds-agent` to forward to Elasticsearch, then visualized with either Kibana or Grafana.

In the process, the script selects and computes CPU utilization and process number change from the raw data before forwarding. 

## Prerequisite

- This script runs with Python2.7.

- The python library for Elasticsearch must be installed. 

  See [elasticsearch-py](https://github.com/elastic/elasticsearch-py) for instructions.

- ericsson-hds-agent binary(ie. running at IP address 172.x.y.a)

- A running Elasticsearch cluster(ie. at address 172.x.y.b:9200)

- A running Kibana configured to be able to query results from Elasticsearch installed above (ie. at address 172.x.y.c:5601)

- A running Grafana(ie. at address 172.x.y.d:3000)

## Work Flow

```
| ericsson-hds-agent | ---> | cpu_to_es_example.py| --->| Elasticsearch |--|
                                                                           |--> Kibana
                                                                           |--> Grafana
```
## Commands

On 172.x.y.a run the python script: 

```
python cpu_to_es_example.py -h
```

to see available flags and default options.

```
-e <elasticsearch server> or --es=<elasticsearch server>
default elastic search address http://localhost:9200

 --host=<host address> --port=<port>
default port: 9090
default host: localhost
```

To start the script, run as follows:

```
python cpu_to_es_example.py --es=172.x.y.b:9200
```

Then start ericsson-hds-agent with the frequency of collection at every 10 seconds and send the output to the python server. The server will select and index data to forward to Elasticsearch
```
 ./ericsson-hds-agent -frequency=10 -destination=tcp:172.x.y.a:9090
```

You should see output like:
```
Check if the index hdsagent-cpu-75c3f9ae7c5204796c20ba65b5cb85a9 exists in the database
The index exists in the elastic database
('Adding data with the document id:', 192)
('Result is:', {u'_type': u'cpu', u'_shards': {u'successful': 1, u'failed': 0, u'total': 2}, u'_index': u'hdsagent-cpu-75c3f9ae7c5204796c20ba65b5cb85a9', u'_version': 1, u'created': True, u'result': u'created', u'_id': u'75c3f9ae7c5204796c20ba65b5cb85a9_192'})
```

## Kibana

We can open Kibana in a browser. Under discover section, type in 
```
node_id:75c3f9ae7c5204796c20ba65b5cb85a9
```
We got this node Id from outputs above, or we can check the node id from ericsson-hds-agent folder


You will see all the documents indexed recently based on the time range selected.

![Kibana Discover](./example_image/Kibana%20Discover.png)

Under Management, click on Index Patterns. 

Under Index name or pattern, put in
```
hdsagent-*
```
Under Time-field name select timestamp then click create. You will see a new pattern created on the side.

Now click on visualize, add new line chart. Click on the ``hdsagent-*`` pattern. Click on the arrow next to Y-Axis. Under the Aggregation, pick Average and pick process_change or cpu_usage for Field. For buckets pick x-Axis and pick Data Histogram for aggregation and timestamp for Field. It is better to pick minute for the interval part according to the frequency of hds-agent setting. Then click on the start button, a new graph will be created. 

![Kibana Visualize](./example_image/Kibana%20Visualize.png)

![Kibana Dashboard](./example_image/Kibana%20Dashboard%20example.png)

## Grafana

### Data Source

Open Grafana in a browser. 

Click the Grafana logo and click on Data Sources. 

Click on add data source. Under config, give a name to the new source. 

Type is Elastic search, Url is 172.x.y.b:9200. Under Elasticsearch details, index name is 
```
hdsagent-*
```
and Time field name is timestamp. Version is 5.x. Then Save & Test

### Dashboard

Click on Grafana logo again and go to Dashboards and add new. 

Click on graph in the new panel. 

Click on panel Title and click edit. 

Under Metrics, click on A, you will see the setting on Query, Metric and Group by.

Put in 
```
node_id: 75c3f9ae7c5204796c20ba65b5cb85a9
```
for the Query.(the actual node id can be obtained from the output)

For metric, select Average and cpu_usage. For Group by, select Date Histogram and timestamp, interval to be 10s.

Panel data source is the data source you just added. Then you will see the graph. 

![Grafana Setting](./example_image/Grafana%20Graph%20Setting.png)

![Grafana Example](./example_image/Grafana%20example.png)
