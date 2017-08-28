package cutil

import (
	"fmt"
	"io/ioutil"
	"syscall"
)

const CGROUP_MEM = "/sys/fs/cgroup/memory/"
const CGROUP_CPU = "/sys/fs/cgroup/cpu/"

//

func SetMemoryLimit(gropName string, memory int, pid int) error {
	err := syscall.Mkdir(CGROUP_MEM+gropName, 0700)
	if err == nil {
		err = ioutil.WriteFile(CGROUP_MEM+gropName+"/memory.limit_in_bytes", []byte(fmt.Sprintf("%dm", memory)), 0700)
		if err == nil {
			//			err = ioutil.WriteFile(CGROUP_MEM+gropName+"/memory.memsw.limit_in_bytes", []byte(fmt.Sprintf("%dm", memory)), 0700)
			//			if err == nil {
			err = ioutil.WriteFile(CGROUP_MEM+gropName+"/notify_on_release", []byte("1"), 0700)
			if err == nil {
				err = ioutil.WriteFile(CGROUP_MEM+gropName+"/cgroup.procs", []byte(fmt.Sprintf("%d", pid)), 0700)
				if err == nil {
					return nil
				}
			}
			//			}
		}
	}
	return err
}

func RemoveMemoryLimit(gropName string) error {
	err := syscall.Rmdir(CGROUP_MEM + gropName)
	if err == nil {
		return nil
	}
	return err
}
