package daemon_test

import (
	"testing"
	"time"

	"github.com/nzions/daemon"
	"github.com/stretchr/testify/assert"
)

type daemonTester struct {
	daemon.Helpers

	DaemonStartST time.Duration
	DaemonDrainST time.Duration
	DaemonStopST  time.Duration

	RunLog       string
	DaemonConfig daemon.Config
}

func (x *daemonTester) Reset() {
	x.RunLog = ""
	x.DaemonStartST = 0
	x.DaemonDrainST = 0
	x.DaemonStopST = 0
}

func (x *daemonTester) DaemonNewConfig() {
	x.RunLog = x.RunLog + " DaemonNewConfig"
}

func (x *daemonTester) DaemonStart() {
	x.RunLog = x.RunLog + " DaemonStart"
	time.Sleep(x.DaemonStartST)
}

func (x *daemonTester) DaemonDrain() {
	x.RunLog = x.RunLog + " DaemonDrain"
	time.Sleep(x.DaemonDrainST)
}

func (x *daemonTester) DaemonStop() {
	x.RunLog = x.RunLog + " DaemonStop"
	time.Sleep(x.DaemonStopST)
}

func TestInit(t *testing.T) {
	dmn := daemon.NewDefaultDaemon(&daemonTester{})
	assert.NotNil(t, dmn)

	go dmn.Run()
	dmn.Stop()
}
