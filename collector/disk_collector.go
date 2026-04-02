package collector

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
)

type diskCollector struct {
	smartHealthDesc             *prometheus.Desc
	smartTemperatureDesc        *prometheus.Desc
	smartReallocatedDesc        *prometheus.Desc
	smartPendingSectorDesc      *prometheus.Desc
	smartOfflineUncorrectableDesc *prometheus.Desc
	raidControllerStatusDesc    *prometheus.Desc
	raidPhysicalDiskStatusDesc  *prometheus.Desc
	raidLogicalDiskStatusDesc   *prometheus.Desc
}

func newDiskCollector() *diskCollector {
	return &diskCollector{
		smartHealthDesc: prometheus.NewDesc(
			"disk_smart_health_status",
			"SMART health status for the disk (1 = PASSED, 0 = FAILED).",
			[]string{"device", "model", "serial"},
			nil,
		),
		smartTemperatureDesc: prometheus.NewDesc(
			"disk_smart_temperature_celsius",
			"SMART temperature reported by the disk in Celsius.",
			[]string{"device", "model", "serial"},
			nil,
		),
		smartReallocatedDesc: prometheus.NewDesc(
			"disk_smart_reallocated_sectors_count",
			"SMART reallocated sector count for the disk.",
			[]string{"device", "model", "serial"},
			nil,
		),
		smartPendingSectorDesc: prometheus.NewDesc(
			"disk_smart_current_pending_sector_count",
			"SMART current pending sector count for the disk.",
			[]string{"device", "model", "serial"},
			nil,
		),
		smartOfflineUncorrectableDesc: prometheus.NewDesc(
			"disk_smart_offline_uncorrectable_count",
			"SMART offline uncorrectable sector count for the disk.",
			[]string{"device", "model", "serial"},
			nil,
		),
		raidControllerStatusDesc: prometheus.NewDesc(
			"raid_controller_status",
			"RAID controller status (1 = optimal/healthy, 0 = degraded or failed).",
			[]string{"controller"},
			nil,
		),
		raidPhysicalDiskStatusDesc: prometheus.NewDesc(
			"raid_physical_disk_status",
			"RAID physical disk status (1 = online, 0 = offline/faulty).",
			[]string{"controller", "slot", "device", "serial"},
			nil,
		),
		raidLogicalDiskStatusDesc: prometheus.NewDesc(
			"raid_logical_disk_status",
			"RAID logical disk status (1 = optimal, 0 = degraded or failed).",
			[]string{"controller", "logical_drive", "raid_level"},
			nil,
		),
	}
}

func (collector *diskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.smartHealthDesc
	ch <- collector.smartTemperatureDesc
	ch <- collector.smartReallocatedDesc
	ch <- collector.smartPendingSectorDesc
	ch <- collector.smartOfflineUncorrectableDesc
	ch <- collector.raidControllerStatusDesc
	ch <- collector.raidPhysicalDiskStatusDesc
	ch <- collector.raidLogicalDiskStatusDesc
}

func (collector *diskCollector) Collect(ch chan<- prometheus.Metric) {
	collector.collectSmartMetrics(ch)
	collector.collectRaidMetrics(ch)
}

