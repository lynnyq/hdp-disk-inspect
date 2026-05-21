# 部署指南

本指南介绍如何在生产环境中部署 HDP Disk Inspect。

## 目录

1. [系统要求](#系统要求)
2. [依赖软件安装](#依赖软件安装)
3. [生产环境配置](#生产环境配置)
4. [防火墙配置](#防火墙配置)
5. [高可用部署](#高可用部署)

## 系统要求

### 硬件要求

- **CPU**: 1 核以上
- **内存**: 128MB 以上
- **磁盘**: 10MB 可用空间

### 软件要求

- **操作系统**: Linux (推荐: Debian 11+, Ubuntu 20.04+, RHEL/CentOS 8+)
- **Go**: 1.20+ (仅编译时需要)
- **依赖软件**: 见 [依赖软件安装](#依赖软件安装)

## 依赖软件安装

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

### CentOS/RHEL

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

### 验证依赖安装

```bash
# 验证 smartmontools
smartctl --version

# 验证 lldpd/lldpad
lldpd -v 2>/dev/null || lldpad -v

# 验证 ethtool
ethtool --version

# 验证 dmidecode
dmidecode --version

# 验证 lspci
lspci --version
```

## 生产环境配置

### 1. 创建专用用户

```bash
sudo useradd -r -s /usr/sbin/nologin -d /opt/hdp-disk-inspect hdp-disk-inspect
```

### 2. 创建安装目录

```bash
sudo mkdir -p /opt/hdp-disk-inspect
sudo chown -R hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect
```

### 3. 编译和部署二进制文件

```bash
# 在开发机器上编译（或者直接在服务器上编译）
GOOS=linux GOARCH=amd64 go build -o hdp-disk-inspect .

# 上传到服务器
scp hdp-disk-inspect user@server:/tmp/

# 在服务器上移动到安装目录
sudo mv /tmp/hdp-disk-inspect /opt/hdp-disk-inspect/
sudo chown hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/hdp-disk-inspect
sudo chmod 750 /opt/hdp-disk-inspect/hdp-disk-inspect
```

### 4. 配置 systemd 服务

创建 `/etc/systemd/system/hdp-disk-inspect.service`:

```ini
[Unit]
Description=HDP Disk Inspect - Server Hardware and Network Monitor
Documentation=https://github.com/lynnyq/hdp-disk-inspect
After=network.target

[Service]
Type=simple
User=hdp-disk-inspect
Group=hdp-disk-inspect
WorkingDirectory=/opt/hdp-disk-inspect
ExecStart=/opt/hdp-disk-inspect/hdp-disk-inspect \
    -s 0.0.0.0:58002 \
    -log-level info

# 重启配置
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=/tmp

# 资源限制
MemoryLimit=256M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

### 5. 启用和启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable hdp-disk-inspect
sudo systemctl start hdp-disk-inspect
sudo systemctl status hdp-disk-inspect
```

### 6. 查看日志

```bash
# 查看服务日志
sudo journalctl -u hdp-disk-inspect -f

# 查看最近 100 行日志
sudo journalctl -u hdp-disk-inspect -n 100
```

## 防火墙配置

### UFW (Ubuntu/Debian)

```bash
# 允许 Prometheus 指标端口 (58002)
sudo ufw allow from 192.168.1.0/24 to any port 58002 comment "Prometheus metrics"

# 如果启用了 TLS，确保防火墙规则正确
sudo ufw enable
sudo ufw status
```

### firewalld (CentOS/RHEL)

```bash
# 添加服务
sudo firewall-cmd --permanent --new-service=hdp-disk-inspect
sudo firewall-cmd --permanent --service=hdp-disk-inspect --add-port=58002/tcp
sudo firewall-cmd --permanent --service=hdp-disk-inspect --set-description="HDP Disk Inspect Metrics"

# 允许特定来源
sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="192.168.1.0/24" service name="hdp-disk-inspect" accept'

# 重载防火墙
sudo firewall-cmd --reload
sudo firewall-cmd --list-all
```

### iptables

```bash
# 允许特定来源访问 58002 端口
sudo iptables -A INPUT -p tcp -s 192.168.1.0/24 --dport 58002 -j ACCEPT
sudo iptables-save > /etc/iptables/rules.v4
```

## 高可用部署

HDP Disk Inspect 是单机工具，通常不需要复杂的高可用配置。但是你可以考虑以下方案：

### 1. 使用监控告警

在 Prometheus 和 Alertmanager 中配置告警，当服务不可用时通知管理员。

### 2. 使用 systemd 的自动重启

通过 systemd 配置实现服务崩溃自动重启。

### 3. 定期检查服务状态

```bash
# 编写健康检查脚本
cat > /opt/hdp-disk-inspect/health-check.sh << 'EOF'
#!/bin/bash
if ! curl -s -o /dev/null http://localhost:58002/metrics; then
    echo "Service not responding - restarting..."
    systemctl restart hdp-disk-inspect
fi
EOF

chmod +x /opt/hdp-disk-inspect/health-check.sh

# 添加到 cron 定期运行
sudo echo "*/5 * * * * root /opt/hdp-disk-inspect/health-check.sh" >> /etc/cron.d/hdp-disk-inspect
```

### 4. 多实例部署（可选）

如果需要在多个服务器上运行，直接在每个服务器上独立部署即可。

## Prometheus 集成配置

### Prometheus 配置

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'hdp-disk-inspect'
    metrics_path: /metrics
    static_configs:
      - targets:
          - server1:58002
          - server2:58002
          - server3:58002
        labels:
          environment: production

    # 重命名标签
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
```

### Grafana 仪表盘

可以创建 Grafana 仪表盘监控：

- 磁盘健康状态
- 网络流量
- Bonding 状态
- LLDP 邻居信息

## 备份和恢复

### 1. 备份配置

```bash
# 备份 systemd 服务文件
sudo cp /etc/systemd/system/hdp-disk-inspect.service /backup/

# 如果使用 TLS，备份证书
sudo cp -r /opt/hdp-disk-inspect/certs /backup/ 2>/dev/null
```

### 2. 恢复

```bash
# 恢复服务文件
sudo cp /backup/hdp-disk-inspect.service /etc/systemd/system/
sudo systemctl daemon-reload

# 恢复证书（如果有）
sudo cp -r /backup/certs /opt/hdp-disk-inspect/ 2>/dev/null
sudo chown -R hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/

# 重启服务
sudo systemctl restart hdp-disk-inspect
```

## 升级流程

```bash
# 1. 下载新版本
cd /tmp
GOOS=linux GOARCH=amd64 go build -o hdp-disk-inspect-new https://github.com/lynnyq/hdp-disk-inspect.git

# 2. 停止服务
sudo systemctl stop hdp-disk-inspect

# 3. 备份当前版本
sudo cp /opt/hdp-disk-inspect/hdp-disk-inspect /opt/hdp-disk-inspect/hdp-disk-inspect.backup

# 4. 部署新版本
sudo mv /tmp/hdp-disk-inspect-new /opt/hdp-disk-inspect/hdp-disk-inspect
sudo chown hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/hdp-disk-inspect
sudo chmod 750 /opt/hdp-disk-inspect/hdp-disk-inspect

# 5. 启动服务
sudo systemctl start hdp-disk-inspect

# 6. 验证
sudo systemctl status hdp-disk-inspect
```

## 安全建议

1. **不要以 root 运行**：使用专用非 root 用户运行
2. **启用 TLS**：生产环境建议启用 TLS 双向认证
3. **限制访问**：使用防火墙限制仅允许 Prometheus 访问
4. **定期更新**：及时更新到最新版本
5. **监控日志**：定期查看服务日志，及时发现异常
