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
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/vishvananda/netlink"
)

const (
	InterfaceUP   = "UP"
	InterfaceDOWN = "DOWN"
)

var (
	amdHostDeviceCNI = AMDHostDeviceCNI{}
)

func cmdAdd(args *skel.CmdArgs) error {
	cniConf, hostInterfaceName, err := amdHostDeviceCNI.getInterfaceNameFromRequest(args.StdinData)
	if err != nil {
		log.Printf("error getting interface name from request, err: %v", err)
		return err
	}

	deviceID, err := getDeviceIDFromArgs(cniConf)
	if err != nil {
		log.Printf("failed to get deviceID, err: %v", err)
		return err
	}
	// 1. Get the IP from the host device.
	args.IfName = hostInterfaceName
	devLink, err := netlink.LinkByName(hostInterfaceName)
	if err != nil {
		log.Printf("cannot find link %s, err: %v", hostInterfaceName, err)
		return err
	}

	// 5. Fetch list of addresses to be passed as a static config
	var addresses []map[string]interface{}
	var addrs []string

	// Attempt to get IPv4 addresses
	addrsV4, errV4 := netlink.AddrList(devLink, netlink.FAMILY_V4)
	if errV4 != nil {
		log.Printf("error getting IPv4 address for %s: %v", hostInterfaceName, errV4)
	} else {
		if len(addrsV4) > 0 {
			for _, a := range addrsV4 {
				addr := a.IPNet.String()
				addresses = append(addresses, map[string]interface{}{
					"address": addr,
				})
				addrs = append(addrs, addr)
			}
		}
	}

	// Attempt to get IPv6 addresses
	addrsV6, errV6 := netlink.AddrList(devLink, netlink.FAMILY_V6)
	if errV6 != nil {
		log.Printf("error getting IPv6 address for %s: %v", hostInterfaceName, errV6)
	} else {
		// Append IPv6 addresses if any were found
		if len(addrsV6) > 0 {
			for _, a := range addrsV6 {
				// Skip IPv6 link-local addresses (fe80::/10)
				if a.IP.IsLinkLocalUnicast() {
					continue
				}
				addr := a.IPNet.String()
				addresses = append(addresses, map[string]interface{}{
					"address": addr,
				})
				addrs = append(addrs, addr)
			}
		}
	}

	// 3. Create static IPAM config.
	if len(addresses) > 0 {
		log.Printf("got IP addresses %v from host interface %s", addrs, hostInterfaceName)
		ipamConf := map[string]interface{}{
			"ipam": map[string]interface{}{
				"type":      "static",
				"addresses": addresses,
			},
		}

		// Construct the full CNI configuration with static IPAM config to be sent to the host-device CNI plugin
		if _, ok := cniConf["ipam"].(map[string]interface{}); !ok {
			cniConf["ipam"] = ipamConf["ipam"]
		}
	} else {
		log.Printf("failed to get IP address or none found from host interface %s,err: %v", hostInterfaceName, err)
	}

	cniConfBytes, err := json.Marshal(cniConf)
	if err != nil {
		log.Printf("failed to marshal CNI config, err: %v", err)
		return err
	}

	// 4. Execute the host-device plugin with the modified CNI config
	executeResult, err := execPlugin("host-device", "ADD", cniConfBytes, args, true)
	if err != nil {
		log.Printf("failed to execute host-device plugin %v: %v", string(cniConfBytes), err)
		return err
	}

	// 5. Store this mapping (Interface->IP) in a local mappings file;
	// "null" will be stored if no IP address was found.
	if err := amdHostDeviceCNI.loadInterfaceIPMappings(); err == nil {
		if err := amdHostDeviceCNI.addInterfaceIPMapping(deviceID, hostInterfaceName, getLinkState(devLink), addrs); err != nil {
			log.Printf("error adding interface mapping, err: %v", err)
			return err
		}
	} else {
		log.Printf("failed to load mappings file, err: %v", err)
		return err
	}

	return types.PrintResult(executeResult, cniConf["cniVersion"].(string))
}

