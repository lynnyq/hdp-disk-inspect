package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
)

type networkCollector struct {
	interfaceUpDesc        *prometheus.Desc
	interfaceSpeedDesc     *prometheus.Desc
	interfaceRXBytesDesc   *prometheus.Desc
	interfaceTXBytesDesc   *prometheus.Desc
	interfaceRXPacketsDesc *prometheus.Desc
	interfaceTXPacketsDesc *prometheus.Desc
	interfaceRXErrorsDesc  *prometheus.Desc
	interfaceTXErrorsDesc  *prometheus.Desc
	bondSlaveLinkUpDesc    *prometheus.Desc
	bondActiveSlaveDesc    *prometheus.Desc
	bondSlaveSpeedDesc     *prometheus.Desc
	bondSlaveLinkFailDesc  *prometheus.Desc
}

func newNetworkCollector() *networkCollector {
	return &networkCollector{
		interfaceUpDesc: prometheus.NewDesc(
			"network_interface_up",
			"Network interface operational status (1 = up, 0 = down).",
			[]string{"interface"},
			nil,
		),
		interfaceSpeedDesc: prometheus.NewDesc(
			"network_interface_speed_mbps",
			"Network interface speed in Mbps.",
			[]string{"interface"},
			nil,
		),
		interfaceRXBytesDesc: prometheus.NewDesc(
			"network_interface_rx_bytes",
			"Number of bytes received on the interface.",
			[]string{"interface"},
			nil,
		),
		interfaceTXBytesDesc: prometheus.NewDesc(
			"network_interface_tx_bytes",
			"Number of bytes transmitted on the interface.",
			[]string{"interface"},
			nil,
		),
		interfaceRXPacketsDesc: prometheus.NewDesc(
			"network_interface_rx_packets",
			"Number of packets received on the interface.",
			[]string{"interface"},
			nil,
		),
		interfaceTXPacketsDesc: prometheus.NewDesc(
			"network_interface_tx_packets",
			"Number of packets transmitted on the interface.",
			[]string{"interface"},
			nil,
		),
		interfaceRXErrorsDesc: prometheus.NewDesc(
			"network_interface_rx_errors",
			"Number of receive errors on the interface.",
			[]string{"interface"},
			nil,
		),
		interfaceTXErrorsDesc: prometheus.NewDesc(
			"network_interface_tx_errors",
			"Number of transmit errors on the interface.",
			[]string{"interface"},
			nil,
		),
		bondSlaveLinkUpDesc: prometheus.NewDesc(
			"bond_slave_link_up",
			"Bond slave link state (1 = up, 0 = down).",
			[]string{"bond", "slave"},
			nil,
		),
		bondActiveSlaveDesc: prometheus.NewDesc(
			"bond_active_slave",
			"Active slave for the bond interface.",
			[]string{"bond", "active_slave"},
			nil,
		),
		bondSlaveSpeedDesc: prometheus.NewDesc(
			"bond_slave_speed_mbps",
			"Bond slave link speed in Mbps.",
			[]string{"bond", "slave"},
			nil,
		),
		bondSlaveLinkFailDesc: prometheus.NewDesc(
			"bond_slave_link_failures",
			"Bond slave link failure count.",
			[]string{"bond", "slave"},
			nil,
		),
	}
}

func (collector *networkCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.interfaceUpDesc
	ch <- collector.interfaceSpeedDesc
	ch <- collector.interfaceRXBytesDesc
	ch <- collector.interfaceTXBytesDesc
	ch <- collector.interfaceRXPacketsDesc
	ch <- collector.interfaceTXPacketsDesc
	ch <- collector.interfaceRXErrorsDesc
	ch <- collector.interfaceTXErrorsDesc
	ch <- collector.bondSlaveLinkUpDesc
	ch <- collector.bondActiveSlaveDesc
	ch <- collector.bondSlaveSpeedDesc
	ch <- collector.bondSlaveLinkFailDesc
}

