package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
)

const (
	DBUS_NAME            = "com.example.goscopecontroller1"
	UNIT          string = "go-scope-controller.service"
	SCOPE         string = "go-scope-worker.scope"
	DBUS_OBJ_PATH string = "/org/freedesktop/systemd1/unit/go_2dscope_2dcontroller_2eservice"
)

func main() {
	// Connect to DBus
	systemBus, err := godbus.SystemBus()
	if err != nil {
		log.Fatalf("failed to connect to system bus: %v\n", err)
	}
	defer systemBus.Close()

	// Claim well-known name
	reply, err := systemBus.RequestName(DBUS_NAME, godbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatalf("failed to request well-known DBus name")
	}

	if reply != godbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("DBus name %v already taken", DBUS_NAME)
	}

	// Setup signal watch
	err = systemBus.AddMatchSignal(
		godbus.WithMatchObjectPath(godbus.ObjectPath(DBUS_OBJ_PATH)),
		godbus.WithMatchInterface("org.freedesktop.systemd1.Scope"),
		godbus.WithMatchSender("org.freedesktop.systemd1"),
		godbus.WithMatchMember("RequestStop"))

	if err != nil {
		log.Fatalf("failed to subscribe to DBus signal")
	}

	stopChan := make(chan *godbus.Signal, 1)
	systemBus.Signal(stopChan)

	// Create dbus connection to systemd
	sd, err := dbus.New()
	if err != nil {
		log.Fatalf("failed to setup DBus connection to systemd: %v\n", err)
	}
	defer sd.Close()

	// Spawn "worker" process
	cmd := exec.Command("sleep", "infinity")
	err = cmd.Start()
	if err != nil {
		log.Fatalf("failed to start worker: %v\n", err)
	}

	log.Printf("worker running as PID: %d\n", cmd.Process.Pid)

	resultChan := make(chan string)
	controller := dbus.Property{Name: "Controller", Value: godbus.MakeVariant(DBUS_NAME)}
	scopeProperties := []dbus.Property{dbus.PropSlice("system.slice"), dbus.PropPids(uint32(cmd.Process.Pid)), controller}

	_, err = sd.StartTransientUnit(SCOPE, "replace", scopeProperties, resultChan)
	if err != nil {
		log.Fatalf("failed to create worker scope unit: %v\n", err)
	}

	r := <-resultChan
	if r != "done" {
		log.Fatalf("StartTransientUnit() failed with job result: %v\n", r)
	}

	log.Printf("%s started\n", SCOPE)

	// Setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Ignore()
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	select {
	case <-signalChan:
		log.Printf("received stop signal")
	case <-stopChan:
		log.Printf("received RequestStop signal over DBus")

	}

	log.Printf("about to stop %s\n", SCOPE)

	// Kill the worker process and wait for it
	// Note that calling StopUnit() DBus API with "replace" job mode would create deadlock
	err = cmd.Process.Kill()
	if err != nil {
		log.Fatalf("failed to kill worker process: %v\n", err)
	}

	cmd.Wait()

	log.Printf("worker killed\n")
	log.Printf("%s exiting\n", UNIT)

}
