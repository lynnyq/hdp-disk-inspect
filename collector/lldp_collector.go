package collector

import (
	"bytes"
	"encoding/xml"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hdp-disk-inspect/utils"

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
	devicePath := filepath.Join("/sys/class/net", ifaceName, "device")
	link, err := os.Readlink(devicePath)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.Logger.WithError(err).WithField("interface", ifaceName).Warn("failed to read device symlink")
		}
		return ""
	}

	// link like ../../../0000:00:1f.6
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