func (collector *networkCollector) Collect(ch chan<- prometheus.Metric) {
	interfaces, err := os.ReadDir("/sys/class/net")
	if err != nil {
		utils.Logger.WithError(err).Error("failed to read /sys/class/net")
		return
	}

	for _, iface := range interfaces {
		ifaceName := iface.Name()
		state, err := readInterfaceOperState(ifaceName)
		if err != nil {
			utils.Logger.WithError(err).WithField("interface", ifaceName).Debug("failed to read interface operstate")
			continue
		}
		value := 0.0
		if state {
			value = 1
		}
		ch <- prometheus.MustNewConstMetric(collector.interfaceUpDesc, prometheus.GaugeValue, value, ifaceName)

		speed, err := readInterfaceSpeed(ifaceName)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(collector.interfaceSpeedDesc, prometheus.GaugeValue, float64(speed), ifaceName)
		}

		stats, err := readInterfaceStatistics(ifaceName)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(collector.interfaceRXBytesDesc, prometheus.GaugeValue, float64(stats.RXBytes), ifaceName)
			ch <- prometheus.MustNewConstMetric(collector.interfaceTXBytesDesc, prometheus.GaugeValue, float64(stats.TXBytes), ifaceName)
			ch <- prometheus.MustNewConstMetric(collector.interfaceRXPacketsDesc, prometheus.GaugeValue, float64(stats.RXPackets), ifaceName)
			ch <- prometheus.MustNewConstMetric(collector.interfaceTXPacketsDesc, prometheus.GaugeValue, float64(stats.TXPackets), ifaceName)
			ch <- prometheus.MustNewConstMetric(collector.interfaceRXErrorsDesc, prometheus.GaugeValue, float64(stats.RXErrors), ifaceName)
			ch <- prometheus.MustNewConstMetric(collector.interfaceTXErrorsDesc, prometheus.GaugeValue, float64(stats.TXErrors), ifaceName)
		}
	}

	bondInfos, err := collectBondInfo()
	if err != nil {
		utils.Logger.WithError(err).Debug("failed to collect bond info")
		return
	}

	for bondName, bond := range bondInfos {
		if bond.ActiveSlave != "" {
			ch <- prometheus.MustNewConstMetric(collector.bondActiveSlaveDesc, prometheus.GaugeValue, 1, bondName, bond.ActiveSlave)
		}
		for _, slave := range bond.Slaves {
			linkUp := 0.0
			if strings.EqualFold(slave.MIIStatus, "up") {
				linkUp = 1
			}
			ch <- prometheus.MustNewConstMetric(collector.bondSlaveLinkUpDesc, prometheus.GaugeValue, linkUp, bondName, slave.Name)

			speed, err := readInterfaceSpeed(slave.Name)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(collector.bondSlaveSpeedDesc, prometheus.GaugeValue, float64(speed), bondName, slave.Name)
			} else {
				utils.Logger.WithError(err).WithFields(map[string]interface{}{"bond": bondName, "slave": slave.Name}).Debug("failed to read bond slave speed")
			}

			ch <- prometheus.MustNewConstMetric(collector.bondSlaveLinkFailDesc, prometheus.GaugeValue, float64(slave.LinkFailureCount), bondName, slave.Name)
		}
	}
}

type bondInfo struct {
	ActiveSlave string
	Slaves      []bondSlave
}

type bondSlave struct {
	Name             string
	MIIStatus        string
	LinkFailureCount int64
}

func readInterfaceOperState(iface string) (bool, error) {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "operstate"))
	if err != nil {
		return false, err
	}
	state := strings.TrimSpace(string(data))
	return state == "up", nil
}

func readInterfaceSpeed(iface string) (int64, error) {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "speed"))
	if err != nil {
		return 0, err
	}
	speedStr := strings.TrimSpace(string(data))
	return strconv.ParseInt(speedStr, 10, 64)
}

type interfaceStatistics struct {
	RXBytes   int64
	TXBytes   int64
	RXPackets int64
	TXPackets int64
	RXErrors  int64
	TXErrors  int64
}

func readInterfaceStatistics(iface string) (*interfaceStatistics, error) {
	stats := &interfaceStatistics{}
	fields := map[string]*int64{
		"rx_bytes":   &stats.RXBytes,
		"tx_bytes":   &stats.TXBytes,
		"rx_packets": &stats.RXPackets,
		"tx_packets": &stats.TXPackets,
		"rx_errors":  &stats.RXErrors,
		"tx_errors":  &stats.TXErrors,
	}

	for name, ptr := range fields {
		data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "statistics", name))
		if err != nil {
			return nil, err
		}
		value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return nil, err
		}
		*ptr = value
	}

	return stats, nil
}

func collectBondInfo() (map[string]bondInfo, error) {
	files, err := filepath.Glob("/proc/net/bonding/*")
	if err != nil {
		return nil, err
	}

	bondInfos := make(map[string]bondInfo)
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			utils.Logger.WithError(err).WithField("bond", filepath.Base(path)).Debug("failed to read bond file")
			continue
		}
		bondName := filepath.Base(path)
		bondInfos[bondName] = parseBondFile(string(content))
	}

	return bondInfos, nil
}

func parseBondFile(content string) bondInfo {
	scanner := bufio.NewScanner(strings.NewReader(content))
	bond := bondInfo{}
	var currentSlave *bondSlave

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Active Slave:") {
			bond.ActiveSlave = strings.TrimSpace(strings.TrimPrefix(line, "Active Slave:"))
			continue
		}
		if strings.HasPrefix(line, "Slave Interface:") {
			slaveName := strings.TrimSpace(strings.TrimPrefix(line, "Slave Interface:"))
			bond.Slaves = append(bond.Slaves, bondSlave{Name: slaveName})
			currentSlave = &bond.Slaves[len(bond.Slaves)-1]
			continue
		}
		if currentSlave != nil && strings.HasPrefix(line, "MII Status:") {
			currentSlave.MIIStatus = strings.TrimSpace(strings.TrimPrefix(line, "MII Status:"))
			continue
		}
		if currentSlave != nil && strings.HasPrefix(line, "Link Failure Count:") {
			countStr := strings.TrimSpace(strings.TrimPrefix(line, "Link Failure Count:"))
			count, err := strconv.ParseInt(countStr, 10, 64)
			if err == nil {
				currentSlave.LinkFailureCount = count
			}
		}
	}

	return bond
}

func RegisterNetworkCollector() {
	if err := prometheus.Register(newNetworkCollector()); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			utils.Logger.WithError(are).Debug("network collector already registered")
			return
		}
		utils.Logger.WithError(err).Error("failed to register network collector")
	}
}
