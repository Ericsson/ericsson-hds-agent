## Python simple server

This example server receives hds-agent data and writes it out in the current running folders.  The file structure in this simple example is 
as below:

```
├── cpu-<ip:port>.metric
└── inventory-<ip:port>.all
```

The cpu file is the metric data and the inventory file is the inventory data that were collected from the Ericsson HDS Agent.


### Steps:

Run the example server

```
python simple-storage-server.py
```

Run hds-agent on the same host

```
sudo ./hds-agent -destination=tcp:<ip_addr>:9090 -frequency=15
```

ip_addr= ip address of the host machine where the simple-storage-server runs.