func cmdDel(args *skel.CmdArgs) error {
	var cniConf map[string]interface{}
	if err := json.Unmarshal(args.StdinData, &cniConf); err != nil {
		log.Printf("error unmarshalling json, err: %v", err)
		return err
	}

	deviceID, err := getDeviceIDFromArgs(cniConf)
	if err != nil {
		log.Printf("failed to get deviceID, err: %v", err)
		return err
	}

	// get the IP from the mapping file for the interface
	var interfaceMappingFound bool
	var m *Mapping
	if err := amdHostDeviceCNI.loadInterfaceIPMappings(); err == nil {
		if m, interfaceMappingFound = amdHostDeviceCNI.getInterfaceIPMapping(deviceID); interfaceMappingFound {
			args.IfName = m.HostInterfaceName
		} else {
			// This scenario occurs when a NAD is updated to use the amd-host-device CNI
			// after a workload has already started. During deletion, the amd-host-device CNI will be invoked,
			// but it won't find the expected mapping that would have been created during the ADD phase if it had been used initially.
			// As a result, the delete operation may repeatedly fail and trigger retries.
			// To prevent this, allow the flow to proceed so that the actual CNI delete operation can complete.
			log.Printf("interface mapping not found for %s", args.IfName)
		}
	} else {
		log.Printf("failed to load mappings file, err: %v", err)
		return err
	}

	_, err = execPlugin("host-device", "DEL", args.StdinData, args, false)
	if err != nil {
		log.Printf("failed to execute host-device plugin %v: %v", string(args.StdinData), err)
		return err
	}

	// Restore the IP address on the host interface if it was found in the mapping
	if interfaceMappingFound {
		amdHostDeviceCNI.configureHostInterface(m.HostInterfaceName, m.State, m.HostInterfaceIPs)
		amdHostDeviceCNI.removeInterfaceIPMapping(deviceID)
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	// Construct the full CNI configuration.
	var cniConf map[string]interface{}
	if err := json.Unmarshal(args.StdinData, &cniConf); err != nil {
		log.Printf("failed to unmarshal original CNI config, err: %v", err)
		return err
	}

	deviceID, err := getDeviceIDFromArgs(cniConf)
	if err != nil {
		log.Printf("failed to get deviceID, err: %v", err)
		return err
	}

	// update the request ifName to the host interface name before calling the plugin
	var found bool
	var m *Mapping
	if err := amdHostDeviceCNI.loadInterfaceIPMappings(); err == nil {
		if m, found = amdHostDeviceCNI.getInterfaceIPMapping(deviceID); found {
			args.IfName = m.HostInterfaceName
		} else {
			log.Printf("interface mapping not found for %s", args.IfName)
		}
	} else {
		log.Printf("failed to load mappings file, err: %v", err)
		return err
	}

	executeResult, err := execPlugin("host-device", "CHECK", args.StdinData, args, true)
	if err != nil {
		log.Printf("failed to execute host-device plugin %v: %v", string(args.StdinData), err)
		return err
	}

	return types.PrintResult(executeResult, cniConf["cniVersion"].(string))
}

func cmdStatus(args *skel.CmdArgs) error {
	// Construct the full CNI configuration.
	var cniConf map[string]interface{}
	if err := json.Unmarshal(args.StdinData, &cniConf); err != nil {
		log.Printf("failed to unmarshal original CNI config, err: %v", err)
		return err
	}
	deviceID, err := getDeviceIDFromArgs(cniConf)
	if err != nil {
		log.Printf("failed to get deviceID, err: %v", err)
		return err
	}

	// update the request ifName to the host interface name before calling the plugin
	var found bool
	var m *Mapping
	if err := amdHostDeviceCNI.loadInterfaceIPMappings(); err == nil {
		if m, found = amdHostDeviceCNI.getInterfaceIPMapping(deviceID); found {
			args.IfName = m.HostInterfaceName
		} else {
			log.Printf("interface mapping not found for %s", args.IfName)
		}
	} else {
		log.Printf("failed to load mappings file, err: %v", err)
		return err
	}

	executeResult, err := execPlugin("host-device", "STATUS", args.StdinData, args, true)
	if err != nil {
		log.Printf("failed to execute host-device plugin %v: %v", string(args.StdinData), err)
		return err
	}

	return types.PrintResult(executeResult, cniConf["cniVersion"].(string))
}

func main() {
	log.SetPrefix(getLogPrefix())
	if err := os.MkdirAll(AMDHostDeviceCNILocalStore, 0700); err != nil {
		log.Fatalf("failed to create directory %s: %v", AMDHostDeviceCNILocalStore, err)
	}

	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:    cmdAdd,
		Del:    cmdDel,
		Check:  cmdCheck,
		Status: cmdStatus,
		/* FIXME GC */
	}, version.All, CNIPluginName)
}

// Check if the link is up or down by inspecting the Flags
func getLinkState(link netlink.Link) string {
	if link.Attrs().Flags&net.FlagUp != 0 {
		return InterfaceUP
	} else {
		return InterfaceDOWN
	}
}

func getDeviceIDFromArgs(cniConf map[string]interface{}) (string, error) {
	deviceID := cniConf["deviceID"].(string)
	if deviceID == "" {
		return deviceID, fmt.Errorf("deviceID is missing in the CNI config")
	}
	return deviceID, nil
}

func getLogPrefix() string {
	return fmt.Sprintf("[%s]", CNIPluginName)
}
