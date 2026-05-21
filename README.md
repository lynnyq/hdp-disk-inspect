# HDP Disk Inspect

HDP Disk Inspect 是一个用于服务器硬件和网络监控的工具，提供磁盘 RAID 信息采集、网络指标监控和 LLDP 邻居信息发现，同时支持通过 gRPC 远程执行 shell 命令。

## 功能特性

### 1. Prometheus 指标采集

- **磁盘 RAID 指标**：采集磁盘设备路径、序列号、RAID 槽位号、厂商型号、SMART 错误计数、RAID 状态等信息
- **网络指标**：网络接口流量统计、Bonding 状态、接口信息等
- **LLDP 指标**：网络邻居发现信息（交换机名、端口信息等）

### 2. gRPC 远程命令执行

- 安全的远程 shell 命令执行
- 支持命令超时控制
- 完整的命令执行结果返回（stdout/stderr/exit code）
- 可选 TLS 双向认证加密传输

## 系统要求

### 操作系统
- Linux（主要支持）
- Windows（部分功能）

### Go 版本
- Go 1.20+

### 系统依赖软件包

#### 磁盘相关
- `smartmontools`（smartctl）：用于磁盘 SMART 信息采集
- `lsblk`：用于块设备信息采集
- `storcli`（可选）：用于 MegaRAID 硬件 RAID 信息采集
- `lsraid`（可选）：用于 RAID 信息采集
- `mdadm`（可选）：用于软 RAID 信息采集

#### 网络相关
- `lldpd` 或 `lldpad`：用于 LLDP 邻居发现
- `ethtool`：用于网络接口信息查询
- `dmidecode`：用于硬件信息采集
- `lspci`：用于 PCI 设备信息查询

## 快速开始

### 1. 安装

```bash
git clone https://github.com/lynnyq/hdp-disk-inspect.git
cd hdp-disk-inspect
go build -o hdp-disk-inspect .
```

### 2. 运行

```bash
# 基本运行
./hdp-disk-inspect

# 自定义监听地址
./hdp-disk-inspect -s 0.0.0.0:58002

# 启用详细日志
./hdp-disk-inspect -log-level debug
```

### 3. 访问指标

```bash
curl http://localhost:58002/metrics
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-s` | `0.0.0.0:58002` | 服务监听地址 |
| `-v` | - | 显示版本号并退出 |
| `-allow-root` | `false` | 允许以 root 用户运行（不推荐） |
| `-enable-tls` | `false` | 启用 TLS 加密 |
| `-ca-file` | - | CA 证书文件路径 |
| `-cert-file` | - | 服务器证书文件路径 |
| `-key-file` | - | 服务器私钥文件路径 |
| `-log-level` | `info` | 日志级别（debug/info/warn/error/fatal） |

## Prometheus 指标文档

### 磁盘指标 (`node_disk_info`)
```
node_disk_info{
  device="/dev/sda",
  serial_number="S1AXNS0KB01234",
  slot_number="252:0",
  vendor="Samsung",
  model="SSD 860 EVO",
  raid_status="online",
  raid_level="",
  health_status="PASSED"
} 1
```

### 网络指标
- `node_network_receive_bytes_total`：网络接口接收字节总数
- `node_network_transmit_bytes_total`：网络接口发送字节总数
- `node_network_receive_packets_total`：网络接口接收包总数
- `node_network_transmit_packets_total`：网络接口发送包总数
- `node_network_receive_errs_total`：网络接口接收错误包总数
- `node_network_transmit_errs_total`：网络接口发送错误包总数
- `node_network_receive_drop_total`：网络接口接收丢包总数
- `node_network_transmit_drop_total`：网络接口发送丢包总数
- `node_bonding_slaves`：Bonding 接口的 slave 数量
- `node_bonding_active`：Bonding 接口的 active slave 数量
- `node_network_info`：网络接口状态信息

### LLDP 指标 (`lldp_neighbor_info`)
```
lldp_neighbor_info{
  local_host="server1",
  local_interface="eth0",
  local_interface_ip="192.168.1.10",
  local_interface_slot="0000:01:00.0",
  remote_chassis_id="00:1a:2b:3c:4d:5e",
  remote_chassis_name="switch1",
  remote_chassis_mgmt_ip="192.168.1.1",
  remote_port_id="GigabitEthernet1/0/1",
  remote_port_description="To Server1",
  remote_port_ttl="120"
} 1
```

## 项目结构

```
hdp-disk-inspect/
├── main.go                # 主程序入口
├── go.mod                 # Go 模块依赖
├── go.sum                 # Go 依赖校验
├── collector/             # 指标采集器
│   ├── disk_collector.go  # 磁盘指标采集
│   ├── lldp_collector.go  # LLDP 指标采集
│   ├── network_collector.go # 网络指标采集
│   └── metrics.go         # 指标管理
├── rpc/                   # gRPC 相关
│   ├── auth/              # 认证相关
│   ├── proto/             # Proto 定义
│   └── server/            # gRPC 服务实现
├── utils/                 # 工具函数
├── docs/                  # 文档目录
└── server/                # Python gRPC 服务器实现（可选）
```

## 完整文档

更详细的文档请参考 [docs](docs/) 目录：

- [快速入门指南](docs/quickstart.md)
- [指标说明文档](docs/metrics.md)
- [部署指南](docs/deployment.md)
- [API 参考](docs/api.md)
- [TLS 双向认证配置](docs/tls.md)
- [故障排除](docs/troubleshooting.md)

## Python gRPC 服务器

项目还提供了一个 Python 版本的 gRPC 服务器实现，详见 [server/](server/) 目录。

## 许可证

本项目采用 Apache 2.0 许可证。详见 [LICENSE](LICENSE) 文件。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题，请通过 GitHub Issues 联系。
