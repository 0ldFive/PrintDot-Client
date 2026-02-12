# PrintDot Client 使用指南与规范

## 1. 项目简介

**PrintDot Client** 是一个基于 Wails (Go + Vue 3) 开发的本地打印中间件。它充当浏览器（或其他客户端）与操作系统打印机之间的桥梁，通过 WebSocket 协议接收打印指令，并调用系统打印机进行打印。

主要功能：
- 自动获取操作系统已安装的打印机列表。
- 启动 WebSocket 服务监听打印请求（默认端口 1122）。
- 支持自定义服务端口和安全密钥（Secret Key）。
- **仅支持 PDF 打印**：接收 Base64 编码的 PDF 文件内容，并调用系统命令（Windows 使用 SumatraPDF 无感打印 / Unix `lp`）进行打印。
- 支持高级打印参数：打印份数、份数间隔，以及更多打印设置。
- 提供可视化的管理界面，实时查看日志和打印机状态。
- **独立日志窗口**：支持在浏览器中查看实时系统日志。

---

## 2. 启动与配置

### 2.1 启动应用
可以直接运行编译后的可执行文件（如 `print-dot-client.exe`），或在开发环境中使用：

```bash
wails dev
```

**注意**: 程序启动时会自动开启 WebSocket 服务（默认端口 1122）。

**Windows 打印说明**:
- Windows 端使用 **SumatraPDF** 静默打印 PDF。
- 请将 `SumatraPDF.exe` 放在程序同目录，或加入系统 `PATH`，或设置环境变量 `SUMATRAPDF_PATH` 指向该文件。

### 2.2 界面配置
启动后，界面提供以下配置项：
- **Port**: WebSocket 服务监听端口（默认 `1122`）。
- **Secret Key**: 安全密钥（可选）。如果设置了密钥，客户端在连接或发送请求时必须通过鉴权。
- **Start/Stop Server**: 点击按钮即可启动或停止 WebSocket 服务。
- **Connection URL**: 服务启动后，界面会显示完整的连接地址（如 `ws://localhost:1122/ws?key=...`）。

**操作说明**:
- **退出程序**: 使用菜单栏 `Menu` -> `Quit` (Ctrl+Q)，或使用托盘菜单的 `Quit` 可完全退出程序。
- **后台运行**: 点击主窗口关闭按钮 (X) 不会退出程序，而是将程序最小化到系统托盘区。程序启动后会在系统托盘区显示图标，可用于快速唤起窗口或退出。
- **托盘菜单**: 在系统托盘图标上右键单击，可选择 `Show Main Window` 显示主窗口或 `Quit` 退出程序。
- **打开设置**: 使用菜单栏 `Menu` -> `Settings` (Ctrl+I) 可打开设置窗口。
- **查看日志**: 使用菜单栏 `Menu` -> `System Logs` (Ctrl+L) 可打开独立的日志窗口查看实时日志.

---

## 3. WebSocket 接口规范

### 3.1 连接信息
- **协议**: WebSocket (`ws://`)
- **地址**: `ws://localhost:<PORT>/ws`
- **鉴权**: 
  - 如果设置了 `Secret Key`，建议在连接 URL 中携带：`ws://localhost:1122/ws?key=YOUR_PASSWORD`
  - 如果连接时未携带 key，也可以在发送的消息体中包含 `key` 字段（但不推荐，连接可能被拒绝）。

### 3.2 消息类型

#### 3.2.1 连接成功响应 (Server -> Client)
连接建立后，服务端会立即发送当前的打印机列表：
```json
{
  "type": "printer_list",
  "data": ["Microsoft Print to PDF", "ZDesigner GK888t", ...]
}
```

#### 3.2.2 获取打印机列表 (Client -> Server)
客户端可以随时发送以下 JSON 消息主动获取最新的打印机列表：
```json
{
  "type": "get_printers"
}
```
服务端将回复与 **3.2.1** 相同格式的 `printer_list` 消息。

#### 3.2.3 发送打印任务 (Client -> Server)
客户端发送的 JSON 数据包结构如下：

```json
{
  "printer": "Microsoft Print to PDF",  // [必填] 目标打印机名称
  "content": "data:application/pdf;base64,JVBERi...", // [必填] Base64 编码的 PDF 内容 (支持带前缀或纯 Base64)
  "jobName": "My Print Job 001",        // [选填] 任务名称 (仅用于日志记录)
  "key": "123456",                      // [选填] 鉴权密钥 (若连接时已验证可省略)
  "copies": 2,                          // [选填] 打印份数，默认 1
  "jobInterval": 1000,                  // [选填] 份数间延迟(毫秒)，用于手动隔张打印
  "pageRange": "1-3,5",                // [选填] 打印页码范围 (Linux/macOS 支持；Windows 需配合 printSettings)
  "duplex": "long-edge",               // [选填] 双面: simplex | long-edge | short-edge
  "colorMode": "mono",                 // [选填] 颜色: color | mono
  "paper": "A4",                        // [选填] 纸张: A4 | Letter | ...
  "scale": "fit",                      // [选填] 缩放: fit | shrink | none
  "printSettings": "fit,duplex"        // [选填] SumatraPDF 原生 -print-settings 字符串 (Windows)
}
```

> **注意**: 
> 1. `content` 字段必须是 **PDF 文件的 Base64 编码字符串**。
>    - 支持标准 Data URI 格式：`data:application/pdf;base64,JVBERi...`
>    - 也支持纯 Base64 字符串：`JVBERi...`
>    - 服务端会自动去除 `data:` 前缀（如果有）并校验解码后的内容是否以 `%PDF` 开头。

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `printer` | String | 目标打印机名称。 |
| `content` | String | **Base64 编码的 PDF 内容**。 |
| `jobName` | String | 打印任务名称。 |
| `copies` | Integer | **打印份数**。服务端会循环调用系统打印命令指定次数。 |
| `jobInterval` | Integer | **隔张间隔 (ms)**。每份打印之间的等待时间。 |
| `pageRange` | String | **页码范围**。例如 `1-3,5`。Linux/macOS 支持；Windows 需配合 `printSettings`。 |
| `duplex` | String | **双面设置**：`simplex` / `long-edge` / `short-edge`。 |
| `colorMode` | String | **颜色模式**：`color` / `mono`。 |
| `paper` | String | **纸张大小**：如 `A4`、`Letter`。 |
| `scale` | String | **缩放策略**：`fit` / `shrink` / `none`。 |
| `printSettings` | String | **SumatraPDF 原生设置**（Windows）。会直接传给 `-print-settings`。 |

#### 3.2.4 服务端响应 (Server -> Client)
服务端会返回每次打印的结果：

**成功响应:**
```json
{
  "status": "success",
  "message": "Printed successfully"
}
```

**失败响应:**
```json
{
  "status": "error",
  "message": "Content must be a PDF file"
}
```

---

## 4. 调用示例 (JavaScript)

```javascript
const socket = new WebSocket('ws://localhost:1122/ws?key=123456');

socket.onopen = () => {
    console.log('已连接');
};

socket.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'printer_list') {
        console.log('可用打印机:', msg.data);

        // 示例：主动刷新打印机列表
        // socket.send(JSON.stringify({ type: 'get_printers' }));
        
        // 发送打印任务
        socket.send(JSON.stringify({
            printer: msg.data[0],
            content: "JVBERi0xLjQKJ...", // Base64 PDF Data
            copies: 1
        }));
    } else {
        console.log('收到回复:', msg);
    }
};
```
