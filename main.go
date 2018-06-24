package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// go run main.go run <cmd> <args>
// go build; sudo ./lizrice-cfs2 run <cmd> <args>

// rm ./lizrice-cfs2 ; go build; sudo ./lizrice-cfs2 run bash
// forkbomb() { forkbomb | forkbomb &); forkbomb
// cat /sys/fs/cgroup/pids/my_cgroup/pids.current  ## will be 20 since we cap at that
func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("help")
	}
}

func run() {
	fmt.Printf("Running %v as pid %d\n", os.Args[2:], os.Getpid())

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		Credential: &syscall.Credential{Uid: 0, Gid: 0},
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	must(cmd.Run())
}

func child() {
	fmt.Printf("Running %v as pid %d\n", os.Args[2:], os.Getpid())

	cg()

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(syscall.Sethostname([]byte("container")))

	// wont work without linux root filesystem on the desired place on disk
	// sudo mkdir /home/rootfs; cd /home
	// sudo debootstrap --arch=amd64 --variant=buildd xenial rootfs http://archive.ubuntu.com/ubuntu/
	must(syscall.Chroot("/home/rootfs"))

	must(syscall.Chdir("/"))
	must(syscall.Mount("proc", "proc", "proc", 0, ""))
	must(cmd.Run())

	must(syscall.Unmount("proc", 0))
}

func cg() {
	cgroups := "/sys/fs/cgroup"
	pids := filepath.Join(cgroups, "pids")

	must(os.Mkdir(filepath.Join(pids, "my_cgroup"), 0755))
	// limit container to 20 processes max
	must(ioutil.WriteFile(filepath.Join(pids, "my_cgroup/pids.max"), []byte("20"), 0700))
	// removes the new cgroup in place after the container exits
	must(ioutil.WriteFile(filepath.Join(pids, "my_cgroup/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(pids, "my_cgroup/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
