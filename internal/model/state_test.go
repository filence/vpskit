package model

import "testing"

func TestSynchronizeLegacyInstancesBuildsStableAdapterInventory(t *testing.T) {
	state := State{
		Reality:   RealityState{Enabled: true, ID: "reality-main", ListenPort: 443},
		Hysteria2: Hysteria2State{Enabled: false, ID: "hy2-backup", ListenPort: 8443},
	}
	state.SynchronizeLegacyInstances()
	if len(state.Instances) != 2 {
		t.Fatalf("unexpected instance count: %#v", state.Instances)
	}
	reality := state.Instances[0]
	if reality.ID != "reality-main" || reality.Protocol != InstanceProtocolVLESSReality || reality.Adapter != InstanceAdapterXray || !reality.Enabled || reality.Listen != (InstanceListen{Network: InstanceNetworkTCP, Port: 443}) {
		t.Fatalf("unexpected REALITY inventory entry: %#v", reality)
	}
	hy2, found := state.InstanceByID("hy2-backup")
	if !found || hy2.Protocol != InstanceProtocolHysteria2 || hy2.Adapter != InstanceAdapterSingBox || hy2.Enabled || hy2.Listen != (InstanceListen{Network: InstanceNetworkUDP, Port: 8443}) {
		t.Fatalf("unexpected Hysteria2 inventory entry: %#v", hy2)
	}
}

func TestSynchronizeLegacyInstancesOmitsUnconfiguredInbounds(t *testing.T) {
	state := State{}
	state.SynchronizeLegacyInstances()
	if len(state.Instances) != 0 {
		t.Fatalf("unconfigured state created inventory entries: %#v", state.Instances)
	}
}
