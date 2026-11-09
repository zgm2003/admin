package server

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type Collector struct{ startedAt time.Time }

func NewCollector(startedAt time.Time) *Collector { return &Collector{startedAt: startedAt} }

func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return Snapshot{}, fmt.Errorf("hostname: %w", err)
	}
	uptime, err := host.UptimeWithContext(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("host uptime: %w", err)
	}
	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("host memory: %w", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return Snapshot{}, fmt.Errorf("disk root: %w", err)
	}
	usage, err := disk.UsageWithContext(ctx, workingDirectory)
	if err != nil {
		return Snapshot{}, fmt.Errorf("host disk: %w", err)
	}
	percent, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false)
	if err != nil {
		return Snapshot{}, fmt.Errorf("host CPU: %w", err)
	}
	if len(percent) != 1 || memory.Total == 0 || usage.Total == 0 || memory.Available > memory.Total || usage.Free > usage.Total {
		return Snapshot{}, fmt.Errorf("incomplete host metrics")
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return Snapshot{
		SampledAt: time.Now().UTC(), Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH,
		LogicalCores: runtime.NumCPU(), CPUPercent: percent[0],
		Memory: Capacity{TotalBytes: memory.Total, AvailableBytes: memory.Available, UsedPercent: float64(memory.Total-memory.Available) / float64(memory.Total) * 100},
		Disk:   Capacity{TotalBytes: usage.Total, AvailableBytes: usage.Free, UsedPercent: usage.UsedPercent}, DiskPath: workingDirectory,
		HostUptimeSeconds: uptime, ProcessUptimeSeconds: uint64(max(0, time.Since(c.startedAt).Seconds())), StartedAt: c.startedAt.UTC(),
		GoVersion: runtime.Version(), Goroutines: runtime.NumGoroutine(), HeapAllocBytes: stats.HeapAlloc,
	}, nil
}
