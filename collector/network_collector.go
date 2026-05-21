package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lynnyq/hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
)

type networkCollector struct {
	nodeNetworkReceiveBytesDesc       *prometheus.Desc
	nodeNetworkTransmitBytesDesc      *prometheus.Desc
	nodeNetworkReceivePacketsDesc     *prometheus.Desc
	nodeNetworkTransmitPacketsDesc    *prometheus.Desc
	nodeNetworkReceiveErrsDesc        *prometheus.Desc
	nodeNetworkTransmitErrsDesc       *prometheus.Desc
	nodeNetworkReceiveDropDesc        *prometheus.Desc
	nodeNetworkTransmitDropDesc       *prometheus.Desc
	nodeNetworkReceiveFifoDesc        *prometheus.Desc
	nodeNetworkReceiveFrameDesc       *prometheus.Desc
	nodeNetworkReceiveMulticastDesc   *prometheus.Desc
	nodeNetworkTransmitCollsDesc      *prometheus.Desc
	nodeNetworkTransmitCarrierDesc    *prometheus.Desc
	nodeNetworkTransmitFifoDesc       *prometheus.Desc
	nodeNetworkReceiveCompressedDesc  *prometheus.Desc
	nodeNetworkTransmitCompressedDesc *prometheus.Desc
	nodeNetworkReceiveNoHandlerDesc   *prometheus.Desc
	nodeBondingSlavesDesc             *prometheus.Desc
	nodeBondingActiveDesc             *prometheus.Desc
	nodeNetworkInfoDesc               *prometheus.Desc
}

func newNetworkCollector() *networkCollector {
	return &networkCollector{
		nodeNetworkReceiveBytesDesc: prometheus.NewDesc(
			"node_network_receive_bytes_total",
			"Network device receive bytes total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitBytesDesc: prometheus.NewDesc(
			"node_network_transmit_bytes_total",
			"Network device transmit bytes total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceivePacketsDesc: prometheus.NewDesc(
			"node_network_receive_packets_total",
			"Network device receive packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitPacketsDesc: prometheus.NewDesc(
			"node_network_transmit_packets_total",
			"Network device transmit packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveErrsDesc: prometheus.NewDesc(
			"node_network_receive_errs_total",
			"Network device receive error packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitErrsDesc: prometheus.NewDesc(
			"node_network_transmit_errs_total",
			"Network device transmit error packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveDropDesc: prometheus.NewDesc(
			"node_network_receive_drop_total",
			"Network device receive dropped packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitDropDesc: prometheus.NewDesc(
			"node_network_transmit_drop_total",
			"Network device transmit dropped packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveFifoDesc: prometheus.NewDesc(
			"node_network_receive_fifo_total",
			"Network device receive FIFO errors total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveFrameDesc: prometheus.NewDesc(
			"node_network_receive_frame_total",
			"Network device receive frame errors total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveMulticastDesc: prometheus.NewDesc(
			"node_network_receive_multicast_total",
			"Network device receive multicast packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitCollsDesc: prometheus.NewDesc(
			"node_network_transmit_colls_total",
			"Network device transmit collision packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitCarrierDesc: prometheus.NewDesc(
			"node_network_transmit_carrier_total",
			"Network device transmit carrier packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitFifoDesc: prometheus.NewDesc(
			"node_network_transmit_fifo_total",
			"Network device transmit FIFO errors total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveCompressedDesc: prometheus.NewDesc(
			"node_network_receive_compressed_total",
			"Network device receive compressed packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkTransmitCompressedDesc: prometheus.NewDesc(
			"node_network_transmit_compressed_total",
			"Network device transmit compressed packets total.",
			[]string{"device"},
			nil,
		),
		nodeNetworkReceiveNoHandlerDesc: prometheus.NewDesc(
			"node_network_receive_nohandler_total",
			"Network device receive nohandler packets total.",
			[]string{"device"},
			nil,
		),
		nodeBondingSlavesDesc: prometheus.NewDesc(
			"node_bonding_slaves",
			"Number of configured slaves per bonding interface.",
			[]string{"master"},
			nil,
		),
		nodeBondingActiveDesc: prometheus.NewDesc(
			"node_bonding_active",
			"Number of active slaves per bonding interface.",
			[]string{"master"},
			nil,
		),
		nodeNetworkInfoDesc: prometheus.NewDesc(
			"node_network_info",
			"Non-numeric data about network interfaces.",
			[]string{"device", "operstate", "speed", "duplex"},
			nil,
		),
	}
}

