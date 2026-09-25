/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/vishvananda/netlink"
)

// Capitalized fields so they can be marshaled/unmarshaled properly
type Mapping struct {
	HostInterfaceName string   `json:"hostInterfaceName"`
	HostInterfaceIPs  []string `json:"hostInterfaceIPs"`
	State             string   `json:"state,omitempty"`
}

type InterfaceIPMappings map[string]*Mapping

type AMDHostDeviceCNI struct {
	interfaceIPMappings InterfaceIPMappings
	mu                  sync.Mutex
}

func (a *AMDHostDeviceCNI) loadInterfaceIPMappings() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := os.ReadFile(mappingsFilePath)
	if os.IsNotExist(err) {
		emptyMappings := make(InterfaceIPMappings)
		jsonData, err := json.MarshalIndent(emptyMappings, "", "  ")
		if err != nil {
			log.Printf("failed to marshal empty mappings, err: %v", err)
			return err
		}

		if err := os.WriteFile(mappingsFilePath, jsonData, 0644); err != nil {
			log.Printf("failed to create mappings file, err: %v", err)
			return err
		}
		log.Printf("created mappings file: %s", mappingsFilePath)
		return nil
	}
	if err != nil {
		log.Printf("failed to read mappings file, err: %v", err)
		return err
	}

	if err := json.Unmarshal(data, &a.interfaceIPMappings); err != nil {
		log.Printf("failed to unmarshal mappings, err: %v", err)
		return err
	}

	return nil
}

func (a *AMDHostDeviceCNI) addInterfaceIPMapping(podIfName, hostIfName, state string, addresses []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.interfaceIPMappings == nil {
		a.interfaceIPMappings = make(InterfaceIPMappings)
	}
	a.interfaceIPMappings[podIfName] = &Mapping{
		HostInterfaceName: hostIfName,
		HostInterfaceIPs:  addresses,
		State:             state,
	}

	return a.saveInterfaceIPMappings()
}

func (a *AMDHostDeviceCNI) getInterfaceIPMapping(podIfName string) (*Mapping, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if m, ok := a.interfaceIPMappings[podIfName]; ok {
		return m, true
	}

	return nil, false
}

func (a *AMDHostDeviceCNI) removeInterfaceIPMapping(podIfName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.interfaceIPMappings == nil {
		return fmt.Errorf("IP mappings not loaded")
	}

	delete(a.interfaceIPMappings, podIfName)

	return a.saveInterfaceIPMappings()
}

func (a *AMDHostDeviceCNI) saveInterfaceIPMappings() error {
	jsonData, err := json.MarshalIndent(a.interfaceIPMappings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mappingsFilePath, jsonData, 0644)
}

// configureHostInterface configures the host interface with the provided IP address and state.
func (a *AMDHostDeviceCNI) configureHostInterface(ifName, state string, addresses []string) error {
	// 1. Get the interface link.
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		log.Printf("failed to get the host interface link, err: %v", err)
		return err
	}

	// 2. Add the IP address to the interface.
	if err := a.assignIPAddresses(link, addresses); err != nil {
		log.Printf("failed to add IP addresses %v to interface %s, err: %v", addresses, ifName, err)
		return err
	}

	// 3. Set the link state of the interface.
	if err := a.setInterfaceState(link, state); err != nil {
		log.Printf("failed to set link state for interface %s to %s, err: %v", ifName, state, err)
		return err
	}

	return nil
}

