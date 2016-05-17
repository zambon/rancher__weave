package common

import "fmt"
import "net"
import "github.com/vishvananda/netlink"
import "github.com/weaveworks/weave/common/odp"
import weavenet "github.com/weaveworks/weave/net"
import "github.com/coreos/go-iptables/iptables"

type BridgeConfig struct {
	DockerBridgeName string
	WeaveBridgeName  string
	DatapathName     string
	NoFastdp         bool
	NoBridgedFastdp  bool
	MTU              int
	Port             int
}

func CreateBridge(config *BridgeConfig) (weavenet.BridgeType, error) {
	var e ErrorHandler
	bridgeType := weavenet.DetectBridgeType(config.WeaveBridgeName, config.DatapathName)

	if bridgeType == weavenet.None {
		bridgeType = weavenet.Bridge
		if !config.NoFastdp {
			bridgeType = weavenet.BridgedFastdp
			if config.NoBridgedFastdp {
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

		switch bridgeType {
		case weavenet.Bridge:
			e.Err = initBridge(config)
		case weavenet.Fastdp:
			e.Err = initFastdp(config)
		case weavenet.BridgedFastdp:
			e.Err = initBridgedFastdp(config)
		default:
			e.Err = fmt.Errorf("Cannot initialise bridge type %v", bridgeType)
		}

		e.Do(func() { e.Err = configureIPTables(config) })
	}

	if bridgeType == weavenet.Bridge {
		e.Do(func() { e.Err = weavenet.EthtoolTXOff(config.WeaveBridgeName) })
	}

	e.Do(func() { e.Err = linkSetUpByName(config.WeaveBridgeName) })
	e.Do(func() { e.Err = weavenet.ConfigureARPCache(config.WeaveBridgeName) })

	return bridgeType, e.Err
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
	var e ErrorHandler
	var mac net.HardwareAddr
	/* Derive the bridge MAC from the system (aka bios) UUID, or,
	   failing that, the hypervisor UUID. Elsewhere we in turn derive
	   the peer name from that, which we want to be stable across
	   reboots but otherwise unique. The system/hypervisor UUID fits
	   that bill, unlike, say, /etc/machine-id, which is often
	   identical on VMs created from cloned filesystems. If we cannot
	   determine the system/hypervisor UUID we just generate a random MAC. */
	e.Do(func() { mac, e.Err = weavenet.PersistentMAC("/sys/class/dmi/id/product_uuid") })
	e.IfErr(func() { mac, e.Err = weavenet.PersistentMAC("/sys/hypervisor/uuid") })
	e.IfErr(func() { mac, e.Err = weavenet.RandomMAC() })

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = config.WeaveBridgeName
	linkAttrs.HardwareAddr = mac
	mtu := config.MTU
	if mtu == 0 {
		mtu = 65535
	}
	linkAttrs.MTU = mtu // TODO this probably doesn't work - see weave script
	e.Do(func() { e.Err = netlink.LinkAdd(&netlink.Bridge{LinkAttrs: linkAttrs}) })

	return e.Err
}

func initFastdp(config *BridgeConfig) error {
	datapath, err := netlink.LinkByName(config.DatapathName)
	if err != nil {
		return err
	}
	mtu := config.MTU
	if mtu == 0 {
		/* GCE has the lowest underlay network MTU we're likely to encounter on
		   a local network, at 1460 bytes.  To get the overlay MTU from that we
		   subtract 20 bytes for the outer IPv4 header, 8 bytes for the outer
		   UDP header, 8 bytes for the vxlan header, and 14 bytes for the inner
		   ethernet header. */
		mtu = 1410
	}
	return netlink.LinkSetMTU(datapath, mtu)
}

func initBridgedFastdp(config *BridgeConfig) error {
	var e ErrorHandler
	e.Do(func() { e.Err = initFastdp(config) })
	e.Do(func() { e.Err = initBridge(config) })

	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "vethwe-bridge",
			MTU:  config.MTU},
		PeerName: "vethwe-datapath",
	}

	e.Do(func() { e.Err = netlink.LinkAdd(link) })
	var bridge netlink.Link
	e.Do(func() { bridge, e.Err = netlink.LinkByName(config.WeaveBridgeName) })
	e.Do(func() { e.Err = netlink.LinkSetMasterByIndex(link, bridge.Attrs().Index) })
	e.Do(func() { e.Err = netlink.LinkSetUp(link) })
	e.Do(func() { e.Err = linkSetUpByName(link.PeerName) })
	e.Do(func() { e.Err = odp.AddDatapathInterface(config.DatapathName, link.PeerName) })
	e.Do(func() { e.Err = linkSetUpByName(config.DatapathName) })

	if e.Err != nil {
		return e.Err
	}
	return nil
}

// Add a rule to iptables, if it doesn't exist already
func addIPTablesRule(e *ErrorHandler, ipt *iptables.IPTables, table, chain string, rulespec ...string) {
	var exists bool
	e.Do(func() { exists, e.Err = ipt.Exists(table, chain, rulespec...) })
	if !exists {
		e.Do(func() { e.Err = ipt.Append(table, chain, rulespec...) })
	}
}

func configureIPTables(config *BridgeConfig) error {
	var e ErrorHandler
	var ipt *iptables.IPTables
	e.Do(func() { ipt, e.Err = iptables.New() })
	if config.WeaveBridgeName != config.DockerBridgeName {
		e.Do(func() {
			e.Err = ipt.Insert("filter", "FORWARD", 1, "-i", config.DockerBridgeName, "-o", config.WeaveBridgeName, "-j", "DROP")
		})
	}

	var dockerBridgeIP net.IP
	e.Do(func() { dockerBridgeIP, e.Err = DeviceIP(config.DockerBridgeName) })

	// forbid traffic to the Weave port from other containers
	addIPTablesRule(&e, ipt, "filter", "INPUT", "-i", config.DockerBridgeName, "-p", "tcp", "--dst", dockerBridgeIP.String(), "--dport", fmt.Sprint(config.Port), "-j", "DROP")
	addIPTablesRule(&e, ipt, "filter", "INPUT", "-i", config.DockerBridgeName, "-p", "udp", "--dst", dockerBridgeIP.String(), "--dport", fmt.Sprint(config.Port), "-j", "DROP")
	addIPTablesRule(&e, ipt, "filter", "INPUT", "-i", config.DockerBridgeName, "-p", "udp", "--dst", dockerBridgeIP.String(), "--dport", fmt.Sprint(config.Port+1), "-j", "DROP")

	// let DNS traffic to weaveDNS, since otherwise it might get blocked by the likes of UFW
	addIPTablesRule(&e, ipt, "filter", "INPUT", "-i", config.DockerBridgeName, "-p", "udp", "--dport", "53", "-j", "ACCEPT")
	addIPTablesRule(&e, ipt, "filter", "INPUT", "-i", config.DockerBridgeName, "-p", "tcp", "--dport", "53", "-j", "ACCEPT")

	// Work around the situation where there are no rules allowing traffic
	// across our bridge. E.g. ufw
	addIPTablesRule(&e, ipt, "filter", "FORWARD", "-i", config.WeaveBridgeName, "-o", config.WeaveBridgeName, "-j", "ACCEPT")

	// create a chain for masquerading
	e.Do(func() { e.Err = ipt.NewChain("nat", "WEAVE") })
	addIPTablesRule(&e, ipt, "nat", "POSTROUTING", "-j", "WEAVE")

	return e.Err
}

func linkSetUpByName(linkName string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(link)
}