func (collector *networkCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.nodeNetworkReceiveBytesDesc
	ch <- collector.nodeNetworkTransmitBytesDesc
	ch <- collector.nodeNetworkReceivePacketsDesc
	ch <- collector.nodeNetworkTransmitPacketsDesc
	ch <- collector.nodeNetworkReceiveErrsDesc
	ch <- collector.nodeNetworkTransmitErrsDesc
	ch <- collector.nodeNetworkReceiveDropDesc
	ch <- collector.nodeNetworkTransmitDropDesc
	ch <- collector.nodeNetworkReceiveFifoDesc
	ch <- collector.nodeNetworkReceiveFrameDesc
	ch <- collector.nodeNetworkReceiveMulticastDesc
	ch <- collector.nodeNetworkTransmitCollsDesc
	ch <- collector.nodeNetworkTransmitCarrierDesc
	ch <- collector.nodeNetworkTransmitFifoDesc
	ch <- collector.nodeNetworkReceiveCompressedDesc
	ch <- collector.nodeNetworkTransmitCompressedDesc
	ch <- collector.nodeNetworkReceiveNoHandlerDesc
	ch <- collector.nodeBondingSlavesDesc
	ch <- collector.nodeBondingActiveDesc
	ch <- collector.nodeNetworkInfoDesc
}

func (collector *networkCollector) Collect(ch chan<- prometheus.Metric) {
	interfaces, err := os.ReadDir("/sys/class/net")
	if err != nil {
		utils.Logger.WithError(err).Error("failed to read /sys/class/net")
		return
	}

	for _, iface := range interfaces {
		ifaceName := iface.Name()
		stats, err := readInterfaceStatistics(ifaceName)
		if err != nil {
			utils.Logger.WithError(err).WithField("interface", ifaceName).Debug("failed to read interface statistics")
			continue
		}

		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveBytesDesc, prometheus.CounterValue, float64(stats.RXBytes), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitBytesDesc, prometheus.CounterValue, float64(stats.TXBytes), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceivePacketsDesc, prometheus.CounterValue, float64(stats.RXPackets), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitPacketsDesc, prometheus.CounterValue, float64(stats.TXPackets), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveErrsDesc, prometheus.CounterValue, float64(stats.RXErrors), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitErrsDesc, prometheus.CounterValue, float64(stats.TXErrors), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveDropDesc, prometheus.CounterValue, float64(stats.RXDropped+stats.RXMissedErrors), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitDropDesc, prometheus.CounterValue, float64(stats.TXDropped), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveFifoDesc, prometheus.CounterValue, float64(stats.RXFIFO), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveFrameDesc, prometheus.CounterValue, float64(stats.RXFrame+stats.RXLengthErrors+stats.RXOverErrors+stats.RXCRCErrors), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveMulticastDesc, prometheus.CounterValue, float64(stats.Multicast), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitCollsDesc, prometheus.CounterValue, float64(stats.Collisions), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitCarrierDesc, prometheus.CounterValue, float64(stats.TXCarrierErrors+stats.TXAbortedErrors+stats.TXHeartbeatErrors+stats.TXWindowErrors), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitFifoDesc, prometheus.CounterValue, float64(stats.TXFIFO), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveCompressedDesc, prometheus.CounterValue, float64(stats.RXCompressed), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkTransmitCompressedDesc, prometheus.CounterValue, float64(stats.TXCompressed), ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkReceiveNoHandlerDesc, prometheus.CounterValue, float64(stats.RXNoHandler), ifaceName)
	}

	bondStats, err := readBondingStats("/sys/class/net")
	if err != nil {
		utils.Logger.WithError(err).Debug("failed to collect bonding stats")
		return
	}

	for master, status := range bondStats {
		ch <- prometheus.MustNewConstMetric(collector.nodeBondingSlavesDesc, prometheus.GaugeValue, float64(status[0]), master)
		ch <- prometheus.MustNewConstMetric(collector.nodeBondingActiveDesc, prometheus.GaugeValue, float64(status[1]), master)
	}

	for _, iface := range interfaces {
		ifaceName := iface.Name()
		operstate, speed, duplex := readInterfaceInfo(ifaceName)
		ch <- prometheus.MustNewConstMetric(collector.nodeNetworkInfoDesc, prometheus.GaugeValue, 1, ifaceName, operstate, speed, duplex)
	}
}

