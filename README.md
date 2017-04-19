Ericsson HDS Agent
==================

What is Ericsson HDS Agent?
--------------------------
Ericsson HDS (Hyperscale Datacenter Systems) Agent is a Linux based program designed to run on any Linux System. It collects an inventory of the host machine's hardware inventory and runtime system metrics. It is composed of many built-in collectors, formatters and a forwarder.

![image](./docs/images/datahub-io.png) 

### Supported Platforms:
   * LINUX/x86_64
   * LINUX/ARM64

### Recommended System Tools  
The agent uses various Linux commands to collect inventory data from the host machine.  Please refer to the documentation in docs folder for more details.



Getting Started
---------------

Installation from Binary Distribution
-------------------------------------
1. **Download**
   
   Please go to the release folder to download the binary code.

1. **Install**

   Extract the downloaded file to a directory from where the user wants to run. 
   ```
   tar -zxvf ericsson-hds-agent_x86_64.tar.gz
   ```

1. **Run**

   After the files are extracted, the program can be executed as follows: 
   ```
   cd ericsson-hds-agent/

   sudo ./ericsson-hds-agent -stdout
   ```
   A `node.id` file is automatically created and contains an unique identifier for this host system.

   The above command runs the inventory and metrics collectors once and sends the collected data to standard output.

   This behavior can be changed by passing different sets of command line arguments. Here are some examples.
 
   ```
   sudo ./ericsson-hds-agent -stdout -frequency=15
   ```
   The above command collects metrics at an interval of 15 seconds and sends the data to standard output.
  
   ```
   sudo ./ericsson-hds-agent  -frequency=15 -destination=tcp:<ip_addr>:9090
   ```
   (ip_addr = ip address of the storage server.)

   The above command collects metrics at an interval of 15 seconds and sends the data to a storage server running on port 9090. A [sample storage server](./examples/simple-storage-server.py) has been provided to ingest the data.

### Command Line Arguments and Other details  
Please refer to [detailed document](./docs/agent-overview.md#running-the-agent) for a complete list of supported parameters and other details of the Ericsson HDS Agent.


Installation from Source Code
-----------------------------

### Prerequisite:
   * **git**
   * **go 1.6**

   Follow these installation and setup instruction links [**`git`**](https://www.digitalocean.com/community/tutorials/how-to-install-git-on-ubuntu-14-04) and [**`go 1.6`**](https://golang.org/doc/install) if you neeed to install.

1. **How to Download**

   User can download the Ericsson HDS Agent source code using `go` command

   ```
   mkdir -p $HOME/go/src

   export GOPATH=$HOME/go

   cd $GOPATH/src

   go get github.com/Ericsson/ericsson-hds-agent
   ```

1. **How to Build**

   Execute the following set of commands to build HDS Agent binary:
   ```
   cd $GOPATH/src/github.com/Ericsson/ericsson-hds-agent/agent/apps/ericsson-hds-agent

   go get ./...

   go build
   ```

1. **How to Run**

   After the build completes, follow this instruction to execute the Ericsson HDS Agent program: 
   ```
   sudo ./ericsson-hds-agent -stdout
   ```
   
   After the agent is run, a `node.id` file is automatically created. This file contains an unique identifier for this agent.
   To run with other parameters refer to section [Running the agent](./docs/agent-overview.md#running-the-agent)

1. **How to Validate Data Being Collected**

   User can validate Ericsson HDS Agent output with system command(s)


   For example, cpu inventory data output from Ericsson HDS Agent(shown below) can be verified using Linux command `cat /proc/cpuinfo`.
   
   ```
	{
	    "Category": "cpuinfo",
	    "Details": [
		{
		    "Tag": "processor",
		    "Value": "0"
		},
		{
		    "Tag": "vendor_id",
		    "Value": "GenuineIntel"
		},
		{
		    "Tag": "cpu family",
		    "Value": "6"
		},
		{
		    "Tag": "model",
		    "Value": "70"
		},
		{
		    "Tag": "model name",
		    "Value": "Intel(R) Core(TM) i7-4870HQ CPU @ 2.50GHz"
		},
		...
	    ]
	}
   ```

1. **How to Use the Data**  
In this example, the inventory and metrics data from the Ericsson HDS Agent can be viewed on a dashboard to analyze the inventory of the data center and how each machine is operating. 

![image](https://cloud.githubusercontent.com/assets/10677356/23774990/4536b29a-04db-11e7-89f8-4bbe8006d720.png)
![image](https://cloud.githubusercontent.com/assets/10677356/23774989/45343d44-04db-11e7-9814-016969749479.png)
