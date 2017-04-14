User Scripts 
===============

Ericsson HDS Agent has a built-in feature that allows users to add their own collectors via user scripts, thus making this agent very flexible.

User scripts fall into two categories: Inventory or Metrics. Inventory user scripts collect the system's hardware configuration. Metrics user scripts collect the system's dynamic data.

The steps are:

  1. In the agent's working directory, create `Inventory` and `Metrics` sub-directories. 

  2. Place the user script in either `Inventory` or `Metrics` sub-directory. 

  3. Make sure the scripts have executable permission. 
  
  4. Run ericsson-hds-agent as normal. It will look for scripts inside `Inventory` and `Metrics` sub-directories to execute.

If scripts are added while ericsson-hds-agent is already running, they will not be discovered until the agent is restarted. If a script is removed as the agent is running, error messages will be displayed until the agent is restarted.

The user scripts are executed at the same frequency as built-in collectors.


### Metrics Example

Place the following `process_count.sh` sample user script in the `Metrics` sub-directory. Set its permission to be executable.

```
#!/bin/bash
#This is a very simple user-script to collect number of running processes 

echo "Number_of_running_processes"
ps -aux | wc -l
```

Next run the agent:
```
./ericsson-hds-agent
```

The metric output from ericsson-hds-agent looks like this:
```
:=:header user.process_count d2de10c785862d9ade8f92ad086594b2 0 #timestamp Number_of_running_process
:=:user.process_count d2de10c785862d9ade8f92ad086594b2 0 1491592824 194 
```

The header line uses the script's name as the collector name(ie. `user.`_your-script_). The file extension is omitted from the collector's name. The collector name is followed by the agent's node.id, frequency of collection, timestamp at collection time, and column name(s) as defined in the user script.

### Inventory Example

See a sample ESX collector in this repo.

