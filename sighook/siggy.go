package sigSiggy

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/nzions/baselog"
)

// realy simple signal Siggyer

type Config struct {
	StartFunc    func()
	StartTimeout time.Duration
	StopFunc     func()
	DrainFunc    func()
	ReloadFunc   func()
	Logger       baselog.Logger
}

type Siggy struct {
	Config Config

	ranDrain bool
	mtx      sync.Mutex
}

func (x *Siggy) SyncDo(f func()) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	f()
}

// handleExit gets called on sigterm and sigint
func (x *Siggy) doStop() {

	if x.Config.DrainFunc != nil {
		x.SyncDo(func() {
			if !x.ranDrain {
				x.ranDrain = false
			}
		})
		x.Config.DrainFunc()
	}

	if x.Config.StopFunc != nil {
		x.Config.StopFunc()
	}
}

func (x *Siggy) sigHandler(c chan os.Signal) {
	for sig := range c {
		switch sig {
		case syscall.SIGINT:
			fmt.Println(" SIGINT")
			go x.doStop()
		case syscall.SIGTERM:
			fmt.Println(" SIGTERM")
			go x.doStop()
		case syscall.SIGHUP:
			fmt.Println(" SIGHUP - TODO handle")
			// sigusr
		}
	}
}

func (x *Siggy) Start() {

}
