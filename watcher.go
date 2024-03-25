//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	fanapi "rootwatch/pkg/madmo/fanotify"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	errorBufSize   = 10
	eventBufSize   = 1000
	procFsFdInfo   = "/proc/self/fd/%d"
	procFsFilePath = "/proc/%v/%v"
)

func main() {
	nd, err := fanapi.Initialize(fanapi.FAN_CLASS_NOTIF|fanapi.FAN_NONBLOCK, unix.O_RDONLY|unix.O_NONBLOCK)
	if err != nil {
		log.Fatalf("can't init fanotify: %s", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if err = nd.Mark(
		fanapi.FAN_MARK_ADD|fanapi.FAN_MARK_MOUNT,
		fanapi.FAN_MODIFY|fanapi.FAN_ACCESS|fanapi.FAN_OPEN,
		-1, "/",
	); err != nil {
		log.Fatalf("failed to set mark: %s", err)
	}
	done := make(chan any, 1)
	log.Println("fanotify initialized...")

	go func(exitSig <-chan os.Signal) {
		log.Println("Watching for events...")
	exit:
		for {
			select {
			case <-exitSig:
				break exit
			default:
			}
			data, err := nd.GetEvent()
			if err != nil {
				log.Printf("failed to get event: %s", err)
				continue
			}
			log.Printf("received event, metadata: %+v", *data)
			log.Printf("file, fd: %d , name %s:", data.File.Fd(), data.File.Name())

			if (data.Mask & fanapi.FAN_Q_OVERFLOW) == fanapi.FAN_Q_OVERFLOW {
				log.Printf("overflow event")
				continue
			}

			if (data.Mask & fanapi.FAN_OPEN) == fanapi.FAN_OPEN {
				log.Printf("FAN.E - file open")
			}

			if (data.Mask & fanapi.FAN_ACCESS) == fanapi.FAN_ACCESS {
				log.Printf("FAN.E - file read")
			}

			if (data.Mask & fanapi.FAN_MODIFY) == fanapi.FAN_MODIFY {
				log.Printf("FAN.E - file write")
			}

			path, err := os.Readlink(fmt.Sprintf(procFsFdInfo, data.File.Fd()))
			if err != nil {
				log.Printf("ReadLink failed, fanotify.ev: %v, err: %s", data, err)
				continue
			}

			data.File.Close()

			log.Printf("file path => %v", path)
		}
		log.Println("Exiting...")
		done <- struct{}{}
	}(sigs)

	<-done
	log.Println("done!")

}
