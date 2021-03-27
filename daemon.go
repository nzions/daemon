package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nzions/baselog"
)

// start
// stop
// drain
// newconfig

type DaemonObj interface {
	GetDaemonConfig() Config
	DaemonStart()
	DaemonDrain()
	DaemonStop()
	DaemonNewConfig()
	// DaemonAnnounce(*Daemon) error
}

type Helpers struct {
	Daemon *Daemon
}

func (x *Helpers) Run() {
	x.Daemon.Run()
}

func (x *Helpers) Daemonize(dObj DaemonObj) (err error) {
	x.Daemon = &Daemon{
		DaemonObj: dObj,
	}
	return nil
}

func (Helpers) DaemonDrain() {
	// implement this in your struct if you want to handle drain events
	// don't forget to set EnableDrain to true in Daemon Config
}

func (Helpers) DaemonNewConfig() {
	// implement this in your struct if you want to handle new config events
	// don't forget to set ConfigFile and ConfigObject in Daemon Config
}

func NewConfig() Config {
	return Config{
		ConfigCheckInterval: time.Second * 5,
		StartTimeout:        time.Second * 5,
		DrainTimeout:        time.Second * 5,
		ExitTimeout:         time.Millisecond * 500,
	}
}

type Config struct {
	ConfigFile          string
	ConfigObject        interface{}
	Log                 baselog.Logger
	ConfigCheckInterval time.Duration
	EnableDrain         bool
	StartTimeout        time.Duration
	DrainTimeout        time.Duration
	ExitTimeout         time.Duration
}

type Daemon struct {
	DaemonObj DaemonObj
	Config    Config

	cfStat      os.FileInfo
	keepRunning bool
	exitSt      int
	mtx         sync.RWMutex
	wg          sync.WaitGroup
}

// this belons in daemon
func (x *Daemon) KeepRunning() (kr bool) {
	x.SyncDoRO(func() {
		kr = x.keepRunning
	})
	return
}

func (x *Daemon) RequestExit(exitSt int) {
	x.Config.Log.Debugf("exit requested")
	x.SyncDoRW(func() {
		x.keepRunning = false
	})
	x.wg.Done()
}

func (x *Daemon) DoWithTimeout(timeout time.Duration, f func()) bool {
	c := make(chan bool)
	go func() {
		f()
		c <- true
	}()

	select {
	case <-c:
		return true
	case <-time.After(timeout):
		return false
	}
}

// handleExit gets called on sigterm and sigint
func (x *Daemon) handleExit() {
	if x.Config.EnableDrain {
		x.SyncDoRW(func() {
			x.Config.EnableDrain = false
		})
		x.DoWithTimeout(x.Config.DrainTimeout, x.DaemonObj.DaemonDrain)
	}

	if !x.DoWithTimeout(x.Config.ExitTimeout, x.DaemonObj.DaemonStop) {
		x.Config.Log.Errorf("Timed out waiting for exit... hard exiting")
		x.RequestExit(1)
		return
	}

	if x.KeepRunning() {
		x.Config.Log.Bugf("Exit not called, exiting manually")
		x.RequestExit(1)
		return

	}
	x.RequestExit(1)
}

func (x *Daemon) sigHandler(c chan os.Signal) {
	for sig := range c {
		switch sig {
		case syscall.SIGINT:
			fmt.Println("SIGINT")
			go x.handleExit()
		case syscall.SIGTERM:
			fmt.Println("SIGTERM")
			go x.handleExit()
		case syscall.SIGHUP:
			fmt.Println("SIGHUP")
		}
	}
}

func (x *Daemon) loadConfigFile() {

	fStat, err := os.Stat(x.Config.ConfigFile)
	if err != nil {
		x.Config.Log.Errorf("Unable to Stat Config File: %s", err.Error())
		return
	}

	if x.cfStat != nil {
		if fStat.ModTime() == x.cfStat.ModTime() {
			x.Config.Log.Tracef("Config file not changed")
			return
		}
	}
	x.cfStat = fStat

	jBytes, err := ioutil.ReadFile(x.Config.ConfigFile)
	if err != nil {
		x.Config.Log.Errorf("Unable to Read Config: %s", err.Error())
		return
	}

	if err = json.Unmarshal(jBytes, x.Config.ConfigObject); err != nil {
		x.Config.Log.Errorf("Config File Decode Error: %s", err.Error())
		return
	}
	x.DaemonObj.DaemonNewConfig()
}

func (x *Daemon) ConfigWatcher() {
	for x.KeepRunning() {
		x.loadConfigFile()
		time.Sleep(x.Config.ConfigCheckInterval)
	}
}

func (x *Daemon) SyncDoRO(f func()) {
	x.mtx.RLock()
	defer x.mtx.RUnlock()
	f()
}

func (x *Daemon) SyncDoRW(f func()) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	f()
}

// RunForever
func (x *Daemon) Run() {
	// announce myself
	// if err := x.DaemonObj.DaemonAnnounce(x); err != nil {
	// 	x.Config.Log.Errorf("announce rejected %s", err)
	// 	os.Exit(1)
	// }

	if x.KeepRunning() {
		fmt.Println("Already running")
		return
	}

	// get config
	x.Config = x.DaemonObj.GetDaemonConfig()

	x.SyncDoRW(func() {
		x.keepRunning = true
	})

	// ensure the logger isn't nil
	if x.Config.Log == nil {
		fmt.Println("Error no logger")
		os.Exit(1)
	}

	// load config, then watch
	if x.Config.ConfigFile != "" {
		x.loadConfigFile()
		go x.ConfigWatcher()
	}

	// catch ctrl + c and SIGHUP
	c := make(chan os.Signal, 1)
	go x.sigHandler(c)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// start the daemon
	x.Config.Log.Debugf("Daemon Started, PID %d", os.Getpid())

	// start
	if !x.DoWithTimeout(x.Config.StartTimeout, x.DaemonObj.DaemonStart) {
		x.Config.Log.Errorf("Timout waiting for Start")
		os.Exit(1)
	}

	// now we wait
	x.wg.Add(1)
	x.wg.Wait()
	os.Exit(x.exitSt)
}
