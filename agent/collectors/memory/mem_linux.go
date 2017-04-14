package memory

import (
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/meminfo")
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	lines := strings.Split(string(data), "\n")
	headers := make([]string, 0)
	metrics := make([]string, 0)
	metadataProto := map[string]string{
		"memtotal":          "int Total amount of physical RAM, in kilobytes.",
		"memfree":           "int The amount of physical RAM, in kilobytes, left unused by the system.",
		"buffers":           "int The amount of physical RAM, in kilobytes, used for file buffers.",
		"cached":            "int The amount of physical RAM, in kilobytes, used as cache memory.",
		"swapcached":        "int The amount of swap, in kilobytes, used as cache memory.",
		"active":            "int The total amount of buffer or page cache memory, in kilobytes, that is in active use. This is memory that has been recently used and is usually not reclaimed for other purposes.",
		"inactive":          "int The total amount of buffer or page cache memory, in kilobytes, that are free and available. This is memory that has not been recently used and can be reclaimed for other purposes.",
		"highTotal":         "int The total amount of memory, in kilobytes, that is not directly mapped into kernel space. The HighTotal value can vary based on the type of kernel ",
		"highfree":          "int The free amount of memory, in kilobytes, that is not directly mapped into kernel space.",
		"lowtotal":          "int The total amount of memory, in kilobytes, that is directly mapped into kernel space. The LowTotal value can vary based on the type of kernel used. ",
		"lowfree":           "int The free amount of memory, in kilobytes, that is directly mapped into kernel space.",
		"swaptotal":         "int The total amount of swap available, in kilobytes.",
		"swapfree":          "int The total amount of swap free, in kilobytes.",
		"dirty":             "int The total amount of memory, in kilobytes, waiting to be written back to the disk.",
		"writeback":         "int The total amount of memory, in kilobytes, actively being written back to the disk.",
		"mapped":            "int The total amount of memory, in kilobytes, which have been used to map devices, files, or libraries using the mmap command.",
		"slab":              "int The total amount of memory, in kilobytes, used by the kernel to cache data structures for its own use.",
		"committed_as":      "int The total amount of memory, in kilobytes, estimated to complete the workload. This value represents the worst case scenario value, and also includes swap memory.",
		"pagetables":        "int The total amount of memory, in kilobytes, dedicated to the lowest page table level.",
		"vmalloctotal":      "int The total amount of memory, in kilobytes, of total allocated virtual address space.",
		"vmallocused":       "int The total amount of memory, in kilobytes, of used virtual address space.",
		"vmallocchunk":      "int The largest contiguous block of memory, in kilobytes, of available virtual address space.",
		"hugepages_total":   "int The total number of hugepages for the system. The number is derived by dividing Hugepagesize by the megabytes set aside for hugepages specified in /proc/sys/vm/hugetlb_pool. This statistic only appears on the x86, Itanium, and AMD64 architectures.",
		"hugepages_free":    "int The total number of hugepages available for the system. This statistic only appears on the x86, Itanium, and AMD64 architectures.",
		"hugepagesize":      "int The size for each hugepages unit in kilobytes. By default, the value is 4096 KB on uniprocessor kernels for 32 bit architectures. For SMP, hugemem kernels, and AMD64, the default is 2048 KB. For Itanium architectures, the default is 262144 KB. This statistic only appears on the x86, Itanium, and aMD64 architectures.",
		"hugepages_rsvd":    "int The number of unused huge pages reserved for hugetlbfs.",
		"hugepages_surp":    "int The number of surplus huge pages.",
		"commitlimit":       "int This is the total amount of memory in kilobytes currently available to be allocated on the system.",
		"kernelstack":       "int The amount of memory, in kibibytes, used by the kernel stack allocations done for each task in the system.",
		"directmap4k":       "int The amount of memory, in kibibytes, mapped into kernel address space with 4 kB page mappings.",
		"directmap2m":       "int The amount of memory, in kibibytes, mapped into kernel address space with 2 MB page mappings.",
		"anonpages":         "int The total amount of memory, in kibibytes, used by pages that are not backed by files and are mapped into userspace page tables.",
		"mlocked":           "int The total amount of memory, in kibibytes, that is not evictable because it is locked into memory by user programs.",
		"shmem":             "int The total amount of memory, in kibibytes, used by shared memory (shmem) and tmpfs.",
		"active(anon)":      "int The amount of anonymous and tmpfs/shmem memory, in kibibytes, that is in active use, or was in active use since the last time the system moved something to swap.",
		"inactive(anon)":    "int The amount of anonymous and tmpfs/shmem memory, in kibibytes, that is a candidate for eviction.",
		"active(file)":      "int The amount of file cache memory, in kibibytes, that is in active use, or was in active use since the last time the system reclaimed memory.",
		"inactive(file)":    "int The amount of file cache memory, in kibibytes, that is newly loaded from the disk, or is a candidate for reclaiming.",
		"unevictable":       "int The amount of memory, in kibibytes, discovered by the pageout code, that is not evictable because it is locked into memory by user programs.",
		"nfs_unstable":      "int The amount, in kibibytes, of NFS pages sent to the server but not yet committed to the stable storage.",
		"bounce":            "int The amount of memory, in kibibytes, used for the block device bounce buffers.",
		"anonhugepages":     "int The total amount of memory, in kibibytes, used by huge pages that are not backed by files and are mapped into userspace page tables.",
		"hardwarecorrupted": "int The amount of memory, in kibibytes, with physical memory corruption problems, identified by the hardware and set aside by the kernel so it does not get used.",
		"writebacktmp":      "int The amount of memory, in kibibytes, used by FUSE for temporary writeback buffers.",
		"sreclaimable":      "int The part of Slab that can be reclaimed, such as caches.",
		"sunreclaim":        "int The part of Slab that cannot be reclaimed even when lacking memory.",
		"memavailable":      "int An estimate of how much memory is available for starting new applications, without swapping.",
	}
	metadata := map[string]string{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		header := strings.TrimSpace(parts[0])
		headers = append(headers, header)
		metrics = append(metrics, strings.Fields(strings.TrimSpace(parts[1]))[0])
		if v, ok := metadataProto[strings.ToLower(header)]; ok {
			metadata[header] = v
		}
	}

	result := collectors.BuildMetricResult(strings.Join(headers, " "), strings.Join(metrics, " "), "", metadata)
	return []*collectors.MetricResult{result}, nil
}
