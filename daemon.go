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
	DaemonStart()
	DaemonDrain()
	DaemonStop()
	DaemonNewConfig()
}

type Helpers struct {
	Daemon *Daemon
}

func (x *Helpers) Run() {
	x.Daemon.Run()
}

func (x *Helpers) Stop() {
	x.Daemon.Stop()
}

func (Helpers) DaemonDrain() {
	// implement this in your struct if you want to handle drain events
	// don't forget to set EnableDrain to true in Daemon Config
}

func (Helpers) DaemonNewConfig() {
	// implement this in your struct if you want to handle new config events
	// don't forget to set ConfigFile and ConfigObject in Daemon Config
}

func New(config Config) (d *Daemon, err error) {
	d = &Daemon{
		Config:    config,
		DaemonObj: config.DaemonObj,
	}

	if config.Log == nil {
		return nil, fmt.Errorf("log cannot be nil")
	}

	return d, nil
}

func NewDefaultDaemon(obj DaemonObj) (d *Daemon) {
	d, _ = New(NewDefaultConfig(obj))
	return
}

func NewDefaultConfig(obj DaemonObj) Config {
	return Config{
		ConfigCheckInterval: time.Second * 5,
		StartTimeout:        time.Second * 5,
		DrainTimeout:        time.Second * 5,
		ExitTimeout:         time.Millisecond * 500,
		Log:                 &baselog.STDOutLog{},
		DaemonObj:           obj,
	}
}

type Config struct {
	ConfigFile          string
	ConfigObject        interface{}
	Log                 baselog.Logger
	ConfigCheckInterval time.Duration
	EnableDrain         bool
	NoExit              bool
	StartTimeout        time.Duration
	DrainTimeout        time.Duration
	ExitTimeout         time.Duration
	DaemonObj           DaemonObj
}

type Daemon struct {
	DaemonObj DaemonObj
	Config    Config

	cfStat    os.FileInfo
	isRunning bool
	mtx       sync.RWMutex
	runLock   sync.RWMutex
	stopChan  chan int
}

// KeepRunning returns true if we are still running
func (x *Daemon) KeepRunning() (kr bool) {
	x.SyncDoRO(func() {
		kr = x.isRunning
	})
	return
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
	// set isrunning to false so helper goroutines can stop
	x.SyncDoRW(func() {
		x.isRunning = false
	})

	es := 0
	if x.Config.EnableDrain {
		x.SyncDoRW(func() {
			x.Config.EnableDrain = false
		})
		if !x.DoWithTimeout(x.Config.DrainTimeout, x.DaemonObj.DaemonDrain) {
			x.Config.Log.Errorf("Timed out waiting for Drain")
		}
	}

	// call stop
	if !x.DoWithTimeout(x.Config.ExitTimeout, x.DaemonObj.DaemonStop) {
		x.Config.Log.Errorf("Timed out waiting for Stop")
	}

	// send stop to chan to unlock the Start thread
	x.stopChan <- es
}

func (x *Daemon) sigHandler(c chan os.Signal) {
	for sig := range c {
		switch sig {
		case syscall.SIGINT:
			fmt.Println(" SIGINT")
			go x.handleExit()
		case syscall.SIGTERM:
			fmt.Println(" SIGTERM")
			go x.handleExit()
		case syscall.SIGHUP:
			fmt.Println(" SIGHUP - TODO handle")
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
	x.Config.Log.Tracef("ConfigWatcher stopped")
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

func (x *Daemon) TryExit(es int) {
	if !x.Config.NoExit {
		os.Exit(es)
	}
}

// RunForever
func (x *Daemon) Run() {
	if x.KeepRunning() {
		fmt.Println("Can't call Run again... Already running")
		return
	}

	// set the lock so other components can wait for us to finish
	x.runLock.Lock()

	// create the stop chan
	x.stopChan = make(chan int)

	// set state to running
	x.SyncDoRW(func() {
		x.isRunning = true
	})

	// load config, then watch
	if x.Config.ConfigFile != "" {
		x.loadConfigFile()
		go x.ConfigWatcher()
	}

	// catch ctrl + c and SIGHUP
	c := make(chan os.Signal, 1)
	go x.sigHandler(c)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// start
	if !x.DoWithTimeout(x.Config.StartTimeout, x.DaemonObj.DaemonStart) {
		x.Config.Log.Errorf("Timout waiting for Start")
		x.TryExit(1)
		return
	}

	x.Config.Log.Debugf("Daemon Started, PID %d", os.Getpid())

	// now we wait
	ec := <-x.stopChan
	x.Config.Log.Tracef("STOP recieved on stop chan Exit Code %d", ec)
	x.TryExit(ec)
}

func (x *Daemon) WaitForStop() {
	x.runLock.RLock()
	defer x.runLock.RUnlock()
}

// requests a stop, blocks until daemon has stopped
func (x *Daemon) Stop() {
	x.handleExit()
	x.WaitForStop()
}
