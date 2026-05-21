# HDP Disk Inspect 文档

欢迎使用 HDP Disk Inspect 文档！

## 文档索引

### 入门

- [快速开始](quickstart.md) - 快速安装和运行指南
- [部署指南](deployment.md) - 生产环境部署的完整说明

### 功能说明

- [指标说明](metrics.md) - 详细的 Prometheus 指标文档
- [API 参考](api.md) - gRPC API 文档和客户端示例

### 高级主题

- [TLS 双向认证配置](tls.md) - 安全通信配置
- [依赖软件](dependencies.md) - 指标采集所需的软件包
- [故障排除](troubleshooting.md) - 常见问题解决方案

## 项目概述

HDP Disk Inspect 是一个服务器硬件和网络监控工具，提供：

- **磁盘 RAID 指标**: 磁盘设备信息、健康状态、RAID 信息
- **网络指标**: 接口流量、Bonding 状态、接口信息
- **LLDP 指标**: 网络邻居发现信息
- **gRPC 远程命令**: 安全的远程 shell 命令执行

## 快速导航

- 第一次使用？请查看 [快速开始](quickstart.md)
- 生产环境部署？请阅读 [部署指南](deployment.md)
- 需要配置 TLS？查看 [TLS 配置](tls.md)
- 遇到问题？查看 [故障排除](troubleshooting.md)

## 文档结构

```
docs/
├── README.md          # 本文件，文档索引
├── quickstart.md      # 快速入门
├── metrics.md         # 指标说明
├── deployment.md      # 部署指南
├── api.md             # API 参考
├── tls.md             # TLS 配置
├── dependencies.md    # 依赖软件
└── troubleshooting.md # 故障排除
```

## 相关链接

- [GitHub 项目主页](https://github.com/lynnyq/hdp-disk-inspect)
- [项目主 README](../README.md)
- [License](../LICENSE) - Apache 2.0

## 贡献

欢迎提交 Issue 和 Pull Request 来改进文档！

---

*最后更新: 2024*
