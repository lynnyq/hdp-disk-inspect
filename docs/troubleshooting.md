# 故障排除

本文档提供常见问题的诊断和解决方案。

## 目录

1. [服务无法启动](#服务无法启动)
2. [指标采集问题](#指标采集问题)
3. [磁盘指标问题](#磁盘指标问题)
4. [网络指标问题](#网络指标问题)
5. [LLDP 指标问题](#lldp-指标问题)
6. [TLS/认证问题](#tls认证问题)
7. [性能问题](#性能问题)

---

## 服务无法启动

### 问题：服务立即退出

**检查日志**:

```bash
sudo journalctl -u hdp-disk-inspect -n 50
```

**可能原因**:

1. **以 root 运行但没有 `-allow-root`**
   - 错误: `Do not run hdp-disk-inspect as root user`
   - 解决: 使用非 root 用户或添加 `-allow-root`（不推荐）

2. **端口被占用**
   - 错误: `listen tcp :58002: bind: address already in use`
   - 解决:
     ```bash
     # 查找占用端口的进程
     sudo lsof -i :58002
     sudo ss -tulnp | grep 58002
     
     # 或使用其他端口
     ./hdp-disk-inspect -s 0.0.0.0:58003
     ```

3. **证书文件不存在或权限错误**
   - 错误: `failed to read ca cert file`
   - 解决:
     ```bash
     # 检查证书文件存在
     ls -la certs/ca/ certs/server/
     
     # 检查权限
     sudo chown hdp-disk-inspect:hdp-disk-inspect /opt/hdp-disk-inspect/certs -R
     sudo chmod 400 /opt/hdp-disk-inspect/certs/server/server.key
     ```

---

## 指标采集问题

### 问题：Prometheus 无法抓取指标

**检查服务是否运行**:

```bash
# 检查进程
ps aux | grep hdp-disk-inspect

# 检查端口
sudo ss -tulnp | grep 58002

# 本地测试
curl http://localhost:58002/metrics
```

**检查防火墙**:

```bash
# UFW
sudo ufw status

# firewalld
sudo firewall-cmd --list-all

# iptables
sudo iptables -L -n
```

**检查 Prometheus 日志**:

```bash
# 查看 Prometheus 目标状态
# Prometheus UI -> Status -> Targets
```

### 问题：指标端点返回空指标

**检查指标采集是否正常**:

```bash
# 查看服务日志
sudo journalctl -u hdp-disk-inspect -f -n 100

# 启用 debug 日志
./hdp-disk-inspect -log-level debug
```

---

## 磁盘指标问题

### 问题：没有磁盘指标

**检查依赖软件**:

```bash
# 检查 sysfs
ls -la /sys/block/

# 检查 smartctl
smartctl --version
```

**检查是否有块设备**:

```bash
lsblk
```

**手动测试采集**:

```bash
# 检查磁盘信息
ls -la /sys/block/sd*/ 2>/dev/null
```

### 问题：磁盘序列号为空

**可能原因**:
- 某些虚拟磁盘没有序列号
- 权限问题导致无法读取
- 设备名称不是 sd 开头（可能是 nvme、xd 等）

**解决**:

```bash
# 尝试用 smartctl 读取
sudo smartctl -i /dev/sda

# 检查 sysfs
cat /sys/block/sda/device/serial 2>/dev/null
```

### 问题：没有 RAID 信息

**检查 RAID 工具**:

```bash
# MegaRAID
storcli64 /c0 show all 2>/dev/null || echo "storcli not found"

# MDADM
cat /proc/mdstat

# lsraid
lsraid -A -p 2>/dev/null
```

---

## 网络指标问题

### 问题：没有网络指标

**检查**:

```bash
# 检查 /sys/class/net
ls -la /sys/class/net/

# 检查网络接口
ip link show
```

**常见原因**:
- 使用容器网络，没有挂载 /sys
- 网络接口命名特殊

### 问题：Bonding 指标缺失

**检查 bonding**:

```bash
# 检查 bonding masters
cat /sys/class/net/bonding_masters 2>/dev/null

# 检查 bonding 目录
ls -la /sys/class/net/*/bonding/ 2>/dev/null
```

**示例**:

```bash
# 查看 bonding 状态
cat /proc/net/bonding/bond0
```

### 问题：网络信息（速度/双工）为空

**使用 ethtool 诊断**:

```bash
# 检查 ethtool 是否可用
ethtool --version

# 检查接口信息
sudo ethtool eth0
```

---

## LLDP 指标问题

### 问题：没有 LLDP 指标

**检查 LLDP 服务**:

```bash
# 检查 lldpd
systemctl status lldpd
systemctl status lldpad

# 如果没有运行，启动
sudo systemctl start lldpd
sudo systemctl enable lldpd
```

**启用 LLDP 在网卡上**:

```bash
# lldpd 配置
sudo lldpcli configure ports eth0 lldp status rx-tx

# lldpad 配置
sudo lldptool set-lldp -i eth0 adminStatus=rxtx
```

**验证 LLDP 邻居**:

```bash
# 使用 lldpcli
sudo lldpcli show neighbors

# 使用 lldptool
sudo lldptool get-tlv -i eth0 -n -V neighbor
```

---

## TLS/认证问题

### 问题：TLS 握手失败

**客户端错误**:
```
rpc error: code = Unavailable desc = connection closed before server preface received
```

**服务器错误**:
```
http: TLS handshake error from xxx: tls: client didn't provide a certificate
```

**检查**:

```bash
# 1. 验证证书
openssl verify -CAfile certs/ca/ca.crt certs/server/server.crt
openssl verify -CAfile certs/ca/ca.crt certs/client/client.crt

# 2. 检查服务器证书的 SAN
openssl x509 -in certs/server/server.crt -noout -text | grep -A 10 "Subject Alternative Name"

# 3. 检查使用的地址是否在 SAN 中
# 如果用 192.168.1.100 连接，确保 SAN 包含 IP.1 = 192.168.1.100

# 4. 检查证书有效期
openssl x509 -in certs/server/server.crt -noout -dates
```

### 问题：权限被拒绝

**错误**:
```
rpc error: code = PermissionDenied desc = ...
```

**检查**:

```bash
# 检查客户端证书是否被正确加载
# 检查 CA 是否正确验证了客户端证书
```

---

## 性能问题

### 问题：CPU/内存占用高

**检查**:

```bash
# 查看资源使用
top -p $(pgrep -f hdp-disk-inspect)
ps aux | grep hdp-disk-inspect
```

**可能原因**:
- 指标采集过于频繁（Prometheus scrape interval）
- 系统调用频繁

**解决**:

```yaml
# 调整 Prometheus 抓取间隔
scrape_configs:
  - job_name: 'hdp-disk-inspect'
    scrape_interval: 30s  # 从 15s 增加到 30s
```

---

## 常见诊断命令

### 检查服务状态

```bash
# 查看服务状态
sudo systemctl status hdp-disk-inspect

# 查看服务日志
sudo journalctl -u hdp-disk-inspect -n 100 -f
```

### 检查指标

```bash
# 检查指标端点
curl http://localhost:58002/metrics

# 只看特定指标
curl http://localhost:58002/metrics | grep -E "node_disk_info|node_network_info|lldp_neighbor_info"
```

### 检查依赖

```bash
# 检查所有依赖是否安装
which smartctl lldpcli lldptool ethtool dmidecode lspci lsblk
```

### 网络诊断

```bash
# 检查端口监听
ss -tulnp

# 检查网络连接
ss -tun

# 防火墙测试
nc -zv localhost 58002
```

---

## 日志级别

启用 debug 模式可以看到更多诊断信息：

```bash
./hdp-disk-inspect -log-level debug
```

---

## 提交问题报告

如果以上方法无法解决问题，请在提交 Issue 时提供：

1. **操作系统信息**:
   ```bash
   cat /etc/os-release
   uname -a
   ```

2. **HDP Disk Inspect 版本**:
   ```bash
   ./hdp-disk-inspect -v
   ```

3. **相关日志**:
   ```bash
   sudo journalctl -u hdp-disk-inspect --no-pager -n 200
   ```

4. **指标端点输出**:
   ```bash
   curl http://localhost:58002/metrics
   ```

5. **依赖软件信息**:
   ```bash
   dpkg -l smartmontools lldpd 2>/dev/null || rpm -q smartmontools lldpad 2>/dev/null
   ```