func (collector *diskCollector) collectSmartMetrics(ch chan<- prometheus.Metric) {
	smartctlPath, err := exec.LookPath("smartctl")
	if err != nil {
		utils.Logger.WithError(err).Debug("smartctl not found, skipping disk SMART metrics")
		return
	}

	devices, err := getSmartDevices(smartctlPath)
	if err != nil {
		utils.Logger.WithError(err).Debug("failed to enumerate smartctl devices")
		return
	}

	for _, device := range devices {
		info, err := querySmartDevice(smartctlPath, device)
		if err != nil {
			utils.Logger.WithError(err).WithField("device", device).Debug("failed to query smartctl device")
			continue
		}

		labels := []string{info.Device, info.Model, info.Serial}
		if info.HealthKnown {
			value := 0.0
			if info.SmartHealthOK {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(collector.smartHealthDesc, prometheus.GaugeValue, value, labels...)
		}
		if info.Temperature != nil {
			ch <- prometheus.MustNewConstMetric(collector.smartTemperatureDesc, prometheus.GaugeValue, float64(*info.Temperature), labels...)
		}
		if info.ReallocatedSectorCount != nil {
			ch <- prometheus.MustNewConstMetric(collector.smartReallocatedDesc, prometheus.GaugeValue, float64(*info.ReallocatedSectorCount), labels...)
		}
		if info.CurrentPendingSectorCount != nil {
			ch <- prometheus.MustNewConstMetric(collector.smartPendingSectorDesc, prometheus.GaugeValue, float64(*info.CurrentPendingSectorCount), labels...)
		}
		if info.OfflineUncorrectable != nil {
			ch <- prometheus.MustNewConstMetric(collector.smartOfflineUncorrectableDesc, prometheus.GaugeValue, float64(*info.OfflineUncorrectable), labels...)
		}
	}
}

func (collector *diskCollector) collectRaidMetrics(ch chan<- prometheus.Metric) {
	storcliPath, err := findStorcliPath()
	if err != nil {
		utils.Logger.WithError(err).Debug("storcli not found, skipping RAID metrics")
		return
	}

	output, err := exec.Command(storcliPath, "/c0", "show", "all").CombinedOutput()
	if err != nil {
		utils.Logger.WithError(err).WithField("path", storcliPath).Debug("failed to query storcli controller")
		return
	}

	controller := "0"
	statusValue := 0.0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Controller =") {
			controller = strings.TrimSpace(strings.TrimPrefix(line, "Controller ="))
			continue
		}
		if strings.HasPrefix(line, "State =") || strings.HasPrefix(line, "Status =") {
			value := parseRaidState(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]))
			if value > statusValue {
				statusValue = value
			}
		}
	}
	ch <- prometheus.MustNewConstMetric(collector.raidControllerStatusDesc, prometheus.GaugeValue, statusValue, controller)

	collector.collectStorcliPhysicalDisks(ch, storcliPath, controller)
	collector.collectStorcliLogicalDisks(ch, storcliPath, controller)
}

func (collector *diskCollector) collectStorcliPhysicalDisks(ch chan<- prometheus.Metric, storcliPath, controller string) {
	output, err := exec.Command(storcliPath, "/c0", "/eall", "/sall", "show", "all").CombinedOutput()
	if err != nil {
		utils.Logger.WithError(err).WithField("controller", controller).Debug("failed to query storcli physical disks")
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentSlot string
	var currentDevice string
	var currentSerial string
	var currentState string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Slot Number") || strings.HasPrefix(line, "Slot") {
			currentSlot = parseStorcliValue(line)
			continue
		}
		if strings.HasPrefix(line, "Device ID") {
			currentDevice = parseStorcliValue(line)
			continue
		}
		if strings.HasPrefix(line, "Serial Number") {
			currentSerial = parseStorcliValue(line)
			continue
		}
		if strings.HasPrefix(line, "State") {
			currentState = parseStorcliValue(line)
			continue
		}
		if currentSlot != "" && currentState != "" {
			value := 0.0
			if strings.EqualFold(currentState, "optimal") || strings.EqualFold(currentState, "online") || strings.EqualFold(currentState, "healthy") {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(collector.raidPhysicalDiskStatusDesc, prometheus.GaugeValue, value, controller, currentSlot, currentDevice, currentSerial)
			currentSlot = ""
			currentDevice = ""
			currentSerial = ""
			currentState = ""
		}
	}
}

func (collector *diskCollector) collectStorcliLogicalDisks(ch chan<- prometheus.Metric, storcliPath, controller string) {
	output, err := exec.Command(storcliPath, "/c0", "/vall", "show", "all").CombinedOutput()
	if err != nil {
		utils.Logger.WithError(err).WithField("controller", controller).Debug("failed to query storcli logical disks")
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentVD string
	var currentRaid string
	var currentState string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Virtual Drive:") {
			currentVD = parseStorcliValue(line)
			continue
		}
		if strings.HasPrefix(line, "RAID Level") || strings.HasPrefix(line, "RAID Level:") {
			currentRaid = parseStorcliValue(line)
			continue
		}
		if strings.HasPrefix(line, "State") {
			currentState = parseStorcliValue(line)
			continue
		}
		if currentVD != "" && currentRaid != "" && currentState != "" {
			value := 0.0
			if strings.EqualFold(currentState, "optimal") || strings.EqualFold(currentState, "online") || strings.EqualFold(currentState, "healthy") {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(collector.raidLogicalDiskStatusDesc, prometheus.GaugeValue, value, controller, currentVD, currentRaid)
			currentVD = ""
			currentRaid = ""
			currentState = ""
		}
	}
}

