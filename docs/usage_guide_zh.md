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

#### 3.2.3 获取打印机能力 (Client -> Server)
客户端可以请求指定打印机的可用参数：
```json
{
  "type": "get_printer_caps",
  "printer": "Microsoft Print to PDF"
}
```
服务端响应示例：
```json
{
  "type": "printer_caps",
  "printer": "Microsoft Print to PDF",
  "data": {
    "paperSizes": ["A4", "Letter"],
    "printerPaperNames": ["A4", "Letter"],
    "duplexSupported": false,
    "colorSupported": true
  }
}
```
> 说明：不同系统返回字段会有差异，Windows 返回 Win32_Printer/Win32_PrinterConfiguration 信息；Linux/macOS 返回 lpoptions 解析结果。

#### 3.2.4 发送打印任务 (Client -> Server)
客户端发送的 JSON 数据包结构如下（按功能分类）：

```json
{
  "printer": "Microsoft Print to PDF",  // [必填] 目标打印机名称
  "content": "data:application/pdf;base64,JVBERi...", // [必填] Base64 编码的 PDF 内容 (支持带前缀或纯 Base64)
  "key": "123456",                      // [选填] 鉴权密钥 (若连接时已验证可省略)
  "job": {
    "name": "My Print Job 001",        // [选填] 任务名称 (仅用于日志记录)
    "copies": 2,                         // [选填] 打印份数，默认 1
    "intervalMs": 1000                   // [选填] 份数间延迟(毫秒)，用于手动隔张打印
  },
  "pages": {
    "range": "1-3,5",                  // [选填] 页码范围 (支持 N / N-M / N,M / 反向区间)
    "set": "odd"                       // [选填] odd | even
  },
  "layout": {
    "scale": "fit",                    // [选填] noscale | shrink | fit
    "orientation": "portrait"          // [选填] portrait | landscape
  },
  "color": {
    "mode": "color"                    // [选填] color | monochrome
  },
  "sides": {
    "mode": "duplex"                   // [选填] simplex | duplex | duplexshort | duplexlong
  },
  "paper": {
    "size": "A4"                       // [选填] A4 | letter | legal | tabloid | statement | A2 | A3 | A5 | A6
  },
  "tray": {
    "bin": "2"                         // [选填] 纸盒编号或名称，例如 2 / Manual
  },
  "sumatra": {
    "settings": "fit,duplex"           // [选填] SumatraPDF 原生 -print-settings 字符串 (Windows)
  }
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
| `job.name` | String | 打印任务名称。 |
| `job.copies` | Integer | **打印份数**。`intervalMs` 为 0 时由系统命令一次性处理。 |
| `job.intervalMs` | Integer | **隔张间隔 (ms)**。大于 0 时服务端逐份打印。 |
| `pages.range` | String | **页码范围**：`N` / `N-M` / `N,M` / 反向区间。 |
| `pages.set` | String | **奇偶页**：`odd` / `even`。 |
| `layout.scale` | String | **缩放**：`noscale` / `shrink` / `fit`。 |
| `layout.orientation` | String | **方向**：`portrait` / `landscape`。 |
| `color.mode` | String | **颜色模式**：`color` / `monochrome`。 |
| `sides.mode` | String | **单双面**：`simplex` / `duplex` / `duplexshort` / `duplexlong`。 |
| `paper.size` | String | **纸张**：如 `A4`、`letter`、`legal`。未提供时会尝试从 PDF 的 MediaBox 自动识别常见尺寸。 |
| `tray.bin` | String | **纸盒**：编号或名称。 |
| `sumatra.settings` | String | **SumatraPDF 原生设置**（Windows），会直接传给 `-print-settings`。 |

#### 3.2.3.1 平台支持说明
**Windows (SumatraPDF)**
- `pages.range` / `pages.set` / `layout.scale` / `layout.orientation` / `color.mode` / `sides.mode` / `paper.size` / `tray.bin` / `job.copies` 会被转换为 `-print-settings`。
- `sumatra.settings` 提供时，自动转换会被覆盖。

**Linux/macOS (lp/CUPS)**
- `pages.range` -> `-P`。
- `pages.set` -> `-o page-set=odd|even`。
- `layout.scale` -> `fit-to-page` / `scaling=100`（`shrink` 使用默认行为）。
- `layout.orientation` -> `-o orientation-requested=3|4`（驱动可能忽略）。
- `color.mode` -> `-o ColorModel=Gray` / `-o ColorModel=RGB`（部分驱动可能忽略）。
- `sides.mode` -> `-o sides=...`。
- `paper.size` -> `-o media=...`。
- `tray.bin` -> `-o InputSlot=...`（依赖驱动支持）。
- `sumatra.settings` 不适用于 Linux/macOS。

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

        const targetPrinter = msg.data[0];

        // 示例：主动刷新打印机列表
        // socket.send(JSON.stringify({ type: 'get_printers' }));

        // 获取打印机能力（纸张/单双面/彩色等）
        socket.send(JSON.stringify({
          type: 'get_printer_caps',
          printer: targetPrinter
        }));
    } else if (msg.type === 'printer_caps') {
        const caps = msg.data || {};
        const sizes = caps.printerPaperNames || caps.paperSizes || [];
        const paperSize = sizes[0] || 'A4';
        
        // 发送打印任务（按功能分组）
        socket.send(JSON.stringify({
          printer: msg.printer,
          content: "JVBERi0xLjQKJ...", // Base64 PDF Data
          job: {
            name: "Test Job",
            copies: 2,
            intervalMs: 0
          },
          pages: {
            range: "1-3,5",
            set: "odd"
          },
          layout: {
            scale: "fit",
            orientation: "portrait"
          },
          color: {
            mode: "color"
          },
          sides: {
            mode: "duplex"
          },
          paper: {
            size: paperSize
          },
          tray: {
            bin: "2"
          }
        }));
    } else {
        console.log('收到回复:', msg);
    }
};
```
