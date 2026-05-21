# 指标说明文档

本文档详细描述 HDP Disk Inspect 暴露的所有 Prometheus 指标。

## 目录

1. [磁盘指标](#磁盘指标)
2. [网络指标](#网络指标)
3. [LLDP 指标](#lldp-指标)

---

## 磁盘指标

### `node_disk_info`

磁盘信息指标，包含磁盘的基本信息、RAID 状态、健康状态等。

**类型**: Gauge

**标签**:

| 标签 | 类型 | 描述 |
|------|------|------|
| `device` | string | 设备路径（如 `/dev/sda`） |
| `serial_number` | string | 磁盘序列号 |
| `slot_number` | string | RAID 槽位号（如 `252:0`） |
| `vendor` | string | 厂商名称（如 `Samsung`） |
| `model` | string | 型号信息（如 `SSD 860 EVO`） |
| `raid_status` | string | RAID 状态（`online` / `failed` / `offline`） |
| `raid_level` | string | RAID 级别（`raid0` / `raid1` / `raid5` / `raid6` / `raid10`） |
| `health_status` | string | 健康状态（`PASSED` / `FAILED`） |

**示例**:

```
node_disk_info{
  device="/dev/sda",
  serial_number="S1AXNS0KB01234",
  slot_number="252:0",
  vendor="Samsung",
  model="SSD 860 EVO 500GB",
  raid_status="online",
  raid_level="",
  health_status="PASSED"
} 1
```

**采集来源**:
- `/sys/block/` 系统文件
- `smartctl` 命令
- `lsblk` 命令
- RAID 工具（storcli/lsraid/mdadm）

---

## 网络指标

### 基础流量指标

所有流量指标类型均为 Counter（累积值）。

#### `node_network_receive_bytes_total`

网络接口接收字节总数。

| 标签 | 类型 | 描述 |
|------|------|------|
| `device` | string | 网络接口名称（如 `eth0`） |

#### `node_network_transmit_bytes_total`

网络接口发送字节总数。

| 标签 | 类型 | 描述 |
|------|------|------|
| `device` | string | 网络接口名称 |

#### `node_network_receive_packets_total`

网络接口接收包总数。

#### `node_network_transmit_packets_total`

网络接口发送包总数。

### 错误指标

#### `node_network_receive_errs_total`

网络接口接收错误包总数。

#### `node_network_transmit_errs_total`

网络接口发送错误包总数。

#### `node_network_receive_drop_total`

网络接口接收丢包总数。

#### `node_network_transmit_drop_total`

网络接口发送丢包总数。

### 高级错误指标

#### `node_network_receive_fifo_total`

接收 FIFO 错误总数。

#### `node_network_receive_frame_total`

接收帧错误总数（包含 CRC、长度、溢出等）。

#### `node_network_receive_multicast_total`

接收多播包总数。

#### `node_network_transmit_colls_total`

发送碰撞总数。

#### `node_network_transmit_carrier_total`

发送载波错误总数。

#### `node_network_transmit_fifo_total`

发送 FIFO 错误总数。

### Bonding 指标

#### `node_bonding_slaves`

Bonding 接口的 slave 数量。

**类型**: Gauge

| 标签 | 类型 | 描述 |
|------|------|------|
| `master` | string | Bonding 接口名称（如 `bond0`） |

#### `node_bonding_active`

Bonding 接口的 active slave 数量。

**类型**: Gauge

| 标签 | 类型 | 描述 |
|------|------|------|
| `master` | string | Bonding 接口名称 |

### 接口信息指标

#### `node_network_info`

网络接口状态信息。

**类型**: Gauge

| 标签 | 类型 | 描述 |
|------|------|------|
| `device` | string | 网络接口名称 |
| `operstate` | string | 接口状态（`up` / `down` / `unknown`） |
| `speed` | string | 接口速率（如 `1000` 表示 1Gbps） |
| `duplex` | string | 双工模式（`full` / `half`） |

**示例**:

```
node_network_info{device="eth0",operstate="up",speed="1000",duplex="full"} 1
```

---

## LLDP 指标

### `lldp_neighbor_info`

LLDP 邻居发现信息。

**类型**: Gauge

**标签**:

| 标签 | 类型 | 描述 |
|------|------|------|
| `local_host` | string | 本地主机名 |
| `local_interface` | string | 本地网络接口名 |
| `local_interface_ip` | string | 本地接口 IP 地址 |
| `local_interface_slot` | string | 本地接口 PCI 槽位号 |
| `remote_chassis_id` | string | 远程设备 ID（通常是 MAC 地址） |
| `remote_chassis_name` | string | 远程设备名称（交换机名） |
| `remote_chassis_mgmt_ip` | string | 远程设备管理 IP |
| `remote_port_id` | string | 远程端口 ID |
| `remote_port_description` | string | 远程端口描述 |
| `remote_port_ttl` | string | LLDP 信息 TTL |

**示例**:

```
lldp_neighbor_info{
  local_host="server01",
  local_interface="eth0",
  local_interface_ip="192.168.1.100",
  local_interface_slot="0000:01:00.0",
  remote_chassis_id="00:1a:2b:3c:4d:5e",
  remote_chassis_name="cisco-switch-01",
  remote_chassis_mgmt_ip="192.168.1.1",
  remote_port_id="GigabitEthernet1/0/1",
  remote_port_description="Server01 - Eth0",
  remote_port_ttl="120"
} 1
```

---

## PromQL 查询示例

### 磁盘相关查询

```promql
# 查询所有磁盘信息
node_disk_info

# 查询健康状态为 FAILED 的磁盘
node_disk_info{health_status="FAILED"}

# 查询 RAID 状态为 offline 的磁盘
node_disk_info{raid_status="offline"}
```

### 网络相关查询

```promql
# 网络接口接收速率（bytes/s）
rate(node_network_receive_bytes_total[1m])

# 网络接口发送速率（bytes/s）
rate(node_network_transmit_bytes_total[1m])

# 网络接口接收错误率
rate(node_network_receive_errs_total[1m])

# Bonding 接口的 active slave 数量
node_bonding_active

# 查询所有 UP 状态的接口
node_network_info{operstate="up"}
```

### LLDP 相关查询

```promql
# 查询所有 LLDP 邻居
lldp_neighbor_info

# 查询连接到特定交换机的接口
lldp_neighbor_info{remote_chassis_name="cisco-switch-01"}
```

---

## 告警规则示例

```yaml
groups:
  - name: hdp_disk_inspect_alerts
    rules:
      # 磁盘健康告警
      - alert: DiskHealthFailed
        expr: node_disk_info{health_status="FAILED"} > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Disk health check failed"
          description: "Disk {{ $labels.device }} has health status FAILED"

      # RAID 状态告警
      - alert: RaidDiskOffline
        expr: node_disk_info{raid_status=~"offline|failed"} > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "RAID disk offline/failed"
          description: "Disk {{ $labels.device }} is {{ $labels.raid_status }}"

      # 网络错误告警
      - alert: NetworkHighErrorRate
        expr: rate(node_network_receive_errs_total[5m]) > 100 or rate(node_network_transmit_errs_total[5m]) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Network interface high error rate"
          description: "Interface {{ $labels.device }} has high error rate"

      # Bonding 链路减少告警
      - alert: BondingSlaveMissing
        expr: node_bonding_active < 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Bonding interface missing active slave"
          description: "Bond {{ $labels.master }} has {{ $value }} active slaves"

      # LLDP 邻居丢失告警
      - alert: LLDPNeighborLost
        expr: absent_over_time(lldp_neighbor_info[5m])
        for: 5m
        labels:
          severity: info
        annotations:
          summary: "LLDP neighbor information lost"
```
