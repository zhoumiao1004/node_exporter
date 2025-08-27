// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nosystemdstats
// +build !nosystemdstats

package collector

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
)

const userHZ = 100

type systemdStatsCollector struct {
	Name         string
	Pid          int
	fs           procfs.FS
	cpuSecDesc   *prometheus.Desc
	membytesDesc *prometheus.Desc
	logger       *slog.Logger
}

func init() {
	registerCollector("systemdstats", defaultEnabled, NewSystemdStatsCollector)
}

// NewSystemdStatsCollector returns a new Collector exposing process data read from the proc filesystem.
func NewSystemdStatsCollector(logger *slog.Logger) (Collector, error) {
	fs, err := procfs.NewFS(*procPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open procfs: %w", err)
	}
	subsystem := "systemdstats"
	return &systemdStatsCollector{
		Name: "systemd",
		Pid:  1,
		fs:   fs,
		cpuSecDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "cpu_seconds_total"),
			"Cpu usage in seconds",
			[]string{"pname", "mode"},
			nil,
		),
		membytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "memory_bytes"),
			"number of bytes of memory in use",
			[]string{"pname", "memtype"},
			nil,
		),
		logger: logger,
	}, nil
}

// Update implements the Collector interface
func (c *systemdStatsCollector) Update(ch chan<- prometheus.Metric) error {

	// read from /proc/[pid]/stat
	p, err := procfs.NewProc(c.Pid)
	if err != nil {
		return err
	}

	stat, err := p.Stat()
	if err != nil {
		return err
	}

	// 进程的cpu使用量(seconds)：字段utime(14)和stime(15)，即用户和系统时间
	ch <- prometheus.MustNewConstMetric(c.cpuSecDesc, prometheus.CounterValue, float64(stat.UTime/userHZ), c.Name, "user")
	ch <- prometheus.MustNewConstMetric(c.cpuSecDesc, prometheus.CounterValue, float64(stat.STime/userHZ), c.Name, "system")

	// 进程的内存使用量(bytes): 驻留内存RES(24),虚拟内存VIRT(23),共享内存SHR
	ch <- prometheus.MustNewConstMetric(c.membytesDesc, prometheus.GaugeValue, float64(stat.ResidentMemory()), c.Name, "resident")

	ch <- prometheus.MustNewConstMetric(c.membytesDesc, prometheus.GaugeValue, float64(stat.VirtualMemory()), c.Name, "virtual")

	status, err := p.NewStatus()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.membytesDesc, prometheus.GaugeValue, float64(status.VmSwap), c.Name, "swapped")

	smaps, err := p.ProcSMapsRollup()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.membytesDesc, prometheus.GaugeValue, float64(smaps.Pss), c.Name, "proportionalResident")
	ch <- prometheus.MustNewConstMetric(c.membytesDesc, prometheus.GaugeValue, float64(smaps.SwapPss), c.Name, "proportionalSwapped")

	return nil
}
