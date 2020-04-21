// __author__ = 'jean'
// Created by zhang at 2020/4/20 10:53
// Process by go

package main

import (
	"bytes"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
	"time"
)

var logList seqStringFlag

type seqStringFlag []string

func (f *seqStringFlag) String() string {
	return fmt.Sprint(*f)
}

func (f *seqStringFlag) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		*f = append(*f, v)
	}
	return nil
}

var (
	port   = flag.String("port", "3903", "HTTP port to listen on.")
	progs  = flag.String("progs", "/etc/mtail/", "Name of the directory containing mtail programs")
	period = flag.Int("period", 1, "logs file check period, Unit is minutes")
)

func init() {
	flag.Var(&logList, "logs", "List of log files to monitor, separated by commas.  This flag may be specified multiple times. You can Use fuzzy matching in the path, like --logs \"/Path/*/to/*.log\"")
}

var (
	addrChan = make(chan string, 1)
)

func main() {
	flag.Parse()
	fmt.Println(logList)
	go func() {
		var lastLogsAddr string
		logList = append([]string{"find"}, logList...)
		for {
			cmdFindLogs := exec.Command("bash", "-c", strings.Join(logList, " "))
			var stdout, stderr bytes.Buffer
			cmdFindLogs.Stdout = &stdout
			cmdFindLogs.Stderr = &stderr
			err := cmdFindLogs.Run()
			if err != nil {
				log.Warningf("cmd.Run() failed with %s\n", err)
			}
			outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
			if errStr != "" {
				fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
			}
			LogsAddr := strings.Join(strings.Split(strings.TrimSpace(outStr), "\n"), ",")
			if lastLogsAddr == "" && LogsAddr == "" {
				log.Warningf("No log file found, Please check config.\n")
			} else if lastLogsAddr == "" && LogsAddr != "" {
				fmt.Printf("First run:\n--logs %s\n", LogsAddr)
				addrChan <- LogsAddr
				lastLogsAddr = LogsAddr
			} else if lastLogsAddr != "" && LogsAddr != "" && LogsAddr != lastLogsAddr {
				fmt.Printf("File change:\nlast: %s\nnow :%s\n", lastLogsAddr, LogsAddr)
				addrChan <- LogsAddr
				lastLogsAddr = LogsAddr
			}
			time.Sleep(time.Duration(*period)*time.Minute )
		}
	}()
	mtailReload()
}
func mtailReload() {
	var flag = 0
	var cmdMtail = &exec.Cmd{}
	for logsAddr := range addrChan {
		// Kill it, ignore the first time
		if flag != 0 {
			fmt.Println("process killed", time.Now())
			if err := cmdMtail.Process.Kill(); err != nil {
				log.Fatal("failed to kill process: ", err)
			}
		}
		flag++
		cmdMtail.Wait()
		fmt.Println("process start", time.Now())
		cmdMtail = exec.Command("mtail", "--port",*port,"--logs",logsAddr,"--progs", *progs)
		cmdMtail.Stdout = os.Stdout
		cmdMtail.Stderr = os.Stderr
		if err := cmdMtail.Start(); err != nil {
			log.Warning(err)
		}
	}
}
