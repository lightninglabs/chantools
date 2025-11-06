package itest

import (
	"testing"
)

type testCase struct {
	name string
	fn   func(t *testing.T)
}

var testCases = []testCase{
	{
		name: "zombie recovery lnd <-> lnd",
		fn:   runZombieRecoveryLndLnd,
	},
	{
		name: "zombie recovery lnd <-> cln",
		fn:   runZombieRecoveryLndCln,
	},
	{
		name: "zombie recovery cln <-> lnd",
		fn:   runZombieRecoveryClnLnd,
	},
	{
		name: "zombie recovery cln <-> cln",
		fn:   runZombieRecoveryClnCln,
	},
	{
		name: "sweep remote closed lnd",
		fn:   runSweepRemoteClosedLnd,
	},
	{
		name: "sweep remote closed cln",
		fn:   runSweepRemoteClosedCln,
	},
	{
		name: "trigger force close lnd",
		fn:   runTriggerForceCloseLnd,
	},
	{
		name: "trigger force close cln",
		fn:   runTriggerForceCloseCln,
	},
	{
		name: "scb force close",
		fn:   runScbForceClose,
	},
}

// TestIntegration runs all integration test cases.
func TestIntegration(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, tc.fn)
	}
}
