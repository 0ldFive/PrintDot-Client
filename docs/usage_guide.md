# Print Bridge 使用指南与规范

## 1. 项目简介

**Print Bridge Client** 是一个基于 Wails (Go + Vue 3) 开发的本地打印中间件。它充当浏览器（或其他客户端）与操作系统打印机之间的桥梁，通过 WebSocket 协议接收打印指令，并调用系统打印机进行打印。

主要功能：
- 自动获取操作系统已安装的打印机列表。
- 启动 WebSocket 服务监听打印请求（默认端口 1122）。
- 支持自定义服务端口和安全密钥（Secret Key）。
- 提供可视化的管理界面，实时查看日志和打印机状态。

---

## 2. 启动与配置

### 2.1 启动应用
可以直接运行编译后的可执行文件（如 `print-dot-client.exe`），或在开发环境中使用：

```bash
wails dev
```

### 2.2 界面配置
启动后，界面提供以下配置项：
- **Port**: WebSocket 服务监听端口（默认 `1122`）。
- **Secret Key**: 安全密钥（可选）。如果设置了密钥，客户端在发送打印请求时必须携带正确的密钥，否则将被拒绝。
- **Start/Stop Server**: 点击按钮即可启动或停止 WebSocket 服务。

---

## 3. WebSocket 接口规范

### 3.1 连接信息
- **协议**: WebSocket (`ws://`)
- **地址**: `ws://localhost:<PORT>/ws`
- **默认地址**: `ws://localhost:1122/ws`

### 3.2 通信协议
所有数据交互均使用 JSON 格式。

### 3.3 发送打印任务 (Client -> Server)
客户端发送的 JSON 数据包结构如下：

```json
{
  "printer": "Microsoft Print to PDF",  // [必填] 目标打印机名称 (需与系统名称完全一致)
  "content": "^XA^FO50,50^ADN,36,20^FDHello World^FS^XZ", // [必填] 打印内容 (ZPL, ESC/P 指令或纯文本)
  "jobName": "My Print Job 001",        // [选填] 任务名称，显示在系统打印队列中
  "key": "123456"                       // [选填] 如果服务端设置了密钥，此处必须匹配
}
```

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `printer` | String | 目标打印机名称。可以通过界面查看可用打印机列表。 |
| `content` | String | 原始打印数据。通常是打印机指令集（如 ZPL, TSPL, ESC/POS）或纯文本。 |
| `jobName` | String | 打印任务名称，默认为 "Raw Print Job"。 |
| `key` | String | 鉴权密钥。如果服务端未设置密钥，此字段可忽略。 |

### 3.4 服务端响应 (Server -> Client)
服务端会返回操作结果：

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
  "message": "Invalid Key" // 或其他错误信息，如 "Printer not found"
}
```

---

## 4. 调用示例 (JavaScript)

以下是一个在浏览器端调用打印服务的简单示例：

```javascript
const socket = new WebSocket('ws://localhost:1122/ws');

socket.onopen = () => {
    console.log('已连接到打印服务');

    const printJob = {
        printer: "ZDesigner GK888t",
        content: "Hello World", // 实际场景请替换为具体的打印机指令
        jobName: "Test Job",
        key: "" 
    };

    socket.send(JSON.stringify(printJob));
};

socket.onmessage = (event) => {
    const response = JSON.parse(event.data);
    if (response.status === 'success') {
        console.log('打印成功');
    } else {
        console.error('打印失败:', response.message);
    }
};

socket.onerror = (error) => {
    console.error('连接错误:', error);
};
```

## 5. 注意事项
1. **打印机名称**: 必须与操作系统中显示的名称完全一致（区分大小写）。
2. **驱动程序**: 确保操作系统已安装正确的打印机驱动。
3. **防火墙**: 如果需要跨局域网访问（非 localhost），请确保防火墙允许该端口（如 1122）的入站连接，并在启动时监听 `0.0.0.0` 而非仅 `localhost`（目前版本默认监听所有网卡）。
