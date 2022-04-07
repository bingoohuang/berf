package procstat

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/gg/pkg/osx"

	"github.com/bingoohuang/berf/plugins"
)

type PID int32

type Procstat struct {
	PidFinder              string
	PidFile                string
	Exe                    string
	Pattern                string
	CmdLineTag             bool
	User                   string
	SystemdUnit            string
	IncludeSystemdChildren bool
	CGroup                 string

	finder PIDFinder

	createPIDFinder func() (PIDFinder, error)
	procs           map[PID]Process
	createProcess   func(PID) (Process, error)
}

/*
## Monitor process cpu and memory usage
## PID file to monitor process
PidFile = "/var/run/nginx.pid"
## executable name (ie, pgrep <exe>)
# Exe = "nginx"
## pattern as argument for pgrep (ie, pgrep -f <pattern>)
# Pattern = "nginx"
## user as argument for pgrep (ie, pgrep -u <user>)
# User = "nginx"
## Systemd unit name, supports globs when include_systemd_children is set to true
# SystemdUnit = "nginx.service"
# IncludeSystemdChildren = false
## CGroup name or path, supports globs
# CGroup = "systemd/system.slice/nginx.service"

## When true add the full cmdline as a tag.
# CmdLineTag = false

## Method to use when finding process IDs.  Can be one of 'pgrep', or
## 'native'.  The pgrep finder calls the pgrep executable in the PATH while
## the native finder performs the search directly in a manor dependent on the
## platform.  Default is 'pgrep'
# PidFinder = "pgrep"
*/

type Pids struct {
	PIDS []PID
	Err  error
}

func (p *Procstat) Init() error {
	if p.createPIDFinder == nil {
		switch p.PidFinder {
		case "native":
			p.createPIDFinder = NewNativeFinder
		case "pgrep":
			p.createPIDFinder = NewPgrep
		default:
			p.PidFinder = "pgrep"
			p.createPIDFinder = NewPgrep
		}
	}
	if p.createProcess == nil {
		p.createProcess = NewProc
	}

	newProcs := make(map[PID]Process, len(p.procs))
	pidTags := p.findPids()
	for _, pidTag := range pidTags {
		if pidTag.Err != nil {
			return pidTag.Err
		}

		if _, err := p.updateProcesses(pidTag.PIDS, p.procs, newProcs); err != nil {
			return fmt.Errorf("procstat getting process, exe: [%s] pidfile: [%s] pattern: [%s] user: [%s] %s",
				p.Exe, p.PidFile, p.Pattern, p.User, err.Error())
		}
	}

	p.procs = newProcs
	return nil
}

func (p *Procstat) Series() plugins.Series {
	ps := plugins.Series{}
	for _, proc := range p.procs {
		name, _ := proc.Name()
		pid := proc.PID()

		if len(name) > 5 {
			name = name[len(name)-5:]
		}

		pp := name + ":"
		if len(p.procs) > 1 {
			pp = fmt.Sprintf("%s-%d-", name, pid)
		}

		for _, f := range fields {
			ps.Series = append(ps.Series, pp+f)
		}

		ps.Selected = append(ps.Selected, pp+"rss")
	}

	return ps
}

func (p *Procstat) Gather() ([]interface{}, error) {
	var points []interface{}
	for _, proc := range p.procs {
		metric := p.addMetric(proc)
		for _, f := range fields {
			points = append(points, metric[f])
		}
	}

	return points, nil
}

var fields = []string{"threads", "fds", "%cpu", "rss", "vms", "swap"}

// Add metrics a single Process
func (p *Procstat) addMetric(proc Process) map[string]util.Float64 {
	f := map[string]util.Float64{}

	if v, err := proc.NumThreads(); err == nil {
		f["threads"] = util.Float64(v)
	}

	if v, err := proc.NumFDs(); err == nil {
		f["fds"] = util.Float64(v)
	}

	if v, err := proc.Percent(time.Duration(0)); err == nil {
		f["%cpu"] = util.Float64(v)
	}

	if v, err := proc.MemoryInfo(); err == nil {
		f["rss"] = util.BytesToMEGA(v.RSS)
		f["vms"] = util.BytesToMEGA(v.VMS)
		f["swap"] = util.BytesToMEGA(v.Swap)
	}

	return f
}

