package main

import (
	"bytes"
	"html/template"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
)

var checklist chan Check

// Check contains test for a key, the response from etcd and a stop channel
type Check struct {
	test *Test
	node *client.Response
	stop chan bool
}

// up makes periodical test
func (c Check) up() {
	for {
		select {
		case <-c.stop:
			log.Println("STOP check goroutine", c.node)
			return
		case <-time.Tick(c.test.Interval * time.Second):
			c.makeTest()
		}
	}
}

// MakeTest launches test.
func (c Check) makeTest() {
	var err error

	// find a test and execute it
	if fnc, ok := TESTS[c.test.Test]; ok {
		err = fnc(c.test, c.node)
	} else {
		log.Println(c.test.Test, "is not a known test")
		return
	}

	if err != nil {
		//w.kapi.Delete(context.Background(), node.Node.Key, nil)
		if c.test.CommandFailed == "" {
			log.Println("No command for failed state specified")
			return
		}
		execCommand(c.test.CommandFailed, c.node)
	} else {
		if c.test.CommandOK == "" {
			return
		}
		execCommand(c.test.CommandOK, c.node)
	}
}

// parseCommand returns a parsed command from configuration.
func parseCommand(cmd string, node *client.Response) (*exec.Cmd, error) {

	tpl, err := template.New("Cmd").Parse(cmd)
	if err != nil {
		return nil, err
	}

	var b []byte
	buff := bytes.NewBuffer(b)
	if err := tpl.Execute(buff, node.Node); err != nil {
		return nil, err
	}

	args := strings.Split(buff.String(), " ")

	if len(args) > 0 {
		return exec.Command(args[0], args[1:]...), nil
	}

	return exec.Command(args[0]), nil
}

func execCommand(command string, node *client.Response) {
	// create a parsed command
	cmd, err := parseCommand(command, node)
	if err != nil {
		log.Println(err)
		return
	}
	// Use stdin and stdout to see the result
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	// launch
	if err := cmd.Run(); err != nil {
		log.Println("Run", err)
	}
}

// initialize the parallels routines
func setParallel(size int) {
	checklist = make(chan Check, size)
	runtime.GOMAXPROCS(size)
	log.Println("Launching", size, "check goroutines")
	for i := 0; i < size; i++ {
		go func() {
			for check := range checklist {
				checks[check.node.Node.Key] = check.stop
				check.up()
			}
		}()
	}
}
