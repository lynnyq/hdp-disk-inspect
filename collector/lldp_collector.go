package collector

import (
	"bytes"
	"encoding/xml"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/lynnyq/hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	slotCache   = make(map[string]string)
	slotCacheMu sync.RWMutex
	pciCache    = make(map[string]string)
	pciCacheMu  sync.RWMutex
	dmidecodeCache      string
	dmidecodeCacheMu    sync.RWMutex
)

type lldpCollector struct {
	neighborInfo *prometheus.Desc
}

func newLLDPCollector() *lldpCollector {
	return &lldpCollector{
		neighborInfo: prometheus.NewDesc("lldp_neighbor_info",
			"LLDP neighbor information for the local interface.",
			[]string{"local_host", "local_interface", "local_interface_ip", "local_interface_slot", "remote_chassis_id", "remote_chassis_name", "remote_chassis_mgmt_ip", "remote_port_id", "remote_port_description", "remote_port_ttl"},
			nil,
		),
	}
}

func (collector *lldpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.neighborInfo
}

func (collector *lldpCollector) Collect(ch chan<- prometheus.Metric) {
	cmd := exec.Command("lldpcli", "show", "neighbors", "-f", "xml")

	out, err := cmd.CombinedOutput()
	if err != nil {
		utils.Logger.WithError(err).Error("failed to execute lldpcli")
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		utils.Logger.WithError(err).Warn("failed to get local hostname")
	}
	hostname = safeString(hostname)

	var lldp LLDPData

	decoder := xml.NewDecoder(bytes.NewReader(out))
	err = decoder.Decode(&lldp)
	if err != nil {
		utils.Logger.WithError(err).WithField("raw_output", string(bytes.TrimSpace(out))).Error("failed to parse lldp xml")
		return
	}

	if len(lldp.Interfaces) == 0 {
		utils.Logger.Debug("no lldp neighbors discovered")
		return
	}

	for _, inf := range lldp.Interfaces {
		localIfaceIP := safeString(getInterfaceIPs(inf.Name))
		localIfaceSlot := safeString(getInterfaceSlot(inf.Name))

		metric, err := prometheus.NewConstMetric(
			collector.neighborInfo,
			prometheus.GaugeValue,
			1,
			hostname,
			safeString(inf.Name),
			localIfaceIP,
			localIfaceSlot,
			safeString(inf.Chassis.ID),
			safeString(inf.Chassis.Name),
			// safeString(inf.Chassis.Descr),
			safeString(inf.Chassis.MgmtIP),
			safeString(inf.Port.ID),
			safeString(inf.Port.Descr),
			safeString(inf.Port.TTL),
		)
		if err != nil {
			utils.Logger.WithError(err).
				WithField("interface", inf.Name).
				Error("failed to create LLDP metric")
			continue
		}
		ch <- metric
	}
}

func safeString(value string) string {
	return strings.TrimSpace(value)
}

func getInterfaceIPs(ifaceName string) string {
	if ifaceName == "" {
		return ""
	}

	ifaceName = strings.TrimSpace(ifaceName)

	masterIface := getInterfaceMaster(ifaceName)
	if masterIface != "" {
		// 如果是物理接口已被绑定在 bond 上，则从 master bond 取 ip
		return getInterfaceIPs(masterIface)
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		utils.Logger.WithError(err).WithField("interface", ifaceName).Warn("failed to get interface by name")
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil {
		utils.Logger.WithError(err).WithField("interface", ifaceName).Warn("failed to list addresses for interface")
		return ""
	}

	var ips []string
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		ips = append(ips, ip.String())
	}

	return strings.Join(ips, ",")
}

func getInterfaceMaster(ifaceName string) string {
	masterPath := filepath.Join("/sys/class/net", ifaceName, "master")
	link, err := os.Readlink(masterPath)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.Logger.WithError(err).WithField("interface", ifaceName).Warn("failed to read master symlink")
		}
		return ""
	}

	return filepath.Base(link)
}

