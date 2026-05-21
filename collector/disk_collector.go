package collector

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lynnyq/hdp-disk-inspect/utils"
	"github.com/prometheus/client_golang/prometheus"
)

type DiskInfo struct {
	Device       string  // 设备路径（/dev/sda）
	SerialNumber string  // 磁盘序列号
	SlotNumber   string  // RAID 槽位号
	Size         int64   // 磁盘大小（字节）
	SizeGB       float64 // 磁盘大小（GB）
	Vendor       string  // 厂商
	Model        string  // 型号
	ErrorCount   int64   // SMART 错误计数
	RaidStatus   string  // RAID 状态
	RAIDLevel    string  // RAID 级别
	HealthStatus string  // 健康状态
	CollectedAt  string  // 采集时间
}

type diskCollector struct {
	diskInfo *prometheus.Desc
}

func newDiskCollector() *diskCollector {
	return &diskCollector{
		diskInfo: prometheus.NewDesc(
			"node_disk_info",
			"Disk information including RAID status, serial number, slot number, etc.",
			[]string{
				"device",
				"serial_number",
				"slot_number",
				"vendor",
				"model",
				"raid_status",
				"raid_level",
				"health_status",
			},
			nil,
		),
	}
}

func (c *diskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.diskInfo
}

func (c *diskCollector) Collect(ch chan<- prometheus.Metric) {
	diskInfos := collectDiskInfo()

	for _, diskInfo := range diskInfos {
		metric, err := prometheus.NewConstMetric(
			c.diskInfo,
			prometheus.GaugeValue,
			1,
			diskInfo.Device,
			diskInfo.SerialNumber,
			diskInfo.SlotNumber,
			diskInfo.Vendor,
			diskInfo.Model,
			diskInfo.RaidStatus,
			diskInfo.RAIDLevel,
			diskInfo.HealthStatus,
		)
		if err != nil {
			utils.Logger.WithError(err).WithField("device", diskInfo.Device).Error("failed to create disk metric")
			continue
		}
		ch <- metric
	}
}

func collectDiskInfo() []DiskInfo {
	var disks []DiskInfo
	collectedAt := time.Now().Format(time.RFC3339)

	// 尝试多种方式收集磁盘信息
	// 1. 从 /sys/block 获取基础信息
	disks = collectFromSysfs(collectedAt)

	// 2. 从 lsblk 获取更多信息
	disks = collectFromLsblk(disks, collectedAt)

	// 3. 尝试从 smartctl 获取 SMART 信息
	disks = collectFromSmartctl(disks, collectedAt)

	// 4. 尝试从 RAID 工具获取 RAID 信息
	disks = collectFromRaidTools(disks, collectedAt)

	return disks
}

func collectFromSysfs(collectedAt string) []DiskInfo {
	var disks []DiskInfo

	sysBlockPath := "/sys/block"
	entries, err := os.ReadDir(sysBlockPath)
	if err != nil {
		utils.Logger.WithError(err).Warn("failed to read /sys/block")
		return disks
	}

	for _, entry := range entries {
		deviceName := entry.Name()
		// 只处理真实的块设备，跳过 loop, ram, dm 等
		if strings.HasPrefix(deviceName, "loop") ||
			strings.HasPrefix(deviceName, "ram") ||
			strings.HasPrefix(deviceName, "dm-") ||
			strings.HasPrefix(deviceName, "sr") ||
			strings.HasPrefix(deviceName, "rom") {
			continue
		}

		devicePath := "/dev/" + deviceName
		deviceSysPath := filepath.Join(sysBlockPath, deviceName)

		diskInfo := DiskInfo{
			Device:      devicePath,
			CollectedAt: collectedAt,
		}

		// 获取厂商信息
		if vendorBytes, err := os.ReadFile(filepath.Join(deviceSysPath, "device", "vendor")); err == nil {
			diskInfo.Vendor = strings.TrimSpace(string(vendorBytes))
		}

		// 获取型号信息
		if modelBytes, err := os.ReadFile(filepath.Join(deviceSysPath, "device", "model")); err == nil {
			diskInfo.Model = strings.TrimSpace(string(modelBytes))
		}

		// 获取序列号
		if serialBytes, err := os.ReadFile(filepath.Join(deviceSysPath, "device", "serial")); err == nil {
			diskInfo.SerialNumber = strings.TrimSpace(string(serialBytes))
		}

		// 获取大小
		if sizeBytes, err := os.ReadFile(filepath.Join(deviceSysPath, "size")); err == nil {
			sizeStr := strings.TrimSpace(string(sizeBytes))
			if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
				diskInfo.Size = size * 512
				diskInfo.SizeGB = float64(diskInfo.Size) / (1024 * 1024 * 1024)
			}
		}

		disks = append(disks, diskInfo)
	}

	return disks
}

func collectFromLsblk(existing []DiskInfo, collectedAt string) []DiskInfo {
	disks := make(map[string]DiskInfo)

	for _, d := range existing {
		deviceName := strings.TrimPrefix(d.Device, "/dev/")
		disks[deviceName] = d
	}

	cmd := exec.Command("lsblk", "-J", "-o", "NAME,SERIAL,SIZE,TYPE")
	output, err := cmd.Output()
	if err != nil {
		utils.Logger.WithError(err).Debug("lsblk not available or failed")
		return existing
	}

	outputStr := string(output)
	// 简单的解析（不使用完整的 JSON 解析，避免依赖）
	scanner := bufio.NewScanner(strings.NewReader(outputStr))
	var currentName, currentSerial, currentType string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, `"name":`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentName = strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
			}
		} else if strings.Contains(line, `"serial":`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentSerial = strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
			}
		} else if strings.Contains(line, `"type":`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentType = strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
			}
			if currentType == "disk" && currentName != "" {
				if disk, exists := disks[currentName]; exists {
					if disk.SerialNumber == "" {
						disk.SerialNumber = currentSerial
					}
					disks[currentName] = disk
				}
			}
		}
	}

	result := make([]DiskInfo, 0, len(disks))
	for _, d := range disks {
		d.CollectedAt = collectedAt
		result = append(result, d)
	}
	return result
}

