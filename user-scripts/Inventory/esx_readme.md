`esx.py` is a python script which collects VMware information of ESXi servers.

## Prerequisite

You should have ESXi and python3 installed.

To use this collector, a config file named `esx.config` is required. The file needs to define the ESXi host(using either the hostame or IP address) and login credentials. 

Here is a sample `esx.config` file:

```
VI_SERVER=172.x.y.z
VI_USERNAME=username
VI_PASSWORD=password
VI_THUMBPRINT=4A:2F:DD:13:91:55:39:00:8A:50:1D:2C:78:2F:12:70:22:F9:89:B0
```

The config file needs to be placed in the same folder as the `esx.py` user script.

## Inventory Example

In the agent's working directory, create sub-directory `Inventory`.

Place `esx.py` and `esx.config` files in that sub-directory.

Next run the agent and the user script will be automatically discovered and executed:

```
sudo ./ericsson-hds-agent
```

The result will be in 3-tiered json format. It is recommended that you use a dynamic language to parse the result.

A sample result looks like this:

```
{"Type":"inventory.user","ID":0,"Digest":"1d500376a32d5a22b130c54bef798e42101d8a72","NodeID":"ad98bd4ca430d9a74591b84e93c681e3","Timestamp":"1492115882","Content":{"esx":"{\"Entries\": [[{\"Category\": \"VMs\", \"Details\": [{\"Tag\": \"World ID\", \"Value\": \"38218\"}, {\"Tag\": \"Process ID\", \"Value\": \"0\"}, {\"Tag\": \"VMX Cartel ID\", \"Value\": \"38217\"},... 
```

## Customization

Users can add more ESX commands to the python script. (For additional commands, see [vSphere document](https://pubs.vmware.com/vsphere-50/index.jsp#com.vmware.vcli.ref.doc_50/vcli-right.html))

Add extra commands to the lower section of the script such as:

```
# listing the pci information
res = esxCall(CONFIG_FILE,  "hardware", "pci", "list")
if len(res) > 0:
    """
    Some extra code dealing with the return result
    """

    #store the result in final output
    GenInfo.Entries.append(handleESXResult("PCI", res))
```