func getInterfaceSlot(ifaceName string) string {
	if ifaceName == "" {
		return ""
	}

	ifaceName = strings.TrimSpace(ifaceName)

	slotCacheMu.RLock()
	if slot, ok := slotCache[ifaceName]; ok {
		slotCacheMu.RUnlock()
		return slot
	}
	slotCacheMu.RUnlock()

	masterIface := getInterfaceMaster(ifaceName)
	if masterIface != "" && masterIface != ifaceName {
		slot := getInterfaceSlot(masterIface)
		slotCacheMu.Lock()
		slotCache[ifaceName] = slot
		slotCacheMu.Unlock()
		return slot
	}

	var slot string

	if slot = getSlotFromEthtool(ifaceName); slot != "" {
		slotCacheMu.Lock()
		slotCache[ifaceName] = slot
		slotCacheMu.Unlock()
		return slot
	}

	if slot = getSlotFromSysfs(ifaceName); slot != "" {
		slotCacheMu.Lock()
		slotCache[ifaceName] = slot
		slotCacheMu.Unlock()
		return slot
	}

	if slot = getPhysicalSlotFromDmidecode(ifaceName); slot != "" {
		slotCacheMu.Lock()
		slotCache[ifaceName] = slot
		slotCacheMu.Unlock()
		return slot
	}

	slot = getPciSlotFromLspci(ifaceName)
	slotCacheMu.Lock()
	slotCache[ifaceName] = slot
	slotCacheMu.Unlock()
	return slot
}

func getSlotFromEthtool(ifaceName string) string {
	cmd := exec.Command("ethtool", "-i", ifaceName)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "bus-info:") {
			busInfo := strings.TrimSpace(strings.TrimPrefix(line, "bus-info:"))
			parts := strings.Split(busInfo, ":")
			if len(parts) >= 2 {
				domainBus := strings.Join(parts[len(parts)-2:], ":")
				slot := extractSlotFromBusInfo(domainBus)
				if slot != "" {
					return slot
				}
			}
		}
	}

	return ""
}

func extractSlotFromBusInfo(busInfo string) string {
	parts := strings.Split(busInfo, ":")
	if len(parts) < 3 {
		return ""
	}

	slot := parts[len(parts)-1]
	if slotNum, err := strconv.ParseInt(slot, 16, 64); err == nil {
		return strconv.FormatInt(slotNum, 10)
	}

	return slot
}

func getSlotFromSysfs(ifaceName string) string {
	pciAddr := getPciAddr(ifaceName)
	if pciAddr == "" {
		return ""
	}

	slotPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "slot")
	slotData, err := os.ReadFile(slotPath)
	if err == nil {
		slot := strings.TrimSpace(string(slotData))
		if slot != "" && slot != "0" {
			return slot
		}
	}

	physfnPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "physfn")
	if _, err := os.Stat(physfnPath); err == nil {
		link, err := os.Readlink(physfnPath)
		if err == nil {
			parts := strings.Split(link, "/")
			if len(parts) > 0 {
				physfnPci := parts[len(parts)-1]
				if physfnPci != "" {
					return getSlotFromSysfs(physfnPci)
				}
			}
		}
	}

	sriovPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "sriov")
	if _, err := os.Stat(sriovPath); err == nil {
		vfsPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "physfn/net")
		if vfLinks, err := os.ReadDir(vfsPath); err == nil {
			for _, vfLink := range vfLinks {
				if vfLink.Name() != ifaceName {
					continue
				}
				vfNetPath := filepath.Join(vfsPath, vfLink.Name())
				pcidevPath := filepath.Join(vfNetPath, "device")
				link, err := os.Readlink(pcidevPath)
				if err == nil {
					vfPci := filepath.Base(link)
					if vfPci != "" && vfPci != pciAddr {
						slot := getSlotFromSysfs(vfPci)
						if slot != "" {
							return slot
						}
					}
				}
			}
		}
	}

	return ""
}

