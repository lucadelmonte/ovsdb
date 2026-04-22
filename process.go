// Copyright 2018 Paul Greenberg (greenpau@outlook.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovsdb

import (
	"bufio"
	"fmt"
	"io/ioutil" //nolint:staticcheck
	"os"
	"os/user"
	"strconv"
	"strings"
)

// OvsProcess stores information about a process, e.g. user and
// group, current parent process ids.
type OvsProcess struct {
	ID     int
	User   string
	Group  string
	Parent struct {
		ID int
	}
}

func getProcessInfo(pid int) (OvsProcess, error) {
	p := OvsProcess{
		ID: pid,
	}
	if pid == 0 {
		return p, nil
	}
	f := "/proc/" + strconv.Itoa(pid) + "/status"
	file, err := os.Open(f)
	if err != nil {
		return p, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "PPid:") {
			ppid := strings.Replace(scanner.Text(), "PPid:", "", -1)
			ppid = strings.TrimSpace(ppid)
			ppidi, err := strconv.Atoi(ppid)
			if err != nil {
				return p, err
			}
			p.Parent.ID = ppidi
		}
		if strings.HasPrefix(scanner.Text(), "Uid:") {
			puid := strings.Replace(scanner.Text(), "Uid:", "", -1)
			pUIDArray := strings.Split(strings.TrimSpace(puid), "\t")
			p.User = pUIDArray[0]
			if u, err := user.LookupId(p.User); err == nil {
				p.User = u.Username
			} else {
				p.User = err.Error()
			}
		}
		if strings.HasPrefix(scanner.Text(), "Gid:") {
			pgid := strings.Replace(scanner.Text(), "Gid:", "", -1)
			pGidArray := strings.Split(strings.TrimSpace(pgid), "\t")
			p.Group = pGidArray[0]
			if g, err := user.LookupGroupId(p.Group); err == nil {
				p.Group = g.Name
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return p, err
	}
	return p, nil
}

func getProcessInfoFromFile(f string) (OvsProcess, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return OvsProcess{}, err
	}
	pid, err := strconv.Atoi(strings.TrimSuffix(string(data), "\n"))
	if err != nil {
		return OvsProcess{}, err
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return OvsProcess{}, err
	}
	info, err := getProcessInfo(p.Pid)
	if err != nil {
		return OvsProcess{}, err
	}
	return info, nil
}

// GetProcessInfo probes the liveness of an OVN component by querying its
// control socket. It returns a zero-value OvsProcess on success (the value
// is intentionally empty since PID/user/group read from the host /proc
// namespace is meaningless when the daemon runs inside a container).
// Callers should treat a nil error as "component is reachable".
func (cli *OvnClient) GetProcessInfo(name string) (OvsProcess, error) {
	var ctl string
	switch name {
	case "ovsdb-server-southbound":
		ctl = cli.Database.Southbound.Socket.Control
	case "ovsdb-server-northbound":
		ctl = cli.Database.Northbound.Socket.Control
	case "ovn-northd":
		ctl = cli.Service.Northd.Socket.Control
	default:
		return OvsProcess{}, fmt.Errorf("The '%s' component is unsupported", name)
	}
	if _, err := getVersionViaAppctl(ctl, cli.Timeout); err != nil {
		return OvsProcess{}, fmt.Errorf("'%s' is not reachable via %s: %s", name, ctl, err)
	}
	return OvsProcess{}, nil
}

// GetProcessInfo probes the liveness of an OVS component by querying its
// control socket. The PID read from the .pid file is used only to resolve
// the PID-suffixed socket filename, not to stat /proc on the host, so the
// probe works when the daemon runs in a different PID namespace.
func (cli *OvsClient) GetProcessInfo(name string) (OvsProcess, error) {
	var pidPath string
	switch name {
	case "ovsdb-server":
		pidPath = cli.Database.Vswitch.File.Pid.Path
	case "ovs-vswitchd":
		pidPath = cli.Service.Vswitchd.File.Pid.Path
	default:
		return OvsProcess{}, fmt.Errorf("The '%s' component is unsupported", name)
	}

	pid, err := readPidFromFile(pidPath)
	if err != nil {
		return OvsProcess{}, err
	}

	switch name {
	case "ovsdb-server":
		cli.Database.Vswitch.Process.ID = pid
	case "ovs-vswitchd":
		cli.Service.Vswitchd.Process.ID = pid
	}
	cli.updateRefs()

	var ctl string
	switch name {
	case "ovsdb-server":
		ctl = cli.Database.Vswitch.Socket.Control
	case "ovs-vswitchd":
		ctl = cli.Service.Vswitchd.Socket.Control
	}

	if _, err := getVersionViaAppctl(ctl, cli.Timeout); err != nil {
		return OvsProcess{}, fmt.Errorf("'%s' is not reachable via %s: %s", name, ctl, err)
	}
	return OvsProcess{ID: pid}, nil
}

func readPidFromFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSuffix(string(data), "\n"))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
