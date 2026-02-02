# PrintDot Client Usage Guide

## 1. Introduction

**PrintDot Client** is a local printing middleware developed based on Wails (Go + Vue 3). It acts as a bridge between browsers (or other clients) and the operating system's printers, receiving print instructions via the WebSocket protocol and invoking the system printer for printing.

Key Features:
- Automatically retrieves the list of installed printers in the operating system.
- Starts a WebSocket service to listen for print requests (default port 1122).
- Supports custom service ports and security keys (Secret Key).
- Supports advanced print parameters: copies, interval between copies, orientation, DPI, etc.
- Provides a visual management interface to view logs and printer status in real-time.
- **Independent Log Window**: Supports viewing real-time system logs in a separate window.

---

## 2. Startup and Configuration

### 2.1 Starting the Application
You can run the compiled executable file (e.g., `print-dot-client.exe`) directly.

**Note**: The program automatically starts the WebSocket service (default port 1122) upon startup.

### 2.2 Interface Configuration
After startup, the interface provides the following configuration options:
- **Port**: WebSocket service listening port (default `1122`).
- **Secret Key**: Security key (optional). If a key is set, clients must authenticate when connecting or sending requests.
- **Start/Stop Server**: Click the button to start or stop the WebSocket service.
- **Connection URL**: After the service starts, the interface displays the full connection address (e.g., `ws://localhost:1122/ws?key=...`).

**Operation Instructions**:
- **Exit Program**: Click the main window close button (X), use the menu bar `Menu` -> `Quit` (Ctrl+Q), or use the tray menu `Quit` to completely exit the program (including the log child window).
- **Run in Background**: After startup, an icon appears in the system tray area, which can be used to quickly recall the window or exit.
- **Tray Menu**: Right-click the system tray icon to select `Show Main Window` or `Quit`.
- **View Logs**: Use the menu bar `Menu` -> `System Logs` (Ctrl+L) to open an independent log window to view real-time logs.

---

## 3. WebSocket Interface Specification

### 3.1 Connection Information
- **Protocol**: WebSocket (`ws://`)
- **Address**: `ws://localhost:<PORT>/ws`
- **Authentication**:
  - If a `Secret Key` is set, it is recommended to carry it in the connection URL: `ws://localhost:1122/ws?key=YOUR_PASSWORD`
  - If the key is not carried during connection, it can also be included in the message body sent (but not recommended, as the connection might be rejected).

### 3.2 Message Types

#### 3.2.1 Connection Success Response (Server -> Client)
After the connection is established, the server immediately sends the current printer list:
```json
{
  "type": "printer_list",
  "data": ["Microsoft Print to PDF", "ZDesigner GK888t", ...]
}
```

#### 3.2.2 Get Printer List (Client -> Server)
The client can actively request the latest printer list at any time by sending the following JSON message:
```json
{
  "type": "get_printers"
}
```
The server will reply with a `printer_list` message in the same format as **3.2.1**.

#### 3.2.3 Send Print Job (Client -> Server)
The JSON data packet structure sent by the client is as follows:

```json
{
  "printer": "Microsoft Print to PDF",  // [Required] Target printer name
  "content": "^XA^FO50,50^FDHello^FS^XZ", // [Required] Print content (ZPL/EPL/Raw)
  "jobName": "My Print Job 001",        // [Optional] Job name
  "key": "123456",                      // [Optional] Auth key (can be omitted if verified during connection)
  "copies": 2,                          // [Optional] Number of copies, default 1
  "jobInterval": 1000,                  // [Optional] Delay between copies (ms), used for manual interval
  "orientation": "landscape",           // [Optional] Print orientation (portrait/landscape) *
  "dpi": 203                            // [Optional] Print DPI *
}
```

> **Note (\*)**: `orientation` and `dpi` parameters only take effect if the driver supports them or in specific modes. For **RAW (Instruction)** print mode (such as ZPL/EPL), it is recommended to set the orientation and density directly in the `content` instruction, as RAW mode usually bypasses these settings in the Windows driver.

| Field | Type | Description |
| :--- | :--- | :--- |
| `printer` | String | Target printer name. |
| `content` | String | Raw print data (ZPL, TSPL, ESC/POS, etc.). |
| `jobName` | String | Print job name. |
| `copies` | Integer | **Number of copies**. The server sends the task the specified number of times. |
| `jobInterval` | Integer | **Interval (ms)**. Wait time between each copy, used to prevent buffer overflow or manual tear-off interval. |
| `orientation`| String | `portrait` or `landscape`. (RAW mode recommended using instructions) |
| `dpi` | Integer | Target print DPI. (RAW mode recommended using instructions) |

#### 3.2.4 Server Response (Server -> Client)
The server returns the result of each print:

**Success Response:**
```json
{
  "status": "success",
  "message": "Printed successfully"
}
```

### 3.3 Client Code Example

```javascript
socket.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'printer_list') {
        console.log('Available printers:', msg.data);
        
        // Example: Actively refresh printer list
        // socket.send(JSON.stringify({ type: 'get_printers' }));

        // Send print job
        socket.send(JSON.stringify({
            printer: msg.data[0],
            content: "^XA^FO50,50^FDHello^FS^XZ"
        }));
    }
};
```
