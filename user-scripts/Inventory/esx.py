#!/usr/bin/env python3
import subprocess
import os
import os.path
import json

#Key value class
class Details:
    def __init__(self,line):
        v = line.split(":", 2)
        self.Tag = v[0].strip()
        self.Value = v[1].strip()

#Entry class container for data with some category
class Entry:
    def __init__(self,category):
        self.Category = category
        self.Details = []

#GenericInfo class container for collected data
class GenericInfo:
    def __init__(self):
        self.Entries = []

    def toJSON(self):
        return json.dumps(self, default=lambda o: o.__dict__, sort_keys=True)

def esxCall(configFile, section, cmd, subcmd):
    p = subprocess.Popen('esxcli -c '+ configFile+' '+ section + ' '+ cmd +' '+ subcmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)        
    p.wait()
    out, err = p.communicate()
     
    if p.returncode: 
        return "" # mean error
    return out.decode("utf-8") 



def handleESXResult(categoryName, data):
    c = Entry(categoryName)
    result = []
    lines = data.splitlines()
    for l in lines:
        if l.startswith("  "):
            d = Details(l)
            c.Details.append(d)
        else:
            if len(c.Details) > 0:
                result.append(c)
            c = Entry(categoryName)

    if len(c.Details) > 0:
        result.append(c)

    return result

def handleESXPackages(data):
    fileds = ["Name", "Version", "Vendor", "Acceptance-Level", "Install-Date"]
    result = []
    lines = data.splitlines()
    if len(lines) <= 2:
        return result
    lines = lines[2:]
    for line in lines:
        words = line.split()
        if len(words) != len(fileds):
            continue

        entry = Entry(words[0])
        for i in range(len(words)):
            if i == 0:
                continue #skip name

            entry.Details.append(Details(fileds[i]+":"+words[i]))
        result.append(entry)

    return result

CONFIG_FILE = os.path.join(os.path.dirname(__file__), "esx.config")

if not os.path.exists(CONFIG_FILE):
    quit() #miss config file

GenInfo = GenericInfo()

#get process information
res = esxCall(CONFIG_FILE, "vm", "process", "list")
if len(res) > 0:
    GenInfo.Entries.append(handleESXResult("VMs", res))

#get cpu information
res = esxCall(CONFIG_FILE, "hardware", "cpu", "list")
if len(res) > 0:
    GenInfo.Entries.append(handleESXResult("CPUs", res))

#get memory information
res = esxCall(CONFIG_FILE,  "hardware", "memory", "get")
if len(res) > 0:
    GenInfo.Entries.append(handleESXResult("Memory", res))

#get pci information
res = esxCall(CONFIG_FILE,  "hardware", "pci", "list")
if len(res) > 0:
    GenInfo.Entries.append(handleESXResult("PCI", res))

#get UUID of system
res = esxCall(CONFIG_FILE,  "system", "uuid", "get")
if len(res) > 0:
    vnUUID = Entry("VM-UUID")
    vnUUID.Details.append(Details("UUID:"+ res))
    GenInfo.Entries.append(vnUUID)

#get system version
res = esxCall(CONFIG_FILE, "system", "version", "get")
if len(res) > 0:
    GenInfo.Entries.append(handleESXResult("Version", res)[0])

#get software information
res = esxCall(CONFIG_FILE, "software", "vib", "list")
if len(res) > 0:
    GenInfo.Entries.append(handleESXPackages(res))

print(GenInfo.toJSON())
