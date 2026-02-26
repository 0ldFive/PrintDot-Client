# PrintDot Client

**中文** | [English](README_EN.md)

<img src="build/appicon.png" alt="PrintDot Client Logo" width="96" />

## 简介

PrintDot Client 是一款基于 Wails 与 Vue 的桌面打印助手，主打“稳定、快速、好上手”。它将设备发现、连接管理与转发能力打包到一个轻量客户端里，让你用更少的配置成本，获得更高的打印链路稳定性与可用性。本项目是 [Vue Print Designer](https://github.com/0ldFive/Vue-Print-Designer) 的配套客户端。
## 界面预览

<table>
  <tr>
    <td align="center">
      <img src="docs/images/1.png" width="300" alt="主界面" /><br />
      <em>主界面 - 设备状态与连接管理</em>
    </td>
    <td align="center">
      <img src="docs/images/2.png" width="300" alt="设置页面" /><br />
      <em>设置页面 - 偏好与配置选项</em>
    </td>
  </tr>
</table>
## 优势

- 秒级启动与响应，日常操作几乎零等待
- 稳定可靠的发现与转发链路，长时间运行也很安心
- 跨平台一致体验，减少环境差异带来的折腾
- 轻量架构、低资源占用，老机器也能顺滑跑
- 细节打磨的设置与多语言体验，新手上手更快
- 现代化界面与清晰信息层级，关键状态一眼可见

## 支持平台

- Windows
- macOS
- Linux

## 功能概览

- 自动发现与识别本地/网络设备
- 稳定的连接维护与转发队列
- 简洁的可视化状态与告警提示
- 多语言界面与基础偏好设置
- 适合长期后台运行的轻量模式

## 架构与模块

- 前端：Vue 3 + Vite + Tailwind，负责界面与交互
- 桌面容器：Wails，提供跨平台窗口与系统能力
- 后端：Go 服务层，负责发现、连接、转发与配置

## 安装与运行

### 开发模式

1. 安装 Wails 与 Node.js 依赖
2. 运行开发命令

```bash
wails dev
```

### 生产构建

```bash
wails build
```

#### Windows

```bash
wails build -clean -nsis
```

#### macOS

```bash
wails build -clean -platform darwin/amd64
wails build -clean -platform darwin/arm64
```

#### Linux

```bash
wails build -clean -platform linux/amd64
```

## 配置说明

- 配置文件由应用自动生成并维护
- 可在设置页中调整设备与转发相关选项
- 修改配置后即时生效，无需重启

## 常见问题

**Q: 设备没有出现或连接不稳定怎么办？**

- 请检查同一网络与防火墙放行
- 重启客户端后重新发现
- 若仍异常，请参考使用手册排查

**Q: 是否支持后台常驻？**

- 支持，应用优化了低资源占用与持续转发

## 贡献与开发

- 欢迎提交 Issue 与 Pull Request
- 建议先阅读使用手册与配置说明，保持一致的行为与体验

## 使用手册

- 中文: [docs/usage_guide_zh.md](docs/usage_guide_zh.md)
- English: [docs/usage_guide_en.md](docs/usage_guide_en.md)
