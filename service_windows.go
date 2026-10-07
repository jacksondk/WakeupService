//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "WakeupService"

type winService struct{ cfgPath string }

func (s *winService) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}

	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- run(s.cfgPath, stop) }()

	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-done:
			// run exited on its own (bad config, port in use, ...).
			if err != nil {
				log.Printf("Service stopped: %v", err)
				return false, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				close(stop)
				if err := <-done; err != nil {
					log.Printf("Shutdown error: %v", err)
				}
				return false, 0
			}
		}
	}
}

// runAsService runs under the SCM when launched as a Windows service. It
// returns handled=false when started from a console.
func runAsService(cfgPath string) (bool, error) {
	isSvc, err := svc.IsWindowsService()
	if err != nil || !isSvc {
		return false, err
	}

	// No console under the SCM: log to a file next to the executable.
	if exe, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(exe), "wakeupservice.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			log.SetOutput(f)
		}
	}

	return true, svc.Run(serviceName, &winService{cfgPath: cfgPath})
}
