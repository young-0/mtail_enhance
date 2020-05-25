// __author__ = 'jean'
// Created by zhang at 2020/4/20 10:53
// Process by go

package main

import (
	"context"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	port    = flag.String("port", "3903", "HTTP port to listen on.")
	progs   = flag.String("progs", "/etc/mtail/", "Name of the directory containing mtail programs")
	period  = flag.Int("period", 1, "logs file check period, Unit is minutes")
	isDebug = flag.Bool("debug", false, "Output verbose debug information")
)

func init() {
	flag.Var(&logList, "logs", "List of log files to monitor, separated by commas.  This flag may be specified multiple times. You can Use fuzzy matching in the path, like --logs \"/Path/*/to/*.log\"")
}

var (
	addrChan = make(chan string, 1)
)

func main() {
	flag.Parse()
	if *isDebug {
		log.SetLevel(log.DebugLevel)
		log.Debugln("Enabling debug output")
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	fmt.Printf("parameter logList：\n")
	for n, a := range logList {
		fmt.Printf("%d\t%s\n", n, a)
	}
	go monitorConfigFiles()
	mtailReload()
}
func removeDuplicateElement(l []string) []string {
	r := make([]string, 0, len(l))
	temp := map[string]int{}
	for _, item := range l {
		if _, ok := temp[item]; !ok {
			temp[item] = 0
			r = append(r, item)
		}
	}
	return r
}
func getConfigFiles(path string) (configFiles []string, err error) {
	// Create empty slice for config file list
	configFiles = make([]string, 0)
	files, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	configFiles = append(configFiles, files...)
	return configFiles, nil
}
func monitorConfigFiles() {
	var lastConfigFileStr string
	for {
		var logFiles = make([]string, 0)
		for _, logAddr := range logList {
			files, err := getConfigFiles(logAddr)
			if err != nil {
				log.Errorf("logAddr %s parse err: %s\n", logAddr, err)
				continue
			}
			logFiles = append(logFiles, files...)
		}
		logFiles = removeDuplicateElement(logFiles)
		sort.Strings(logFiles)
		configFilesStr := strings.Join(logFiles, ",")
		if len(configFilesStr) == 0 && len(lastConfigFileStr) == 0 {
			log.Warningf("No log file found, Please check config.\n")
		} else if len(configFilesStr) > 0 && len(lastConfigFileStr) == 0 {
			log.Info("logFiles first search\n")
			for n, a := range logFiles {
				log.Debugf("%d\t%s\n", n, a)
			}
			addrChan <- configFilesStr
			lastConfigFileStr = configFilesStr
		} else if len(configFilesStr) > 0 && len(lastConfigFileStr) > 0 && configFilesStr != lastConfigFileStr {
			log.Info("logFiles change\n")
			for n, a := range logFiles {
				log.Debugf("%d\t%s\n", n, a)
			}
			addrChan <- configFilesStr
			lastConfigFileStr = configFilesStr
		} else {
			log.Debugf("Configuration file has not changed\n")
		}
		time.Sleep(time.Duration(*period) * time.Minute)
	}
}
func mtailReload() {
	configFilesStr := <-addrChan
	for {
		ctx, cancel := context.WithCancel(context.Background())
		cmdMtail := exec.CommandContext(ctx, "mtail", "--port", *port,
			"--progs", *progs, "--override_timezone", "Local", "--disable_fsnotify",
			"--logs", configFilesStr)
		cmdMtail.Stdout = os.Stdout
		cmdMtail.Stderr = os.Stderr
		if err := cmdMtail.Start(); err != nil {
			log.Warning(err)
		}
		configFilesStr = <-addrChan
		cancel()
		if err := cmdMtail.Wait(); err != nil {
			log.Errorf("exec wait info: %v\n", err)
		}
	}
}