// assignIPAddresses assigns one or more IP addresses to the specified interface.
func (a *AMDHostDeviceCNI) assignIPAddresses(link netlink.Link, ipAddrCIDRs []string) error {
	ifName := link.Attrs().Name

	for _, ipAddrCIDR := range ipAddrCIDRs {
		if ipAddrCIDR == "<nil>" || ipAddrCIDR == "" {
			log.Printf("no IP address provided for interface %s, skipping.", ifName)
			continue
		}

		// 1. Parse the IP address and CIDR.
		addr, ipNet, err := net.ParseCIDR(ipAddrCIDR)
		if err != nil {
			log.Printf("failed to parse IP address and CIDR %s, err: %v", ipAddrCIDR, err)
			return err
		}

		// 2. Create the netlink address object.
		netlinkAddr := &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   addr,
				Mask: ipNet.Mask,
			},
		}

		// 3. Determine if we're dealing with IPv4 or IPv6
		var family int
		if addr.To4() != nil {
			family = netlink.FAMILY_V4
		} else {
			family = netlink.FAMILY_V6
		}

		// 4. Check if the IP already exists on the interface.
		existingAddrs, err := netlink.AddrList(link, family)
		if err != nil {
			log.Printf("failed to list existing addresses on interface %s, err: %v", ifName, err)
			return err
		}

		ipExists := false
		for _, existingAddr := range existingAddrs {
			if existingAddr.IPNet.IP.Equal(netlinkAddr.IPNet.IP) && existingAddr.IPNet.Mask.String() == netlinkAddr.IPNet.Mask.String() {
				log.Printf("IP address %s already exists on interface %s, skipping.", ipAddrCIDR, ifName)
				ipExists = true
				break
			}
		}

		// 5. Add the IP address to the interface.
		if !ipExists {
			if err := netlink.AddrAdd(link, netlinkAddr); err != nil {
				log.Printf("failed to add IP address %s to interface %s: %v", ipAddrCIDR, ifName, err)
				return err
			}
			log.Printf("Added IP address %s to interface %s.", ipAddrCIDR, ifName)
		}
	}

	return nil
}

// setInterfaceState sets the state of the interface to UP or DOWN.
func (a *AMDHostDeviceCNI) setInterfaceState(link netlink.Link, state string) error {
	// it will be down by default, so no need to set it to DOWN explicitly
	if state == InterfaceUP {
		ifName := link.Attrs().Name
		log.Printf("Setting interface %s state to UP", ifName)
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring interface %s up: %v", ifName, err)
		}
	}

	return nil
}

func (a *AMDHostDeviceCNI) getInterfaceNameFromRequest(stdinData []byte) (map[string]interface{}, string, error) {
	var cniConf map[string]interface{}
	err := json.Unmarshal(stdinData, &cniConf)
	if err != nil {
		log.Printf("error unmarshalling json, err: %v", err)
		return cniConf, "", err
	}
	pciID, found := cniConf["pciBusID"].(string)
	if !found {
		log.Printf("failed to get PCI BUS ID from the request")
		return cniConf, "", fmt.Errorf("failed to get PCI BUS ID")
	}

	// Get the actual host interface using the PCI address
	netPath := filepath.Join("/sys/bus/pci/devices", pciID, "net")
	if _, err := os.Stat(netPath); os.IsNotExist(err) {
		return cniConf, "", fmt.Errorf("directory not found: %s", netPath)
	}
	interfaces, err := os.ReadDir(netPath)
	if err != nil {
		return cniConf, "", fmt.Errorf("failed to read directory %s: %v", netPath, err)
	}
	if len(interfaces) > 0 {
		// Assuming there is only one network interface per PCI device in this context
		log.Printf("found interface:%s from PCI ID: %s", interfaces[0].Name(), pciID)
		return cniConf, interfaces[0].Name(), nil
	}

	return cniConf, "", fmt.Errorf("no network interface found under %s", netPath)
}

