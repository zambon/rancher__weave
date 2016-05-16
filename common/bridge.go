package common

import "fmt"
import "github.com/vishvananda/netlink"
import "github.com/weaveworks/weave/common/odp"
import weavenet "github.com/weaveworks/weave/net"

type BridgeConfig struct {
	DockerBridgeName string
	WeaveBridgeName  string
	DatapathName     string
	NoFastdp         bool
	NoBridgedFastdp  bool
	MTU              int
}

func CreateBridge(config *BridgeConfig) (weavenet.BridgeType, error) {
	bridgeType := weavenet.DetectBridgeType(config.WeaveBridgeName, config.DatapathName)

	if bridgeType == weavenet.None {
		bridgeType = weavenet.Bridge
		if !config.NoFastdp {
			bridgeType = weavenet.BridgedFastdp
			if !config.NoBridgedFastdp {
				bridgeType = weavenet.Fastdp
				config.DatapathName = config.WeaveBridgeName
			}
			odpSupported, err := odp.CreateDatapath(config.DatapathName)
			if err != nil {
				return weavenet.None, err
			}
			if !odpSupported {
				bridgeType = weavenet.Bridge
			}
		}

		var err error
		switch bridgeType {
		case weavenet.Bridge:
			err = initBridge(config)
		case weavenet.Fastdp:
			err = initFastdp(config)
		case weavenet.BridgedFastdp:
			err = initBridgedFastdp(config)
		default:
			err = fmt.Errorf("Cannot initialise bridge type %v", bridgeType)
		}
		if err != nil {
			return weavenet.None, err
		}

		configureIPTables(config)
	}

	if bridgeType == weavenet.Bridge {
		if err := weavenet.EthtoolTXOff(config.WeaveBridgeName); err != nil {
			return bridgeType, err
		}
	}

	if err := linkSetUpByName(config.WeaveBridgeName); err != nil {
		return bridgeType, err
	}

	if err := weavenet.ConfigureARPCache(config.WeaveBridgeName); err != nil {
		return bridgeType, err
	}

	return bridgeType, nil
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
		return true
	default:
		return false
	}
}

func initBridge(config *BridgeConfig) error {
	mac, err := weavenet.PersistentMAC()
	if err != nil {
		mac, err = weavenet.RandomMAC()
		if err != nil {
			return err
		}
	}

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = config.WeaveBridgeName
	linkAttrs.HardwareAddr = mac
	linkAttrs.MTU = config.MTU // TODO this probably doesn't work - see weave script
	netlink.LinkAdd(&netlink.Bridge{LinkAttrs: linkAttrs})

	return nil
}

func initFastdp(config *BridgeConfig) error {
	datapath, err := netlink.LinkByName(config.DatapathName)
	if err != nil {
		return err
	}
	return netlink.LinkSetMTU(datapath, config.MTU)
}

func initBridgedFastdp(config *BridgeConfig) error {
	if err := initFastdp(config); err != nil {
		return err
	}
	if err := initBridge(config); err != nil {
		return err
	}

	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "vethwe-bridge",
			MTU:  config.MTU},
		PeerName: "vethwe-datapath",
	}

	if err := netlink.LinkAdd(link); err != nil {
		return err
	}

	bridge, err := netlink.LinkByName(config.WeaveBridgeName)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetMasterByIndex(link, bridge.Attrs().Index); err != nil {
		return err
	}

	if err := odp.AddDatapathInterface(config.DatapathName, "vethwe-datapath"); err != nil {
		return err
	}

	if err := linkSetUpByName(config.DatapathName); err != nil {
		return err
	}

	return nil
}

func configureIPTables(config *BridgeConfig) error {
	return fmt.Errorf("Not implemented")
}

func linkSetUpByName(linkName string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(link)
}
