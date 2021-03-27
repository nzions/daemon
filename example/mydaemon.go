package main

import (
	"time"

	"github.com/nzions/baselog"
	"github.com/nzions/daemon"
)

type MyLittleDaemonConfig struct {
	Foo string
	Baz string
}

type MyLittleDaemon struct {
	daemon.Helpers

	Config *MyLittleDaemonConfig
	Logger baselog.Logger
}

func (x *MyLittleDaemon) GetDaemonConfig() daemon.Config {
	dc := daemon.NewConfig()
	dc.EnableDrain = true
	dc.Log = x.Logger

	dc.ConfigFile = "./config.json"
	x.Config = &MyLittleDaemonConfig{}
	dc.ConfigObject = x.Config
	return dc
}

func (x *MyLittleDaemon) DaemonNewConfig() {
	x.Logger.Logf("Got New Config %s", x.Config)
}

func (x *MyLittleDaemon) DaemonStart() {
	x.Logger.Log("Daemon Starting")
}

func (x *MyLittleDaemon) DaemonDrain() {
	x.Logger.Log("Daemon Draining....")
	time.Sleep(5 * time.Second)
}

func (x *MyLittleDaemon) DaemonStop() {
	x.Logger.Log("Daemon Stopping")
}

func main() {
	mld := MyLittleDaemon{
		Logger: &baselog.STDOutLog{},
	}
	mld.Daemonize(&mld)
	mld.Run()
}