// computeInjectedRoutes derives one static-IPAM route per IPv4 address on the interface.
//
//	dst     = local IP & CIDRMask(dstPrefixLen)  (dstPrefixLen=0 yields a default route)
//	nexthop = netlink peer IP when the on-link prefix is /31 and a.Peer is set;
//	          otherwise XOR of the local IP for /31; network address + offset otherwise.
//
// Peer-style addresses (IFA_LOCAL as /32, real prefix and peer in a.Peer) use a.Peer.Mask
// as the on-link prefix so /31 detection and host-bit checks see the real subnet.
// Per-address validation is logged and the offending address skipped.
func computeInjectedRoutes(addrs []netlink.Addr, offset, dstPrefixLen int, ifName string) []map[string]interface{} {
	dstMask := net.CIDRMask(dstPrefixLen, 32)
	var routes []map[string]interface{}
	for _, a := range addrs {
		if a.IPNet == nil || a.IP == nil {
			continue
		}
		mask := onLinkMask(a)
		ones, bits := mask.Size()
		if bits != 32 {
			continue // not an IPv4 mask
		}
		// The destination prefix must be a strict supernet of the interface prefix,
		// otherwise the route is a connected-subnet route (or narrower) and not useful.
		if dstPrefixLen >= ones {
			log.Printf("skipping injected route for %s: routeDstPrefixLen %d must be less than interface prefix length %d", a.IPNet, dstPrefixLen, ones)
			continue
		}
		var nexthop net.IP
		if ones == 31 {
			nexthop = peerFor31(a)
			if nexthop == nil {
				log.Printf("skipping injected route for %s: could not derive /31 peer", a.IPNet)
				continue
			}
		} else {
			// The offset must fit within the on-link subnet's host bits.
			hostBits := uint(bits - ones)
			if uint64(offset) >= uint64(1)<<hostBits {
				log.Printf("skipping injected route for %s: nexthopNetAddrOffset %d does not fit in the /%d host bits", a.IPNet, offset, ones)
				continue
			}
			nh, err := addOffsetToIP(a.IP.Mask(mask), offset)
			if err != nil {
				log.Printf("failed to compute injected nexthop for %s: %v", a.IPNet, err)
				continue
			}
			nexthop = nh
		}
		dstNet := &net.IPNet{IP: a.IP.Mask(dstMask), Mask: dstMask}
		routes = append(routes, map[string]interface{}{
			"dst": dstNet.String(),
			"gw":  nexthop.String(),
		})
		log.Printf("computed injected route: %s via %s for interface %s", dstNet, nexthop, ifName)
	}
	return routes
}

// onLinkMask is the subnet mask that describes the connected prefix. When netlink
// stores IFA_LOCAL as /32 and the real prefix on a.Peer, the peer mask wins.
func onLinkMask(a netlink.Addr) net.IPMask {
	if a.Peer != nil && a.Peer.Mask != nil {
		return a.Peer.Mask
	}
	return a.Mask
}

// formatOnLinkCIDR is the local IPv4 address with the on-link prefix. Peer-style
// netlink (/32 local + prefix on a.Peer) becomes e.g. 10.1.2.5/31 so static IPAM
// and SBR see a CIDR that contains the peer gateway. IPv6 is not handled here.
func formatOnLinkCIDR(a netlink.Addr) string {
	if a.IPNet == nil || a.IP == nil || a.IP.To4() == nil {
		return ""
	}
	mask := onLinkMask(a)
	ones, bits := mask.Size()
	if bits != 32 || ones < 0 {
		return a.IPNet.String()
	}
	return (&net.IPNet{IP: a.IP, Mask: mask}).String()
}

// peerFor31 returns the other end of a /31. Prefer the kernel-provided peer IP
// when present; otherwise XOR the last bit of the local IPv4 address.
func peerFor31(a netlink.Addr) net.IP {
	if a.Peer != nil {
		if ip4 := a.Peer.IP.To4(); ip4 != nil {
			return ip4
		}
	}
	ip4 := a.IP.To4()
	if ip4 == nil {
		return nil
	}
	peer := make(net.IP, 4)
	copy(peer, ip4)
	peer[3] ^= 1
	return peer
}

// addOffsetToIP returns a new IPv4 address with a non-negative offset added to the
// network address. Negative offsets and uint32 overflow are rejected.
func addOffsetToIP(ip net.IP, offset int) (net.IP, error) {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("not an IPv4 address: %s", ip)
	}
	if offset < 0 {
		return nil, fmt.Errorf("negative offset not supported: %d", offset)
	}
	n := binary.BigEndian.Uint32(ip4)
	sum := uint64(n) + uint64(offset)
	if sum > math.MaxUint32 {
		return nil, fmt.Errorf("offset %d overflows IPv4 address %s", offset, ip)
	}
	result := make(net.IP, 4)
	binary.BigEndian.PutUint32(result, uint32(sum))
	return result, nil
}

// parseIntField extracts an integer NAD field where JSON numbers arrive as float64.
// It returns (value, present, err): present is false when the key is absent; err is
// non-nil when the key is set but is not a whole number.
func parseIntField(conf map[string]interface{}, key string) (int, bool, error) {
	v, ok := conf[key]
	if !ok {
		return 0, false, nil
	}
	f, ok := v.(float64)
	if !ok {
		return 0, true, fmt.Errorf("%s must be a number, got %T", key, v)
	}
	if f != math.Trunc(f) {
		return 0, true, fmt.Errorf("%s must be an integer, got %v", key, f)
	}
	return int(f), true, nil
}
