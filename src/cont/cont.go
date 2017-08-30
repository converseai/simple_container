package main

import (
	"cutil"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type stringSlice []string

const IP_COMMAND = "/sbin/ip"

var process *cutil.Runtime

var kill_chan = make(chan os.Signal, 1)

func (s *stringSlice) String() string {
	return fmt.Sprintf("%s", *s)
}

func (s *stringSlice) Set(str string) error {
	*s = append(*s, str)
	return nil
}

func handle_signal(c <-chan os.Signal) {
	sig := <-c
	if process != nil {
		//		fmt.Println("Sending to runing command")
		process.Signal(sig)
	}
}

func main() {
	var envs stringSlice
	veth := flag.String("net", "", "The veth device name")
	ip := flag.String("ip", "", "The conatiner ip")
	gateway := flag.String("gateway", "", "The default gateway address")
	rootfs := flag.String("rootfs", "", "The root file system to use ")
	chdir := flag.String("workdir", "/", "The working directry to execute the command")
	uid := flag.Int("uid", 0, "uid that you want to exec the user as ")
	gid := flag.Int("gid", 0, "gid that you want to exec the user as ")
	flag.Var(&envs, "env", "List of environment vars to be passed")

	flag.Parse()

	app := flag.Args()

	//fmt.Printf("%v", app)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT, os.Interrupt)

	if app == nil || len(app) == 0 {
		fmt.Fprintf(os.Stderr, "No app specified to exec")
		os.Exit(2)
	}

	if *veth != "" {
		// Set the hostname
		err := syscall.Sethostname([]byte("converseai"))
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

		err = waitForVeth(*veth)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		//Set up network
		err = setContainerNetworking(*veth, *ip, *gateway)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		//		fmt.Println("Network is up")
	}

	// Set up the root fs and swap dir this will remove the mounts etc
	err := syscall.Chroot(*rootfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	//	fmt.Println("Chroot mounted ")

	// Mount proc
	err = syscall.Mount("proc", "/proc", "proc", 0, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	//	fmt.Println("proc mounted ")

	err = syscall.Chdir(*chdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	//	fmt.Println("chdir done mounted ")
	//Now exec the process
	var cmd *exec.Cmd
	if len(app) > 0 {
		cmd = exec.Command(app[0], app[1:]...)
	} else {
		cmd = exec.Command(app[0])
	}

	//	fmt.Println("Runing commands ", cmd.Args)

	cmd.Env = []string{"PATH=/bin/:/usr/bin/:/sbin/"}
	cmd.Env = append(cmd.Env, envs[:]...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(*uid),
			Gid: uint32(*gid),
		},
		//		GidMappingsEnableSetgroups: false,
	}

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	process = cutil.NewRuntime(cmd)
	err = process.Start(nil)

	if err == nil {
		go handle_signal(sigc)
		exitCode := <-process.Wait()
		if exitCode.Error != nil {
			fmt.Fprintf(os.Stderr, exitCode.Error.Error())
		}
		os.Exit(exitCode.Code)
	} else {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(-1)
	}
}

func waitForVeth(veth string) error {
	maxWait := time.Second * 5
	checkInterval := time.Microsecond * 30
	timeStarted := time.Now()

	for {
		iface, _ := net.InterfaceByName(veth)

		if iface != nil {
			return nil
		}
		if time.Since(timeStarted) > maxWait {
			return fmt.Errorf("Timeout after %s waiting for network", maxWait)
		}

		time.Sleep(checkInterval)
	}

}

func setContainerNetworking(device, ip, gateway string) error {
	cmd := exec.Command(IP_COMMAND, "link", "set", device, "up")
	err := cmd.Run()
	if err == nil {
		cmd = exec.Command(IP_COMMAND, "addr", "add", ip, "dev", device)
		err = cmd.Run()
		if err == nil {
			cmd = exec.Command(IP_COMMAND, "route", "add", "default", "via", gateway)
			err = cmd.Run()
		}
	}
	if err == nil {
		return nil
	} else {
		return err
	}
}
