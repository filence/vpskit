package app

import (
	"errors"
	"fmt"
	"net"

	"vpskit.local/vpskit/internal/model"
)

// instanceAdapter is the lifecycle boundary for a managed protocol instance.
// Schema 10 keeps protocol-specific fields in State for compatibility, while
// instance operations resolve through this registry rather than a CLI switch.
type instanceAdapter struct {
	Target   string
	ID       string
	Protocol string
	Network  string
	Service  string
	apply    func(*model.State, *model.Secrets, string, int) error
}

var registeredInstanceAdapters = []instanceAdapter{
	{Target: "reality", ID: model.InstanceAdapterXray, Protocol: model.InstanceProtocolVLESSReality, Network: model.InstanceNetworkTCP, Service: xrayServiceUnitName, apply: applyRealityInstanceStateChange},
	{Target: "hysteria2", ID: model.InstanceAdapterSingBox, Protocol: model.InstanceProtocolHysteria2, Network: model.InstanceNetworkUDP, Service: serviceUnitName, apply: applyHysteria2InstanceStateChange},
}

func resolveInstanceAdapter(target string) (instanceAdapter, error) {
	for _, adapter := range registeredInstanceAdapters {
		if adapter.Target == target {
			return adapter, nil
		}
	}
	return instanceAdapter{}, fmt.Errorf("unsupported instance %q; use reality or hysteria2", target)
}

func instanceAdapterDetails() []map[string]any {
	details := make([]map[string]any, 0, len(registeredInstanceAdapters))
	for _, adapter := range registeredInstanceAdapters {
		details = append(details, map[string]any{"target": adapter.Target, "id": adapter.ID, "protocols": []string{adapter.Protocol}, "network": adapter.Network, "service": adapter.Service})
	}
	return details
}

func instanceForAdapter(state model.State, target string) (model.ManagedInstance, bool) {
	adapter, err := resolveInstanceAdapter(target)
	if err != nil {
		return model.ManagedInstance{}, false
	}
	state.SynchronizeLegacyInstances()
	for _, instance := range state.Instances {
		if instance.Adapter == adapter.ID && instance.Protocol == adapter.Protocol && instance.Listen.Network == adapter.Network {
			return instance, true
		}
	}
	return model.ManagedInstance{}, false
}

func applyInstanceStateChange(state *model.State, secrets *model.Secrets, operation, target string, port int) error {
	adapter, err := resolveInstanceAdapter(target)
	if err != nil {
		return err
	}
	return adapter.apply(state, secrets, operation, port)
}

func applyRealityInstanceStateChange(state *model.State, secrets *model.Secrets, operation string, port int) error {
	if state.Reality.ID == "" {
		return errors.New("reality instance does not exist")
	}
	switch operation {
	case "enable":
		if state.Reality.Enabled {
			return errors.New("reality instance is already enabled")
		}
		state.Reality.Enabled = true
	case "disable":
		if !state.Reality.Enabled {
			return errors.New("reality instance is already disabled")
		}
		if !hasAnotherEnabledInstance(*state, state.Reality.ID) {
			return errors.New("refusing to disable the last enabled instance")
		}
		state.Reality.Enabled = false
	case "modify":
		if port == 0 {
			return nil
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid Reality TCP port: %d", port)
		}
		if port == state.Reality.ListenPort {
			return errors.New("reality listen port is unchanged")
		}
		state.Reality.ListenPort = port
	case "delete":
		if !hasAnotherEnabledInstance(*state, state.Reality.ID) {
			return errors.New("refusing to delete the last enabled instance")
		}
		state.Reality = model.RealityState{}
		secrets.RealityUUID, secrets.RealityPrivateKey = "", ""
	}
	return nil
}

func applyHysteria2InstanceStateChange(state *model.State, secrets *model.Secrets, operation string, port int) error {
	if state.Hysteria2.ID == "" {
		return errors.New("Hysteria2 instance does not exist")
	}
	switch operation {
	case "enable":
		if state.Hysteria2.Enabled {
			return errors.New("Hysteria2 instance is already enabled")
		}
		state.Hysteria2.Enabled = true
	case "disable":
		if !state.Hysteria2.Enabled {
			return errors.New("Hysteria2 instance is already disabled")
		}
		if !hasAnotherEnabledInstance(*state, state.Hysteria2.ID) {
			return errors.New("refusing to disable the last enabled instance")
		}
		state.Hysteria2.Enabled = false
	case "modify":
		if port == 0 {
			return nil
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid Hysteria2 UDP port: %d", port)
		}
		if port == state.Hysteria2.ListenPort {
			return errors.New("Hysteria2 listen port is unchanged")
		}
		state.Hysteria2.ListenPort = port
	case "delete":
		if !hasAnotherEnabledInstance(*state, state.Hysteria2.ID) {
			return errors.New("refusing to delete the last enabled instance")
		}
		state.Hysteria2 = model.Hysteria2State{}
		secrets.Hysteria2Password, secrets.Hysteria2ObfuscationPassword = "", ""
	}
	return nil
}

func hasAnotherEnabledInstance(state model.State, excludedID string) bool {
	state.SynchronizeLegacyInstances()
	for _, instance := range state.Instances {
		if instance.ID != excludedID && instance.Enabled {
			return true
		}
	}
	return false
}

func verifyNewInstancePortAvailable(adapter instanceAdapter, port int) error {
	if adapter.Network == model.InstanceNetworkTCP {
		listener, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
		if err != nil {
			return fmt.Errorf("TCP port %d is unavailable: %w", port, err)
		}
		return listener.Close()
	}
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("::"), Port: port})
	if err != nil {
		return fmt.Errorf("UDP port %d is unavailable: %w", port, err)
	}
	return listener.Close()
}
