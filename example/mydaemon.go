package main

import (
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

func (x *MyLittleDaemon) GetConfigObject() *MyLittleDaemonConfig {

	if x.Config == nil {
		x.Config = &MyLittleDaemonConfig{}
	}
	return x.Config
}

func (x *MyLittleDaemon) DaemonNewConfig() {
	x.Logger.Logf("MyLittleDaemon Got New Config %s", x.Config)
}

func (x *MyLittleDaemon) DaemonStart() {
	x.Logger.Log("MyLittleDaemon Starting")
}

func (x *MyLittleDaemon) DaemonDrain() {
	x.Logger.Log("MyLittleDaemon Draining....")
}

func (x *MyLittleDaemon) DaemonStop() {
	x.Logger.Log("MyLittleDaemon Stopping")
}

func main() {
	logger := &baselog.STDOutLog{}
	mld := &MyLittleDaemon{
		Logger: logger,
	}
	dmn := daemon.NewDefaultDaemon(mld)
	dmn.Config.Log = logger
	dmn.Config.EnableDrain = true
	dmn.Config.ConfigFile = "./config.json"
	dmn.Config.ConfigObject = mld.GetConfigObject()

	dmn.Run()
}