func getPhysicalSlotFromDmidecode(ifaceName string) string {
	pciAddr := getPciAddr(ifaceName)
	if pciAddr == "" {
		return ""
	}

	dmidecodeCacheMu.RLock()
	if dmidecodeCache == "" {
		dmidecodeCacheMu.RUnlock()
		dmidecodeCacheMu.Lock()
		if dmidecodeCache == "" {
			cmd := exec.Command("dmidecode", "-t", "slot")
			output, err := cmd.Output()
			if err != nil {
				utils.Logger.WithError(err).Warn("failed to run dmidecode -t slot")
				dmidecodeCache = ""
				dmidecodeCacheMu.Unlock()
				return ""
			}
			dmidecodeCache = string(output)
		}
		dmidecodeCacheMu.Unlock()
		dmidecodeCacheMu.RLock()
	}
	defer dmidecodeCacheMu.RUnlock()

	if dmidecodeCache == "" {
		return ""
	}

	busInfo := extractBusFromPciAddr(pciAddr)
	expectedBusNum := -1
	if busInfo != "" {
		if busNum, err := strconv.ParseInt(busInfo, 16, 64); err == nil {
			expectedBusNum = int(busNum)
		}
	}

	lines := strings.Split(dmidecodeCache, "\n")
	inSlot := false
	var currentDesignation string
	var currentID string
	var bestMatch string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "System Slot Information") {
			inSlot = true
			currentDesignation = ""
			currentID = ""
			continue
		}

		if inSlot {
			if strings.HasPrefix(line, "ID: ") {
				currentID = strings.TrimSpace(strings.TrimPrefix(line, "ID: "))
				if expectedBusNum >= 0 {
					if id, err := strconv.Atoi(currentID); err == nil && id == expectedBusNum+1 {
						if currentDesignation != "" {
							return currentDesignation
						}
						bestMatch = currentID
					}
				}
			}
			if strings.HasPrefix(line, "Designation: ") {
				currentDesignation = strings.TrimSpace(strings.TrimPrefix(line, "Designation: "))
			}
			if strings.HasPrefix(line, "Bus Address:") {
				addrStr := strings.TrimSpace(strings.TrimPrefix(line, "Bus Address:"))
				parts := strings.Split(addrStr, " ")
				if len(parts) > 0 {
					addrParts := strings.Split(parts[0], ":")
					if len(addrParts) >= 2 {
						if busNum, err := strconv.ParseInt(addrParts[1], 16, 64); err == nil {
							if int(busNum) == expectedBusNum {
								if currentDesignation != "" {
									return currentDesignation
								}
								bestMatch = currentID
							}
						}
					}
				}
			}
		}

		if inSlot && line == "" {
			inSlot = false
		}
	}

	return bestMatch
}

func extractBusFromPciAddr(pciAddr string) string {
	parts := strings.Split(pciAddr, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func getPciSlotFromLspci(ifaceName string) string {
	pciAddr := getPciAddr(ifaceName)
	if pciAddr == "" {
		return ""
	}

	cmd := exec.Command("lspci", "-s", pciAddr, "-v")
	output, err := cmd.Output()
	if err != nil {
		return formatPciAddr(pciAddr)
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Slot:") {
			slot := strings.TrimSpace(strings.TrimPrefix(line, "Slot:"))
			if slot != "" {
				return slot
			}
		}
	}

	return formatPciAddr(pciAddr)
}

func formatPciAddr(pciAddr string) string {
	if strings.HasPrefix(pciAddr, "0000:") {
		return pciAddr[5:]
	}
	return pciAddr
}

func getPciAddr(ifaceName string) string {
	pciCacheMu.RLock()
	if pci, ok := pciCache[ifaceName]; ok {
		pciCacheMu.RUnlock()
		return pci
	}
	pciCacheMu.RUnlock()

	devicePath := filepath.Join("/sys/class/net", ifaceName, "device")
	link, err := os.Readlink(devicePath)
	if err != nil {
		return ""
	}

	parts := strings.Split(link, "/")
	pci := ""
	if len(parts) > 0 {
		pci = parts[len(parts)-1]
	}

	pciCacheMu.Lock()
	pciCache[ifaceName] = pci
	pciCacheMu.Unlock()

	return pci
}

func RegisterLLDPCollector() {
	if err := prometheus.Register(newLLDPCollector()); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			utils.Logger.WithError(are).Debug("lldp collector already registered")
			return
		}
		utils.Logger.WithError(err).Error("failed to register lldp collector")
	}
}

// Data represents LLDP XML output for collection.
type LLDPData struct {
	Interfaces []LLDPInterface `xml:"interface"`
}

type LLDPInterface struct {
	Name    string      `xml:"name,attr"`
	Chassis LLDPChassis `xml:"chassis"`
	Port    LLDPPort    `xml:"port"`
}

type LLDPChassis struct {
	ID     string `xml:"id"`
	Name   string `xml:"name"`
	Descr  string `xml:"descr"`
	MgmtIP string `xml:"mgmt-ip"`
}

type LLDPPort struct {
	ID    string `xml:"id"`
	Descr string `xml:"descr"`
	TTL   string `xml:"ttl"`
}
