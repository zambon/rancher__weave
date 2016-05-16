package net

import (
	"github.com/vishvananda/netlink"
)

type BridgeType int

const (
	WeaveBridgeName = "weave"
	DatapathName    = "datapath"

	None BridgeType = iota
	Bridge
	Fastdp
	BridgedFastdp
	Inconsistent
)

// Returns a string that is consistent with the weave script
func (t BridgeType) String() string {
	switch t {
	case None:
		return "none"
	case Bridge:
		return "bridge"
	case Fastdp:
		return "fastdp"
	case BridgedFastdp:
		return "bridged_fastdp"
	case Inconsistent:
		return "inconsistent"
	}
	return "unknown"
}

func DetectBridgeType(weaveBridgeName, datapathName string) BridgeType {
	bridge, _ := netlink.LinkByName(weaveBridgeName)
	datapath, _ := netlink.LinkByName(datapathName)

	switch {
	case bridge == nil && datapath == nil:
		return None
	case isBridge(bridge) && datapath == nil:
		return Bridge
	case isDatapath(bridge) && datapath == nil:
		return Fastdp
	case isDatapath(datapath) && isBridge(bridge):
		return BridgedFastdp
	default:
		return Inconsistent
	}
}

func isBridge(link netlink.Link) bool {
	_, isBridge := link.(*netlink.Bridge)
	return isBridge
}

func isDatapath(link netlink.Link) bool {
	switch link.(type) {
	case *netlink.GenericLink:
		return link.Type() == "openvswitch"
	case *netlink.Device:
		// Assume it's our openvswitch device, and the kernel has not been updated to report the kind.
		return true
	default:
		return false
	}
}