// Update monitored Processes
func (p *Procstat) updateProcesses(pids []PID, prevInfo, procs map[PID]Process) (bool, error) {
	updated := false
	for _, pid := range pids {
		if info, ok := prevInfo[pid]; ok {
			// Assumption: if a process has no name, it probably does not exist
			if name, _ := info.Name(); name == "" {
				updated = true
				continue
			}
			procs[pid] = info
			continue
		}

		proc, err := p.createProcess(pid)
		if err != nil {
			// No problem; process may have ended after we found it
			continue
		}
		// Assumption: if a process has no name, it probably does not exist
		if name, _ := proc.Name(); name == "" {
			continue
		}

		updated = true
		procs[pid] = proc
	}
	return updated, nil
}

// Create and return PIDGatherer lazily
func (p *Procstat) getPIDFinder() (PIDFinder, error) {
	if p.finder == nil {
		f, err := p.createPIDFinder()
		if err != nil {
			return nil, err
		}
		p.finder = f
	}
	return p.finder, nil
}

// Get matching PIDs and their initial tags
func (p *Procstat) findPids() []Pids {
	var pidTags []Pids

	if p.SystemdUnit != "" {
		return p.systemdUnitPIDs()
	} else if p.CGroup != "" {
		return p.cgroupPIDs()
	} else {
		f, err := p.getPIDFinder()
		if err != nil {
			return append(pidTags, Pids{nil, err})
		}
		pids, err := p.SimpleFindPids(f)
		pidTags = append(pidTags, Pids{pids, err})
	}

	return pidTags
}

// SimpleFindPids get matching PIDs and their initial tags
func (p *Procstat) SimpleFindPids(f PIDFinder) ([]PID, error) {
	var pids []PID
	var err error

	if p.PidFile != "" {
		if p.PidFile == "self" {
			return []PID{PID(os.Getpid())}, nil
		}
		pids, err = f.PidFile(p.PidFile)
	} else if p.Exe != "" {
		pids, err = f.Pattern(p.Exe)
	} else if p.Pattern != "" {
		pids, err = f.FullPattern(p.Pattern)
	} else if p.User != "" {
		pids, err = f.UID(p.User)
	} else {
		err = fmt.Errorf("either exe, pid_file, user, pattern, systemd_unit, cgroup, or win_service must be specified")
	}

	return pids, err
}

// execCommand is so tests can mock out exec.Command usage.
var execCommand = exec.Command

func (p *Procstat) systemdUnitPIDs() []Pids {
	if p.IncludeSystemdChildren {
		p.CGroup = fmt.Sprintf("systemd/system.slice/%s", p.SystemdUnit)
		return p.cgroupPIDs()
	}

	var pidTags []Pids

	pids, err := p.simpleSystemdUnitPIDs()
	pidTags = append(pidTags, Pids{pids, err})
	return pidTags
}

func (p *Procstat) simpleSystemdUnitPIDs() ([]PID, error) {
	var pids []PID

	cmd := execCommand("systemctl", "show", p.SystemdUnit)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		kv := bytes.SplitN(line, []byte{'='}, 2)
		if len(kv) != 2 {
			continue
		}
		if !bytes.Equal(kv[0], []byte("MainPID")) {
			continue
		}
		if len(kv[1]) == 0 || bytes.Equal(kv[1], []byte("0")) {
			return nil, nil
		}
		pid, err := strconv.ParseInt(string(kv[1]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid pid '%s'", kv[1])
		}
		pids = append(pids, PID(pid))
	}

	return pids, nil
}

func (p *Procstat) cgroupPIDs() []Pids {
	var pidTags []Pids

	procsPath := p.CGroup
	if procsPath[0] != '/' {
		procsPath = "/sys/fs/cgroup/" + procsPath
	}
	items, err := filepath.Glob(procsPath)
	if err != nil {
		pidTags = append(pidTags, Pids{nil, fmt.Errorf("glob failed '%s'", err)})
		return pidTags
	}
	for _, item := range items {
		pids, err := p.singleCgroupPIDs(item)
		pidTags = append(pidTags, Pids{pids, err})
	}

	return pidTags
}

func (p *Procstat) singleCgroupPIDs(path string) ([]PID, error) {
	var pids []PID

	ok, err := isDir(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("not a directory %s", path)
	}
	procsPath := filepath.Join(path, "cgroup.procs")
	out := osx.ReadFile(procsPath)
	if out.Err != nil {
		return nil, out.Err
	}
	for _, pidBS := range bytes.Split(out.Data, []byte{'\n'}) {
		if len(pidBS) == 0 {
			continue
		}
		pid, err := strconv.ParseInt(string(pidBS), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid pid '%s'", pidBS)
		}
		pids = append(pids, PID(pid))
	}

	return pids, nil
}

func isDir(path string) (bool, error) {
	result, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return result.IsDir(), nil
}

func init() {
	plugins.Add("procstat", func() plugins.Input {
		return &Procstat{PidFile: "self"}
	})
}
