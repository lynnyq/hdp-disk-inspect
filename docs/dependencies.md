# 依赖软件

本文档列出 HDP Disk Inspect 指标采集所依赖的软件包。

## 目录

1. [必需依赖](#必需依赖)
2. [可选依赖](#可选依赖)
3. [按功能分类](#按功能分类)
4. [安装命令](#安装命令)
5. [依赖说明](#依赖说明)

---

## 必需依赖

这些软件是完整功能所必需的，虽然有些功能不依赖它们但强烈建议安装。

| 软件 | 用途 | 备注 |
|------|------|------|
| `util-linux` | 提供 `lsblk` 等工具 | 基本磁盘信息 |
| `smartmontools` | 提供 `smartctl` | 磁盘 SMART 信息和健康状态 |
| `lldpd` 或 `lldpad` | LLDP 守护进程 | 网络邻居发现 |
| `ethtool` | 网络接口工具 | 网络接口信息查询 |
| `dmidecode` | DMI 信息工具 | 硬件信息采集 |
| `pciutils` | 提供 `lspci` | PCI 设备信息 |

---

## 可选依赖

这些软件仅在特定环境或功能中需要。

| 软件 | 用途 | 备注 |
|------|------|------|
| `storcli` (MegaRAID) | MegaRAID 管理工具 | 仅用于带有 MegaRAID 卡的服务器 |
| `lsraid` | RAID 信息工具 | 用于某些 RAID 卡的信息采集 |
| `mdadm` | Linux 软 RAID 管理 | 用于软 RAID 信息采集 |

---

## 按功能分类

### 磁盘指标

| 依赖 | 指标 |
|------|------|
| `/sys/block` (内核提供) | 设备路径、大小、厂商、型号 |
| `smartctl` | 磁盘序列号、健康状态、错误计数 |
| `lsblk` | 块设备信息 |
| `storcli`/`lsraid`/`mdadm` | RAID 槽位号、RAID 状态、RAID 级别 |

### 网络指标

| 依赖 | 指标 |
|------|------|
| `/sys/class/net` (内核提供) | 流量统计、错误统计、Bonding 信息 |
| `ethtool` | 接口速度、双工模式 |

### LLDP 指标

| 依赖 | 指标 |
|------|------|
| `lldpd` / `lldpad` | 邻居信息 |
| `dmidecode` | 本地主机信息 |
| `lspci` | PCI 槽位信息 |

---

## 安装命令

### Debian/Ubuntu

```bash
sudo apt update
sudo apt install -y \
    smartmontools \
    lldpd \
    ethtool \
    dmidecode \
    pciutils \
    util-linux
```

### CentOS/RHEL 8+

```bash
sudo yum install -y epel-release
sudo yum install -y \
    smartmontools \
    lldpad \
    ethtool \
    dmidecode \
    pciutils \
    util-linux
```

### CentOS/RHEL 7

```bash
sudo yum install -y epel-release
sudo yum install -y \
    smartmontools \
    lldpad \
    ethtool \
    dmidecode \
    pciutils \
    util-linux
```

### Alpine Linux

```bash
apk add --no-cache \
    smartmontools \
    ethtool \
    dmidecode \
    pciutils \
    util-linux

# lldpd 在 Alpine 上可用但可能需要额外配置
```

---

## 依赖说明

### smartmontools (smartctl)

- **用途**: 读取磁盘的 SMART（Self-Monitoring, Analysis and Reporting Technology）信息
- **提供指标**: 磁盘序列号、健康状态、错误计数
- **验证安装**:

```bash
smartctl --version
sudo smartctl -i /dev/sda  # 检查是否可以读取磁盘信息
```

### lldpd / lldpad

- **用途**: LLDP（Link Layer Discovery Protocol）协议的实现
- **提供指标**: 网络邻居信息（交换机名、端口名、VLAN等）
- **两个选择**:
  - `lldpd` (推荐): 更现代、功能更全
  - `lldpad`: 传统实现，某些系统使用
- **验证安装**:

```bash
# 对于 lldpd
which lldpd
sudo systemctl status lldpd
sudo lldpcli show neighbors

# 对于 lldpad
which lldpad
sudo systemctl status lldpad
sudo lldptool get-tlv -i eth0 -n -V neighbor
```

### ethtool

- **用途**: 查询和控制网络接口
- **提供指标**: 接口速度、双工模式
- **验证安装**:

```bash
ethtool --version
sudo ethtool eth0
```

### dmidecode

- **用途**: 读取 DMI（Desktop Management Interface）信息
- **提供指标**: 服务器序列号、产品信息
- **验证安装**:

```bash
dmidecode --version
sudo dmidecode -s system-serial-number
```

### pciutils (lspci)

- **用途**: 列出 PCI 设备信息
- **提供指标**: 网卡插槽信息
- **验证安装**:

```bash
lspci --version
lspci
```

### util-linux (lsblk)

- **用途**: 列出块设备信息
- **提供指标**: 磁盘设备信息
- **验证安装**:

```bash
lsblk --version
lsblk
```

### storcli (可选, MegaRAID)

- **用途**: 管理 Broadcom/LSI MegaRAID 控制器
- **提供指标**: RAID 槽位号、RAID 状态
- **安装**: 需要从 Broadcom 官网下载

```bash
# 检查是否安装
which storcli64
sudo storcli64 /c0 show
```

### mdadm (可选, 软 RAID)

- **用途**: 管理 Linux 软 RAID
- **提供指标**: 软 RAID 信息
- **验证安装**:

```bash
mdadm --version
cat /proc/mdstat
```

---

## 完整的依赖检查脚本

可以运行以下脚本检查依赖是否安装：

```bash
#!/bin/bash

echo "=== HDP Disk Inspect Dependency Check ==="
echo ""

check_dependency() {
    local name=$1
    local cmd=$2
    local note=${3:-""}
    
    printf "%-20s " "$name:"
    if command -v "$cmd" >/dev/null 2>&1; then
        echo -e "\033[32m✓ Installed\033[0m"
    else
        echo -e "\033[31m✗ Missing\033[0m $note"
    fi
}

# 必需依赖
echo "Required Dependencies:"
check_dependency "smartctl" "smartctl" "for SMART data"
check_dependency "lldpd" "lldpd" "OR lldpad"
check_dependency "lldpad" "lldpad" "if no lldpd"
check_dependency "ethtool" "ethtool" "for network info"
check_dependency "dmidecode" "dmidecode" "for hardware info"
check_dependency "lspci" "lspci" "for PCI info"
check_dependency "lsblk" "lsblk" "for block devices"
echo ""

# 可选依赖
echo "Optional Dependencies:"
check_dependency "storcli64" "storcli64" "for MegaRAID"
check_dependency "lsraid" "lsraid" "for RAID info"
check_dependency "mdadm" "mdadm" "for software RAID"
echo ""

echo "Check complete!"
```

---

## 缺少依赖时的行为

如果某些依赖未安装，对应的指标将不可用，但其他功能仍继续工作：

| 缺少的依赖 | 影响 |
|-----------|------|
| `smartmontools` | 没有磁盘序列号、健康状态、错误计数 |
| `lldpd` / `lldpad` | 没有 LLDP 邻居信息 |
| `ethtool` | 网络速度/双工信息为空 |
| `storcli`/`lsraid`/`mdadm` | 没有 RAID 相关信息 |

---

## 相关文档

- [部署指南](deployment.md) - 包含更多关于部署的信息
- [故障排除](troubleshooting.md) - 诊断问题
