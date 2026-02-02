# Print Bridge 使用指南与规范

## 1. 项目简介

**Print Bridge Client** 是一个基于 Wails (Go + Vue 3) 开发的本地打印中间件。它充当浏览器（或其他客户端）与操作系统打印机之间的桥梁，通过 WebSocket 协议接收打印指令，并调用系统打印机进行打印。

主要功能：
- 自动获取操作系统已安装的打印机列表。
- 启动 WebSocket 服务监听打印请求（默认端口 1122）。
- 支持自定义服务端口和安全密钥（Secret Key）。
- 支持高级打印参数：打印份数、份数间隔、打印方向、DPI 等。
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

### 2.2 界面配置
启动后，界面提供以下配置项：
- **Port**: WebSocket 服务监听端口（默认 `1122`）。
- **Secret Key**: 安全密钥（可选）。如果设置了密钥，客户端在连接或发送请求时必须通过鉴权。
- **Start/Stop Server**: 点击按钮即可启动或停止 WebSocket 服务。
- **Connection URL**: 服务启动后，界面会显示完整的连接地址（如 `ws://localhost:1122/ws?key=...`）。

**操作说明**:
- **后台运行**: 点击窗口关闭按钮不会退出程序，而是隐藏到**系统托盘区**。
- **托盘菜单**: 在系统托盘图标上右键单击，可选择 `Show Main Window` 显示主窗口或 `Quit` 退出程序。
- **退出程序**: 若需完全退出，请使用菜单栏的 `File` -> `Quit`、快捷键 `Ctrl+Q`，或使用托盘菜单的 `Quit`。
- **查看日志**: 使用菜单栏 `File` -> `System Logs` (Ctrl+L) 可打开独立的浏览器窗口查看实时日志。

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

#### 3.2.2 发送打印任务 (Client -> Server)
客户端发送的 JSON 数据包结构如下：

```json
{
  "printer": "Microsoft Print to PDF",  // [必填] 目标打印机名称
  "content": "^XA^FO50,50^FDHello^FS^XZ", // [必填] 打印内容 (ZPL/EPL/Raw)
  "jobName": "My Print Job 001",        // [选填] 任务名称
  "key": "123456",                      // [选填] 鉴权密钥 (若连接时已验证可省略)
  "copies": 2,                          // [选填] 打印份数，默认 1
  "jobInterval": 1000,                  // [选填] 份数间延迟(毫秒)，用于手动隔张打印
  "orientation": "landscape",           // [选填] 打印方向 (portrait/landscape) *
  "dpi": 203                            // [选填] 打印精度 *
}
```

> **注意 (\*)**: `orientation` 和 `dpi` 参数仅在驱动程序支持或特定模式下生效。对于 **RAW (指令)** 打印模式（如 ZPL/EPL），建议直接在 `content` 指令中设置方向和浓度，因为 RAW 模式通常会绕过 Windows 驱动的这些设置。

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `printer` | String | 目标打印机名称。 |
| `content` | String | 原始打印数据 (ZPL, TSPL, ESC/POS 等)。 |
| `jobName` | String | 打印任务名称。 |
| `copies` | Integer | **打印份数**。服务端会循环发送指定次数的任务。 |
| `jobInterval` | Integer | **隔张间隔 (ms)**。每份打印之间的等待时间，可用于防止缓冲区溢出或手动撕纸间隔。 |
| `orientation`| String | `portrait` (纵向) 或 `landscape` (横向)。(RAW模式建议使用指令控制) |
| `dpi` | Integer | 目标打印 DPI。(RAW模式建议使用指令控制) |

#### 3.2.3 服务端响应 (Server -> Client)
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
  "message": "Printer not found"
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
        
        // 发送打印任务
        socket.send(JSON.stringify({
            printer: msg.data[0],
            content: "RAW DATA HERE...",
            copies: 2,           // 打印 2 份
            jobInterval: 500     // 间隔 0.5 秒
        }));
    } else {
        console.log('收到回复:', msg);
    }
};
```
