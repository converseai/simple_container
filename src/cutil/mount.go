package cutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Mount struct {
	Path        string
	Ephemeral   bool
	upper       string
	containerID string
}

const BASE_PATH = "/var/lib/simple_container/"

func WriteEphemeralMount(paths []string, containerID string) (*Mount, error) {
	mount := new(Mount)
	mount.Ephemeral = true
	mount.containerID = containerID
	// Create a new ephemeral folder is one os the remove it
	efolderPath := filepath.Join(BASE_PATH, containerID, "ephemeral")
	syscall.Unlink(efolderPath)
	// Create the folder
	err := os.MkdirAll(efolderPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}
	mount.upper = efolderPath

	mPath := filepath.Join(BASE_PATH, containerID, "mnt")
	syscall.Unlink(mPath)

	err = os.MkdirAll(mPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}

	workPath := filepath.Join(BASE_PATH, containerID, "work")
	syscall.Unlink(workPath)
	err = os.MkdirAll(workPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(paths, ":"), efolderPath, workPath)
	//	fmt.Printf("Overlay FS mount %s : %s", mPath, opts)
	err = syscall.Mount("overlay", mPath, "overlay", 0, opts)
	if err != nil {
		return nil, err
	}

	mount.Path = mPath

	return mount, nil
}

func WritePresistMount(paths []string, containerID string) (*Mount, error) {
	mount := new(Mount)
	mount.Ephemeral = false
	mount.containerID = containerID

	mPath := filepath.Join(BASE_PATH, containerID, "mnt")
	syscall.Unlink(mPath)

	err := os.MkdirAll(mPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}

	workPath := filepath.Join(BASE_PATH, containerID, "work")
	syscall.Unlink(workPath)
	err = os.MkdirAll(workPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}
	p := len(paths)
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(paths[:(p-1)], ":"), paths[p-1], workPath)
	err = syscall.Mount("overlay", mPath, "overlay", 0, opts)
	if err != nil {
		return nil, err
	}

	mount.Path = mPath

	return mount, nil
}

func Unmount(m *Mount) error {
	err := syscall.Unmount(m.Path, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	err = os.RemoveAll(filepath.Join(BASE_PATH, m.containerID))
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	return nil
}