func getSmartDevices(smartctlPath string) ([]string, error) {
	output, err := exec.Command(smartctlPath, "--scan").CombinedOutput()
	if err != nil {
		return nil, err
	}

	var devices []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	reDevice := regexp.MustCompile(`^(/dev/\S+)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := reDevice.FindStringSubmatch(line); len(matches) == 2 {
			devices = append(devices, matches[1])
		}
	}
	return devices, nil
}

type smartInfo struct {
	Device                    string
	Model                     string
	Serial                    string
	HealthKnown               bool
	SmartHealthOK             bool
	Temperature               *int64
	ReallocatedSectorCount    *int64
	CurrentPendingSectorCount *int64
	OfflineUncorrectable      *int64
}

func querySmartDevice(smartctlPath, device string) (*smartInfo, error) {
	output, err := exec.Command(smartctlPath, "-H", "-A", "-i", device).CombinedOutput()
	if err != nil {
		return nil, err
	}

	info := &smartInfo{Device: device, Model: "unknown", Serial: "unknown"}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	reModel := regexp.MustCompile(`(?i)^Device Model:\s*(.+)$`)
	reSerial := regexp.MustCompile(`(?i)^Serial Number:\s*(.+)$`)
	reSmartHealth := regexp.MustCompile(`(?i)SMART overall-health self-assessment test result:\s*(\S+)`)
	reTemp := regexp.MustCompile(`(?i)^\s*194\s+Temperature_Celsius\s+.*\s+(\d+)$`)
	reRealloc := regexp.MustCompile(`(?i)^\s*5\s+Reallocated_Sector_Ct\s+.*\s+(\d+)$`)
	rePending := regexp.MustCompile(`(?i)^\s*197\s+Current_Pending_Sector\s+.*\s+(\d+)$`)
	reOffline := regexp.MustCompile(`(?i)^\s*198\s+Offline_Uncorrectable\s+.*\s+(\d+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case reModel.MatchString(line):
			info.Model = strings.TrimSpace(reModel.FindStringSubmatch(line)[1])
		case reSerial.MatchString(line):
			info.Serial = strings.TrimSpace(reSerial.FindStringSubmatch(line)[1])
		case reSmartHealth.MatchString(line):
			info.HealthKnown = true
			info.SmartHealthOK = strings.EqualFold(strings.TrimSpace(reSmartHealth.FindStringSubmatch(line)[1]), "PASSED")
		case reTemp.MatchString(line):
			temperature, _ := strconv.ParseInt(reTemp.FindStringSubmatch(line)[1], 10, 64)
			info.Temperature = &temperature
		case reRealloc.MatchString(line):
			count, _ := strconv.ParseInt(reRealloc.FindStringSubmatch(line)[1], 10, 64)
			info.ReallocatedSectorCount = &count
		case rePending.MatchString(line):
			count, _ := strconv.ParseInt(rePending.FindStringSubmatch(line)[1], 10, 64)
			info.CurrentPendingSectorCount = &count
		case reOffline.MatchString(line):
			count, _ := strconv.ParseInt(reOffline.FindStringSubmatch(line)[1], 10, 64)
			info.OfflineUncorrectable = &count
		}
	}

	return info, nil
}

func findStorcliPath() (string, error) {
	paths := []string{"storcli64", "storcli"}
	for _, name := range paths {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func parseRaidState(value string) float64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.Contains(value, "optimal") || strings.Contains(value, "online") || strings.Contains(value, "good") || strings.Contains(value, "healthy") {
		return 1
	}
	return 0
}

func parseStorcliValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(line, "=", 2)
	}
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func RegisterDiskCollector() {
	if err := prometheus.Register(newDiskCollector()); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			utils.Logger.WithError(are).Debug("disk collector already registered")
			return
		}
		utils.Logger.WithError(err).Error("failed to register disk collector")
	}
}