type interfaceStatistics struct {
	RXBytes           int64
	TXBytes           int64
	RXPackets         int64
	TXPackets         int64
	RXErrors          int64
	TXErrors          int64
	RXDropped         int64
	TXDropped         int64
	Multicast         int64
	Collisions        int64
	RXFIFO            int64
	TXFIFO            int64
	RXFrame           int64
	RXCompressed      int64
	TXCompressed      int64
	RXNoHandler       int64
	RXMissedErrors    int64
	RXLengthErrors    int64
	RXOverErrors      int64
	RXCRCErrors       int64
	TXAbortedErrors   int64
	TXCarrierErrors   int64
	TXHeartbeatErrors int64
	TXWindowErrors    int64
}

func readInterfaceStatistics(iface string) (*interfaceStatistics, error) {
	stats := &interfaceStatistics{}
	fields := map[string]*int64{
		"rx_bytes":            &stats.RXBytes,
		"tx_bytes":            &stats.TXBytes,
		"rx_packets":          &stats.RXPackets,
		"tx_packets":          &stats.TXPackets,
		"rx_errors":           &stats.RXErrors,
		"tx_errors":           &stats.TXErrors,
		"rx_dropped":          &stats.RXDropped,
		"tx_dropped":          &stats.TXDropped,
		"multicast":           &stats.Multicast,
		"collisions":          &stats.Collisions,
		"rx_fifo_errors":      &stats.RXFIFO,
		"rx_frame_errors":     &stats.RXFrame,
		"rx_compressed":       &stats.RXCompressed,
		"tx_compressed":       &stats.TXCompressed,
		"rx_nohandler":        &stats.RXNoHandler,
		"rx_missed_errors":    &stats.RXMissedErrors,
		"rx_length_errors":    &stats.RXLengthErrors,
		"rx_over_errors":      &stats.RXOverErrors,
		"rx_crc_errors":       &stats.RXCRCErrors,
		"tx_aborted_errors":   &stats.TXAbortedErrors,
		"tx_carrier_errors":   &stats.TXCarrierErrors,
		"tx_heartbeat_errors": &stats.TXHeartbeatErrors,
		"tx_window_errors":    &stats.TXWindowErrors,
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

func readBondingStats(root string) (map[string][2]int, error) {
	status := map[string][2]int{}
	masters, err := os.ReadFile(filepath.Join(root, "bonding_masters"))
	if err != nil {
		return nil, err
	}
	for _, master := range strings.Fields(string(masters)) {
		slaves, err := os.ReadFile(filepath.Join(root, master, "bonding", "slaves"))
		if err != nil {
			return nil, err
		}

		sstat := [2]int{0, 0}
		for _, slave := range strings.Fields(string(slaves)) {
			sstat[0]++
			state, err := os.ReadFile(filepath.Join(root, master, fmt.Sprintf("lower_%s", slave), "bonding_slave", "mii_status"))
			if err != nil {
				state, err = os.ReadFile(filepath.Join(root, master, fmt.Sprintf("slave_%s", slave), "bonding_slave", "mii_status"))
			}
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(string(state)) == "up" {
				sstat[1]++
			}
		}
		status[master] = sstat
	}

	return status, nil
}

func readInterfaceInfo(ifaceName string) (operstate, speed, duplex string) {
	operstate = readSysfsFile(filepath.Join("/sys/class/net", ifaceName, "operstate"))
	speed = readSysfsFile(filepath.Join("/sys/class/net", ifaceName, "speed"))
	duplex = readSysfsFile(filepath.Join("/sys/class/net", ifaceName, "duplex"))
	return
}

func readSysfsFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
