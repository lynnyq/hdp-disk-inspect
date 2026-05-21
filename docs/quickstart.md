# 快速入门指南

本指南将帮助你快速开始使用 HDP Disk Inspect。

## 目录

1. [安装](#安装)
2. [运行](#运行)
3. [配置 Prometheus 抓取](#配置-prometheus-抓取)
4. [验证指标](#验证指标)

## 安装

### 从源码编译

```bash
# 克隆仓库
git clone https://github.com/lynnyq/hdp-disk-inspect.git
cd hdp-disk-inspect

# 编译
go build -o hdp-disk-inspect .

# 验证编译成功
./hdp-disk-inspect -v
```

### 安装依赖软件包

根据你的系统安装以下软件包：

**Debian/Ubuntu**:
```bash
sudo apt update
sudo apt install -y \
    smartmontools \
    lldpd \
    ethtool \
    dmidecode \
    pciutils \
    util-linux  # lsblk
```

**CentOS/RHEL**:
```bash
sudo yum install -y \
    smartmontools \
    lldpad \
    ethtool \
    dmidecode \
    pciutils \
    util-linux  # lsblk
```

**可选依赖**：
```bash
# 如果你使用 MegaRAID 硬件 RAID
# 下载并安装 storcli：https://www.broadcom.com/site-search?q=storcli
```

## 运行

### 基本运行

```bash
# 监听所有网卡，使用默认端口 58002
./hdp-disk-inspect

# 或者指定监听地址
./hdp-disk-inspect -s 192.168.1.100:58002
```

### 运行参数

```bash
# 查看所有参数
./hdp-disk-inspect -help

# 启用详细日志
./hdp-disk-inspect -log-level debug

# 允许以 root 运行（不推荐）
./hdp-disk-inspect -allow-root

# 查看版本
./hdp-disk-inspect -v
```

### 使用 systemd 管理

创建 `/etc/systemd/system/hdp-disk-inspect.service`:

```ini
[Unit]
Description=HDP Disk Inspect - Server Hardware and Network Monitor
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=/opt/hdp-disk-inspect
ExecStart=/opt/hdp-disk-inspect/hdp-disk-inspect -s 0.0.0.0:58002 -log-level info
Restart=always
RestartSec=5

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/tmp

[Install]
WantedBy=multi-user.target
```

启用并启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable hdp-disk-inspect --now
sudo systemctl status hdp-disk-inspect
```

## 配置 Prometheus 抓取

编辑你的 `prometheus.yml`：

```yaml
scrape_configs:
  - job_name: 'hdp-disk-inspect'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:58002']
        labels:
          instance: 'your-server-name'
```

重载 Prometheus 配置：

```bash
curl -X POST http://prometheus-server:9090/-/reload
```

## 验证指标

### 检查指标端点

```bash
curl http://localhost:58002/metrics
```

### 使用 Prometheus 查询

在 Prometheus UI 中尝试以下查询：

```promql
# 查看所有磁盘信息
node_disk_info

# 查看网络接收流量
node_network_receive_bytes_total

# 查看 LLDP 邻居
lldp_neighbor_info
```

## 下一步

- 查看 [指标说明文档](metrics.md) 了解更多指标详情
- 阅读 [部署指南](deployment.md) 了解生产环境部署
- 配置 [TLS 双向认证](tls.md) 保护 gRPC 通信
