Ericsson HDS Agent
===========================

**ericsson-hds-agent** is a binary built to run on Linux systems. It collects an inventory of the host machine's hardware and metrics. It comes with a set of built-in collectors and has the ability to run user-provided scripts.

Running the Agent
-----------------

The agent can be run with the command `./ericsson-hds-agent`. It collects inventory and metrics data once. It also automatically generates a unique `node.id` file to identify this agent. The `node.id` file is located in the agent's working directory. See the following section for additional optional command-line flags.

### Command-Line Flags and Arguments

- **`-h`**

  Displays usage information. A quick way to see a list of the valid command-line flags and arguments.

- **`-chdir`**  _directory-path_

  Change the working directory to _directory-path_ (default is ".", the current working directory). 

- **`-collection-timeout`** _timeout-in-seconds_

  Specify collection timeout in seconds (default is 30)

- **`-destination`** _output-destination_
 
  Specify where to send the output to remotely. Valid destinations are in the form _tcp:host:port_ (default is null.) 

- **`-dry-run`**

  Check the system environment settings for running collectors. Use this flag to identify any additional Linux packages that may be missing for running the collectors.

- **`-duration`** _time-in-seconds_

  The number of seconds to run the agent for. 0 means non-stop

- **`-frequency`** _metric-collection-interval_
  
  Time in seconds between subsequent runs of metric collectors. When frequency is greater than 0, inventory and metric data is collected at successive intervals. Inventory data is collected every 30 minutes and only reported if it has changed during that interval. Metrics are collected at the provided interval and are always reported. User-provided inventory and metric scripts run at the same frequency as their built-in counterparts. For frequency values of 0 or less, the collectors will be run only once.

- **`-retrywait`** _time-in-seconds_

  Wait time in seconds before trying to reconnect to destination (default is 10s)

- **`-skip`** _collector_name(s)_

  A comma-separated list of collectors to skip. These collectors may be skipped:

  - cpu
  - disk
  - diskusage
  - load
  - memory
  - net
  - uptime
  - sensor
  - smart
  - sysinfo.bmc.bmc-info
  - sysinfo.bmc.ipmi-tool
  - sysinfo.disk
  - sysinfo.ecc
  - sysinfo.nic
  - sysinfo.package.dpkg-package
  - sysinfo.package.rpm-package
  - sysinfo.pci
  - sysinfo.proc
  - sysinfo.smbios
  - sysinfo.usb

- **`-stdout`**

  Toggles sending output to stdout. Default is `false`

### Example
To collect inventory and metrics every 30 seconds and send data to a server located at 192.0.2.0 at port 9090, run the following command:

```
sudo ./ericsson-hds-agent -destination tcp:192.0.2.0:9090 -frequency 30
```

Collection Tools
----------------

The agent uses the following Linux commands to collect inventory data from the host machine:

 - `bmc-info` or `ipmitool`
 - `dmidecode`
 - `dpkg` or `rpm`
 - `ethtool`
 - `ip`
 - `lspci`
 - `lsusb`
 - `smartctl`

These tools require `sudo` to run. If a tool is not installed on the host machine, ericsson-hds-agent skips the collection of data from that tool and moves on. It is recommended that the host machine install the above list of tools to collect the most amount of data.


User Scripts
------------
See the users-scripts folder for more detail.

