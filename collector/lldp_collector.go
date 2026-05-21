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

	"github.com/lynnyq/hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
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
	// First try to get physical slot from dmidecode
	if slot := getPhysicalSlotFromDmidecode(ifaceName); slot != "" {
		return slot
	}

	// Fallback to PCI slot from lspci or sysfs
	return getPciSlot(ifaceName)
}

func getPhysicalSlotFromDmidecode(ifaceName string) string {
	pciAddr := getPciAddr(ifaceName)
	if pciAddr == "" {
		return ""
	}

	// Extract bus number, assume slot ID = busNum + 1
	parts := strings.Split(pciAddr, ":")
	if len(parts) < 2 {
		return ""
	}
	bus := parts[1]
	busNum, err := strconv.Atoi(bus)
	if err != nil {
		return ""
	}
	expectedSlotID := busNum + 1

	// Run dmidecode -t slot
	cmd := exec.Command("dmidecode", "-t", "slot")
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.WithError(err).WithField("interface", ifaceName).Warn("failed to run dmidecode -t slot")
		return ""
	}

	lines := strings.Split(string(output), "\n")
	inSlot := false
	var designation string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "System Slot Information") {
			inSlot = true
			designation = ""
		}
		if inSlot {
			if strings.HasPrefix(line, "ID: ") {
				idStr := strings.TrimSpace(strings.TrimPrefix(line, "ID: "))
				if id, err := strconv.Atoi(idStr); err == nil && id == expectedSlotID {
					// Found matching slot, return designation if available, else ID
					if designation != "" {
						return designation
					}
					return idStr
				}
			}
			if strings.HasPrefix(line, "Designation: ") {
				designation = strings.TrimSpace(strings.TrimPrefix(line, "Designation: "))
			}
		}
		if inSlot && line == "" {
			inSlot = false
		}
	}

	return ""
}

func getPciSlot(ifaceName string) string {
	pciAddr := getPciAddr(ifaceName)
	if pciAddr == "" {
		return ""
	}

	// Try lspci -s <pciAddr> -v | grep -i slot or something, but lspci doesn't have physical slot
	// For now, just return the PCI address as slot
	cmd := exec.Command("lspci", "-s", pciAddr)
	err := cmd.Run()
	if err == nil {
		// If lspci succeeds, return pciAddr
		return pciAddr
	}

	// Fallback to sysfs slot
	slotPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "slot")
	slotData, err := os.ReadFile(slotPath)
	if err == nil {
		slot := strings.TrimSpace(string(slotData))
		if slot != "" {
			return slot
		}
	}

	// Return simplified PCI address
	if strings.HasPrefix(pciAddr, "0000:") {
		return pciAddr[5:]
	}
	return pciAddr
}

func getPciAddr(ifaceName string) string {
	devicePath := filepath.Join("/sys/class/net", ifaceName, "device")
	link, err := os.Readlink(devicePath)
	if err != nil {
		return ""
	}
	parts := strings.Split(link, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
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