func collectFromSmartctl(existing []DiskInfo, collectedAt string) []DiskInfo {
	disks := make(map[string]DiskInfo)
	for _, d := range existing {
		deviceName := strings.TrimPrefix(d.Device, "/dev/")
		disks[deviceName] = d
	}

	for deviceName, diskInfo := range disks {
		devicePath := "/dev/" + deviceName
		cmd := exec.Command("smartctl", "-a", devicePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			utils.Logger.WithError(err).WithField("device", devicePath).Debug("smartctl failed")
			continue
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Serial Number:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && diskInfo.SerialNumber == "" {
					diskInfo.SerialNumber = strings.TrimSpace(parts[1])
				}
			} else if strings.Contains(line, "Raw_Read_Error_Rate") {
				if fields := strings.Fields(line); len(fields) > 9 {
					if val, err := strconv.ParseInt(fields[9], 10, 64); err == nil {
						diskInfo.ErrorCount += val
					}
				}
			} else if strings.Contains(line, "Reallocated_Sector_Ct") {
				if fields := strings.Fields(line); len(fields) > 9 {
					if val, err := strconv.ParseInt(fields[9], 10, 64); err == nil {
						diskInfo.ErrorCount += val
					}
				}
			} else if strings.Contains(line, "Health Status:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					diskInfo.HealthStatus = strings.TrimSpace(parts[1])
				}
			}
		}
		disks[deviceName] = diskInfo
	}

	result := make([]DiskInfo, 0, len(disks))
	for _, d := range disks {
		d.CollectedAt = collectedAt
		result = append(result, d)
	}
	return result
}

func collectFromRaidTools(existing []DiskInfo, collectedAt string) []DiskInfo {
	disks := make(map[string]DiskInfo)
	for _, d := range existing {
		deviceName := strings.TrimPrefix(d.Device, "/dev/")
		disks[deviceName] = d
	}

	// 尝试使用 MegaRAID 工具 (storcli)
	if cmd := exec.Command("storcli64", "-V"); cmd.Err == nil {
		if output, err := exec.Command("storcli64", "/c0/eall/sall", "show", "all", "J").Output(); err == nil {
			parseStorcliOutput(string(output), disks)
		}
	}

	// 尝试使用 lsraid
	if _, err := exec.LookPath("lsraid"); err == nil {
		parseLsraidOutput(disks)
	}

	// 尝试使用 mdadm 查看软 RAID
	if _, err := exec.LookPath("mdadm"); err == nil {
		parseMdadmOutput(disks)
	}

	result := make([]DiskInfo, 0, len(disks))
	for _, d := range disks {
		d.CollectedAt = collectedAt
		result = append(result, d)
	}
	return result
}

func parseStorcliOutput(output string, disks map[string]DiskInfo) {
	// 简单的 storcli JSON 输出解析
	// 查找 EID:Slt 和 序列号
	lines := strings.Split(output, "\n")
	var currentEID, currentSerial string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `"EID:Slt"`) {
			if parts := strings.Split(line, ":"); len(parts) > 1 {
				val := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
				currentEID = val
			}
		} else if strings.Contains(line, `"SN"`) {
			if parts := strings.Split(line, ":"); len(parts) > 1 {
				currentSerial = strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
			}
			// 根据序列号匹配磁盘
			for deviceName, diskInfo := range disks {
				if diskInfo.SerialNumber != "" && strings.Contains(strings.ToLower(currentSerial), strings.ToLower(diskInfo.SerialNumber)) {
					diskInfo.SlotNumber = currentEID
					diskInfo.RaidStatus = "online"
					disks[deviceName] = diskInfo
				}
			}
		} else if strings.Contains(line, `"State"`) {
			if parts := strings.Split(line, ":"); len(parts) > 1 {
				state := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), `",`))
				if state != "" {
					for deviceName, diskInfo := range disks {
						if diskInfo.SlotNumber == currentEID {
							diskInfo.RaidStatus = state
							disks[deviceName] = diskInfo
						}
					}
				}
			}
		}
	}
}

func parseLsraidOutput(disks map[string]DiskInfo) {
	cmd := exec.Command("lsraid", "-A", "-p")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			device := fields[0]
			deviceName := strings.TrimPrefix(device, "/dev/")
			if diskInfo, exists := disks[deviceName]; exists {
				if diskInfo.RAIDLevel == "" {
					diskInfo.RAIDLevel = fields[3]
				}
				disks[deviceName] = diskInfo
			}
		}
	}
}

func parseMdadmOutput(disks map[string]DiskInfo) {
	cmd := exec.Command("mdadm", "--detail", "--scan")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "level=") {
			if parts := strings.Split(line, "level="); len(parts) > 1 {
				raidLevel := strings.Fields(parts[1])[0]
				for deviceName, diskInfo := range disks {
					if diskInfo.RAIDLevel == "" {
						diskInfo.RAIDLevel = raidLevel
						disks[deviceName] = diskInfo
					}
				}
			}
		}
	}
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

// ParseDiskInfo is a helper function for testing
func ParseDiskInfo(output []byte) []DiskInfo {
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var disks []DiskInfo

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "sd") {
			disks = append(disks, DiskInfo{Device: line})
		}
	}

	return disks
}
