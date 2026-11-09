package server

import "time"

type Capacity struct {
	TotalBytes     uint64  `json:"totalBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsedPercent    float64 `json:"usedPercent"`
}

// Snapshot describes the responding API instance and its OS-visible host.
// It excludes environment, credentials and network addresses.
type Snapshot struct {
	SampledAt            time.Time `json:"sampledAt"`
	Hostname             string    `json:"hostname"`
	OS                   string    `json:"os"`
	Arch                 string    `json:"arch"`
	LogicalCores         int       `json:"logicalCores"`
	CPUPercent           float64   `json:"cpuPercent"`
	Memory               Capacity  `json:"memory"`
	Disk                 Capacity  `json:"disk"`
	DiskPath             string    `json:"diskPath"`
	HostUptimeSeconds    uint64    `json:"hostUptimeSeconds"`
	ProcessUptimeSeconds uint64    `json:"processUptimeSeconds"`
	StartedAt            time.Time `json:"startedAt"`
	GoVersion            string    `json:"goVersion"`
	Goroutines           int       `json:"goroutines"`
	HeapAllocBytes       uint64    `json:"heapAllocBytes"`
}
