package main

import (
	"bufio"
	"cutil"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

/*	veth := flag.String("net", "", "The veth device name")
	ip := flag.String("ip", "", "The conatiner ip")
	gateway := flag.String("gateway", "", "The default gateway address")
	rootfs := flag.String("rootfs", "", "The root file system to use ")
	chdir := flag.String("workdir", "/", "The working directry to execute the command")
	uid := flag.Int("uid", 0, "uid that you want to exec the user as ")
	gid := flag.Int("gid", 0, "gid that you want to exec the user as ")
	envs := flag.Var(&stringVars, "env", "List of environment vars to be passed")
*/
const CONTAINER_COMMAND = "/bin/sc_runtime"
const CONTAINER_PROXY = "/bin/sc_proxy"
const CONTAINER_NET_SETUP = "/bin/hnetsetup"

type Container struct {
	WorkDir string      `json:"workdir"`
	Nework  *Nework     `json:"network"`
	Env     []string    `json:env`
	RootFs  *FileSystem `json:"rootFs"`
	Exec    *Exec       `json:"exec"`
	Limits  *Limits     `json:"limits"`
}

type Limits struct {
	Mem           int `json:"mem"`
	MemSwappiness int `json:"memSwappiness"`
	Cpu           int `json:"cpu"`
}

type Proxy struct {
	HostPort      int    `json:"hostPort"`
	HostIP        string `json:"hostIp"`
	ContainerIP   string `json:"containerIp"`
	ContainerPort int    `json:"containerPort"`
}

type Exec struct {
	Uid     int      `json:"uid"`
	Gid     int      `json:"gid"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type FileSystem struct {
	Paths     []string `json:"paths"`
	Ephemeral bool     `json:"ephemeral"`
}

type DevicePair struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

type Nework struct {
	Ip      string      `json:"ip"`
	Gateway string      `json:"gateway"`
	Bridge  string      `json:"bridge"`
	DevPair *DevicePair `json:"devPair"`
}

type ContainerSpec struct {
	Id        string     `json:"id"`
	Proxy     *Proxy     `json:"proxy"`
	Container *Container `json:"container"`
}

//var c_kill_chan chan os.Signal
//var p_kill_chan chan os.Signal

var fsMount *cutil.Mount
var container *cutil.Runtime
var proxy *cutil.Runtime

func handle_signal(c <-chan os.Signal) {
	//fmt.Println("Host Handling signal")
	sig := <-c
	if container != nil {
		container.Signal(sig)
		//fmt.Println("Signal send to container")
	}
}

func main() {
	configFile := flag.String("config", "", "Path to the config file ")
	flag.Parse()
	var spec *ContainerSpec

	if *configFile != "" {
		// Config need to be read form the stdin
		b, err := ioutil.ReadFile(*configFile)
		if err == nil {
			err = json.Unmarshal(b, &spec)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	} else {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		err := json.Unmarshal([]byte(text), &spec)
		if err != nil {
			panic(err)
		}
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT, os.Interrupt)

	var cloneFlags uintptr
	cloneFlags = syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER

	cmd := exec.Command(CONTAINER_COMMAND)

	if spec.Container != nil && spec.Container.RootFs != nil {
		if spec.Container.WorkDir == "" {
			spec.Container.WorkDir = "/"
		}

		cmd.Args = append(cmd.Args, "--workdir", spec.Container.WorkDir, "--uid",
			fmt.Sprintf("%d", spec.Container.Exec.Uid), "--gid", fmt.Sprintf("%d", spec.Container.Exec.Gid))
		if spec.Container.Env != nil {
			for _, v := range spec.Container.Env {
				cmd.Args = append(cmd.Args, "--env", v)
			}
		}

		// Get the file system mounted
		var err error
		fsMount, err = setUpMount(spec.Id, spec.Container.RootFs)
		if err == nil {
			cmd.Args = append(cmd.Args, "--rootfs", fsMount.Path)
			// See if the need to be netwoking set up
			if spec.Container.Nework != nil {
				// If container needs
				cloneFlags |= syscall.CLONE_NEWNET

				if spec.Container.Nework.DevPair == nil {
					spec.Container.Nework.DevPair = &DevicePair{}
					netName := spec.Id
					spec.Container.Nework.DevPair.Host = "v" + netName
					spec.Container.Nework.DevPair.Container = "e" + netName
				}

				cmd.Args = append(cmd.Args, "--net", spec.Container.Nework.DevPair.Container, "--ip", spec.Container.Nework.Ip,
					"--gateway", spec.Container.Nework.Gateway)
			}

			// Adding Exec args
			cmd.Args = append(cmd.Args, spec.Container.Exec.Command)
			cmd.Args = append(cmd.Args, spec.Container.Exec.Args...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			wg := &sync.WaitGroup{}
			container, err = startContainer(cmd, cloneFlags, wg)

			if err == nil {
				pid, _ := container.GetPid()
				if spec.Container.Limits != nil {
					memLimit := &cutil.MemoryLimits{}
					memLimit.Memory = spec.Container.Limits.Mem
					memLimit.Swappiness = spec.Container.Limits.MemSwappiness
					err = cutil.SetMemoryLimit(spec.Id, memLimit, pid)
				}
				if spec.Container.Nework != nil && err == nil {
					err = setUpNetworking(pid, spec.Container.Nework)
				}
				// Now start the proxy
				if err == nil {
					if spec.Proxy != nil {
						//fmt.Println("Setting up proxy")
						proxy, err = startProxy(spec.Proxy, wg)
					}
				}
			}

			if err == nil {
				go handle_signal(sigc)
				exitCode := <-container.Wait()
				cleanUp(spec.Id)
				//fmt.Println("Container exited with ", exitCode.Code, exitCode.Error)
				wg.Wait()
				os.Exit(exitCode.Code)
			} else {
				fmt.Fprintf(os.Stderr, "Error on container set up %s\n", err.Error())
				if container != nil {
					container.Kill()
				}
				cleanUp(spec.Id)
			}

		} else {
			panic(err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No container config or root fs specified")
	}
}

// Go

func cleanUp(containerID string) {
	//fmt.Println("Cleaning up the system")
	if fsMount != nil {
		cutil.Unmount(fsMount)
	}
	if proxy != nil {
		proxy.Kill()
	}
	cutil.RemoveMemoryLimit(containerID)

}

func setUpMount(containerId string, fs *FileSystem) (*cutil.Mount, error) {
	var mount *cutil.Mount
	var err error
	if fs.Ephemeral {
		mount, err = cutil.WriteEphemeralMount(fs.Paths, containerId)
	} else {
		mount, err = cutil.WritePresistMount(fs.Paths, containerId)
	}
	if err != nil {
		return nil, err
	}
	return mount, nil
}

func startContainer(cmd *exec.Cmd, cloneFlags uintptr, wg *sync.WaitGroup) (*cutil.Runtime, error) {

	cmd.SysProcAttr = &syscall.SysProcAttr{

		Cloneflags: cloneFlags,

		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        65536,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        65536,
			},
		},
		GidMappingsEnableSetgroups: true,
	}

	r := cutil.NewRuntime(cmd)

	err := r.Start(wg)
	if err == nil {
		return r, nil
	}
	return nil, err
}

func setUpNetworking(p int, net *Nework) error {
	pid := fmt.Sprintf("%d", p)
	cmd := exec.Command(CONTAINER_NET_SETUP, "-b", net.Bridge, "-h", net.DevPair.Host, "-c", net.DevPair.Container, "-p", pid)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return err
}

func startProxy(p *Proxy, wg *sync.WaitGroup) (*cutil.Runtime, error) {

	hostLis := fmt.Sprintf("%s:%d", p.HostIP, p.HostPort)
	containerLis := fmt.Sprintf("%s:%d", p.ContainerIP, p.ContainerPort)

	cmd := exec.Command(CONTAINER_PROXY, hostLis, containerLis)
	cmd.Stderr = nil
	cmd.Stdout = nil

	po := cutil.NewRuntime(cmd)
	err := po.Start(wg)
	if err != nil {
		//		wg.Done()
		return nil, err
	}
	return po, nil
}
