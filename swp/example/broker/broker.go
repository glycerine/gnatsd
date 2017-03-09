package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	swp "github.com/glycerine/hnatsd/swp"
)

func main() {
	forkChildReturnsParentDies()

	host := os.Getenv("BROKER_HOST")
	port := os.Getenv("BROKER_PORT")
	if host == "" {
		fmt.Fprintf(os.Stderr, "BROKER_HOST in env was not set. Setting required.\n")
		os.Exit(1)
	}
	if port == "" {
		fmt.Fprintf(os.Stderr, "BROKER_PORT in env was not set. Setting required.\n")
		os.Exit(1)
	}
	nport, err := strconv.Atoi(port)
	panicOn(err)

	fmt.Printf("starting nats://%v:%v", host, nport)

	gnats := swp.StartGnatsd(host, nport)
	fmt.Printf("\nnats://%v:%v\n", host, port)
	fmt.Printf("export BROKER_HOST=%v\n", host)
	fmt.Printf("export BROKER_PORT=%v\n", port)

	detach()
	gnats.Start()
	select {}
}

// getAvailPort asks the OS for an unused port.
// There's a race here, where the port could be grabbed by someone else
// before the caller gets to Listen on it, but in practice such races
// are rare. Uses net.Listen("tcp", ":0") to determine a free port, then
// releases it back to the OS with Listener.Close().
func getAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}

func forkChildReturnsParentDies() {
	// avoid race condition between os.StartProcess and os.Exit
	err := syscall.FcntlFlock(os.Stdout.Fd(), syscall.F_SETLKW, &syscall.Flock_t{
		Type: syscall.F_WRLCK, Whence: 0, Start: 0, Len: 0})
	if err != nil {
		log.Fatalln("Failed to lock stdout:", err)
	}
	pid := os.Getpid()
	if os.Getppid() != 1 {
		fmt.Printf("Pre fork, I am the parent, pid %v\n", pid)

		// I am the parent, spawn child to run as daemon
		binary, err := exec.LookPath(os.Args[0])
		if err != nil {
			log.Fatalln("Failed to lookup binary:", err)
		}
		_, err = os.StartProcess(binary, os.Args, &os.ProcAttr{Dir: "", Env: nil,
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}, Sys: nil})
		if err != nil {
			log.Fatalln("Failed to start process:", err)
		}
		os.Exit(0)
	}
}

func detach() {
	pid := os.Getpid()

	// I am the child, i.e. the daemon, start new session and detach from terminal
	fmt.Printf("After fork, I am the child, pid %v\n", pid)
	_, err := syscall.Setsid()
	if err != nil {
		log.Fatalln("Failed to create new session:", err)
	}

	fn := fmt.Sprintf("background.out.%v", pid)
	file, err := os.Create(fn)
	if err != nil {
		log.Fatalf("Failed to open '%s': '%s'", fn, err)
	}
	devnull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		log.Fatalf("Failed to open /dev/null: '%s'", fn, err)
	}

	syscall.Dup2(int(devnull.Fd()), int(os.Stdin.Fd()))
	syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd()))
	syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
	file.Close()
	devnull.Close()
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
