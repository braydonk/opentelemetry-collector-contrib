// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *scraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.Iowait, metadata.AttributeStateWait)
}

func (s *scraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.Iowait, metadata.AttributeStateWait)
}

func getProcessName(ctx context.Context, proc processHandle, _ string) (string, error) {
	name, err := proc.NameWithContext(ctx)
	if err != nil {
		return "", err
	}

	return name, err
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	exe, err := proc.ExeWithContext(ctx)
	if err != nil {
		return "", err
	}

	return exe, nil
}

func getProcessCgroup(ctx context.Context, proc processHandle) (string, error) {
	cgroup, err := proc.CgroupWithContext(ctx)
	if err != nil {
		return "", err
	}

	return cgroup, nil
}

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var cmd string
	if len(cmdline) > 0 {
		cmd = cmdline[0]
	}

	command := &commandMetadata{command: cmd, commandLineSlice: cmdline}
	return command, nil
}

func getStatData(pid int32, tid int32) (uint64, int32, *cpu.TimesStat, int64, uint32, int32, *process.PageFaultsStat, error) {
	clockTicks := 100
	var statPath string

	if tid == -1 {
		statPath = filepath.Join("/proc", strconv.Itoa(int(pid)), "stat")
	} else {
		statPath = filepath.Join("/proc", strconv.Itoa(int(pid)), "task", strconv.Itoa(int(tid)), "stat")
	}

	content, err := os.ReadFile(statPath)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}

	nameStart := bytes.IndexByte(content, '(')
	nameEnd := bytes.LastIndexByte(content, ')')
	restFields := strings.Fields(string(content[nameEnd+2:])) // +2 skip ') '
	name := content[nameStart+1 : nameEnd]
	pidField := strings.TrimSpace(string(content[:nameStart]))
	fields := make([]string, 3, len(restFields)+3)
	fields[1] = string(pidField)
	fields[2] = string(name)
	fields = append(fields, restFields...)

	terminal, err := strconv.ParseUint(fields[7], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}

	ppid, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	utime, err := strconv.ParseFloat(fields[14], 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}

	stime, err := strconv.ParseFloat(fields[15], 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}

	// There is no such thing as iotime in stat file.  As an approximation, we
	// will use delayacct_blkio_ticks (aggregated block I/O delays, as per Linux
	// docs).  Note: I am assuming at least Linux 2.6.18
	var iotime float64
	if len(fields) > 42 {
		iotime, err = strconv.ParseFloat(fields[42], 64)
		if err != nil {
			iotime = 0 // Ancient linux version, most likely
		}
	} else {
		iotime = 0 // e.g. SmartOS containers
	}

	cpuTimes := &cpu.TimesStat{
		CPU:    "cpu",
		User:   utime / float64(clockTicks),
		System: stime / float64(clockTicks),
		Iowait: iotime / float64(clockTicks),
	}

	var bootTime uint64
	procStat, _ := os.Open("/proc/stat")
	s := bufio.NewScanner(procStat)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "btime") {
			f := strings.Fields(line)
			if len(f) != 2 {
				bootTime = 0
			}
			b, err := strconv.ParseInt(f[1], 10, 64)
			if err != nil {
				bootTime = 0
			}
			bootTime = uint64(b)
			break
		}
	}
	t, err := strconv.ParseUint(fields[22], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	ctime := (t / uint64(clockTicks)) + uint64(bootTime)
	createTime := int64(ctime * 1000)

	rtpriority, err := strconv.ParseInt(fields[18], 10, 32)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	if rtpriority < 0 {
		rtpriority = rtpriority*-1 - 1
	} else {
		rtpriority = 0
	}

	//	p.Nice = mustParseInt32(fields[18])
	// use syscall instead of parse Stat file
	snice, _ := unix.Getpriority(0, int(pid))
	nice := int32(snice) // FIXME: is this true?

	minFault, err := strconv.ParseUint(fields[10], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	cMinFault, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	majFault, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}
	cMajFault, err := strconv.ParseUint(fields[13], 10, 64)
	if err != nil {
		return 0, 0, nil, 0, 0, 0, nil, err
	}

	faults := &process.PageFaultsStat{
		MinorFaults:      minFault,
		MajorFaults:      majFault,
		ChildMinorFaults: cMinFault,
		ChildMajorFaults: cMajFault,
	}

	return terminal, int32(ppid), cpuTimes, createTime, uint32(rtpriority), nice, faults, nil
}
