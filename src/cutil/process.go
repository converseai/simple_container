package cutil

import (
	"errors"
	//	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type ExitCode struct {
	Code  int
	Error error
}

type Runtime struct {
	cmd      *exec.Cmd
	exitChan chan *ExitCode
	killChan chan os.Signal
	exit     bool
}

func NewRuntime(cmd *exec.Cmd) *Runtime {
	r := &Runtime{
		cmd:      cmd,
		exitChan: make(chan *ExitCode, 1),
		killChan: make(chan os.Signal, 1),
	}
	return r
}

func (cr *Runtime) Start(wg *sync.WaitGroup) error {
	err := cr.cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		kill := <-cr.killChan
		if kill != nil {
			cr.exit = true
			//			fmt.Println("Send to runtime ", kill, cr.cmd.Args[0])
			_ = cr.cmd.Process.Signal(kill)
			//			fmt.Println("Send to runtime ", kill, cr.cmd.Args[0], err)
		}
	}()

	go func() {
		if wg != nil {
			wg.Add(1)
		}
		err := cr.cmd.Wait()
		//		fmt.Println("Out of wait", cr.cmd.Args[0], err)
		if wg != nil {
			wg.Done()
		}
		exit := &ExitCode{}
		exit.Code = 0
		if err == nil {
			exit.Code = cr.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
		} else {

			if exitError, ok := err.(*exec.ExitError); ok {
				exit.Code = exitError.Sys().(syscall.WaitStatus).ExitStatus()
			} else {
				exit.Code = 1
				exit.Error = err
			}

		}
		cr.exitChan <- exit
		close(cr.killChan)
		close(cr.exitChan)
	}()
	return nil
}

func (cr *Runtime) GetPid() (int, error) {
	if cr.cmd.Process != nil {
		return cr.cmd.Process.Pid, nil
	} else {
		return -1, errors.New("Process is not started yet")
	}
}

func (cr *Runtime) Kill() {
	cr.killChan <- syscall.SIGKILL
}

func (cr *Runtime) Terminate() {
	cr.killChan <- syscall.SIGTERM
}

func (cr *Runtime) Signal(sig os.Signal) error {
	cr.killChan <- sig
	return nil
}

func (cr *Runtime) Wait() <-chan *ExitCode {
	return cr.exitChan
}
