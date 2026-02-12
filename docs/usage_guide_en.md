# PrintDot Client Usage Guide

## 1. Introduction

**PrintDot Client** is a local printing middleware developed based on Wails (Go + Vue 3). It acts as a bridge between browsers (or other clients) and the operating system's printers, receiving print instructions via the WebSocket protocol and invoking the system printer for printing.

Key Features:
- Automatically retrieves the list of installed printers in the operating system.
- Starts a WebSocket service to listen for print requests (default port 1122).
- Supports custom service ports and security keys (Secret Key).
- **PDF Printing Only**: Accepts Base64 encoded PDF content and invokes system commands (Windows uses SumatraPDF silent printing / Unix `lp`).
- Supports advanced print parameters: copies, interval between copies, and more print settings.
- Provides a visual management interface to view logs and printer status in real-time.
- **Independent Log Window**: Supports viewing real-time system logs in a separate window.

---

## 2. Startup and Configuration

### 2.1 Starting the Application
You can run the compiled executable file (e.g., `print-dot-client.exe`) directly.

**Note**: The program automatically starts the WebSocket service (default port 1122) upon startup.

**Windows printing note**:
- Windows uses **SumatraPDF** for silent PDF printing.
- Place `SumatraPDF.exe` next to the app, add it to `PATH`, or set the `SUMATRAPDF_PATH` environment variable.

### 2.2 Interface Configuration
After startup, the interface provides the following configuration options:
- **Port**: WebSocket service listening port (default `1122`).
- **Secret Key**: Security key (optional). If a key is set, clients must authenticate when connecting or sending requests.
- **Start/Stop Server**: Click the button to start or stop the WebSocket service.
- **Connection URL**: After the service starts, the interface displays the full connection address (e.g., `ws://localhost:1122/ws?key=...`).

**Operation Instructions**:
- **Exit Program**: Use the menu bar `Menu` -> `Quit` (Ctrl+Q) or the tray menu `Quit` to completely exit the program.
- **Run in Background**: Clicking the main window close button (X) will not exit the program but minimize it to the system tray. An icon appears in the system tray area, which can be used to quickly recall the window or exit.
- **Tray Menu**: Right-click the system tray icon to select `Show Main Window` or `Quit`.
- **Open Settings**: Use the menu bar `Menu` -> `Settings` (Ctrl+I) to open the settings window.
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

#### 3.2.3 Get Printer Capabilities (Client -> Server)
Request capabilities for a specific printer:
```json
{
  "type": "get_printer_caps",
  "printer": "Microsoft Print to PDF"
}
```
Example response:
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
> Note: fields vary by platform. Windows returns Win32_Printer/Win32_PrinterConfiguration info; Linux/macOS returns parsed lpoptions data.

#### 3.2.4 Send Print Job (Client -> Server)
The JSON payload is grouped by feature:

```json
{
  "printer": "Microsoft Print to PDF",  // [Required] Target printer name
  "content": "data:application/pdf;base64,JVBERi...", // [Required] Base64 encoded PDF content (Supports Data URI prefix or raw Base64)
  "key": "123456",                      // [Optional] Auth key (can be omitted if verified during connection)
  "job": {
    "name": "My Print Job 001",        // [Optional] Job name (for logging only)
    "copies": 2,                         // [Optional] Number of copies, default 1
    "intervalMs": 1000                   // [Optional] Delay between copies (ms)
  },
  "pages": {
    "range": "1-3,5",                  // [Optional] Page range (N / N-M / N,M / reverse ranges)
    "set": "odd"                       // [Optional] odd | even
  },
  "layout": {
    "scale": "fit",                    // [Optional] noscale | shrink | fit
    "orientation": "portrait"          // [Optional] portrait | landscape
  },
  "color": {
    "mode": "color"                    // [Optional] color | monochrome
  },
  "sides": {
    "mode": "duplex"                   // [Optional] simplex | duplex | duplexshort | duplexlong
  },
  "paper": {
    "size": "A4"                       // [Optional] A4 | letter | legal | tabloid | statement | A2 | A3 | A5 | A6
  },
  "tray": {
    "bin": "2"                         // [Optional] Tray number or name, e.g. 2 / Manual
  },
  "sumatra": {
    "settings": "fit,duplex"           // [Optional] SumatraPDF native -print-settings string (Windows)
  }
}
```

> **Note**: 
> 1. The `content` field must be a **Base64 encoded string of a PDF file**.
>    - Supports standard Data URI format: `data:application/pdf;base64,JVBERi...`
>    - Also supports raw Base64 string: `JVBERi...`
>    - The server automatically strips the `data:` prefix (if present) and validates that the decoded content starts with `%PDF`.

| Field | Type | Description |
| :--- | :--- | :--- |
| `printer` | String | Target printer name. |
| `content` | String | **Base64 encoded PDF content**. |
| `job.name` | String | Print job name. |
| `job.copies` | Integer | **Number of copies**. If `intervalMs` is 0, handled by the system command. |
| `job.intervalMs` | Integer | **Interval (ms)**. When > 0, the server prints one copy per run. |
| `pages.range` | String | **Page range**: `N` / `N-M` / `N,M` / reverse ranges. |
| `pages.set` | String | **Odd/Even**: `odd` / `even`. |
| `layout.scale` | String | **Scaling**: `noscale` / `shrink` / `fit`. |
| `layout.orientation` | String | **Orientation**: `portrait` / `landscape`. |
| `color.mode` | String | **Color mode**: `color` / `monochrome`. |
| `sides.mode` | String | **Duplex**: `simplex` / `duplex` / `duplexshort` / `duplexlong`. |
| `paper.size` | String | **Paper size**: e.g. `A4`, `letter`, `legal`. If omitted, we try to auto-detect common sizes from PDF MediaBox. |
| `tray.bin` | String | **Tray**: number or name. |
| `sumatra.settings` | String | **SumatraPDF native settings** (Windows), passed to `-print-settings`. |

#### 3.2.3.1 Platform Support
**Windows (SumatraPDF)**
- `pages.range` / `pages.set` / `layout.scale` / `layout.orientation` / `color.mode` / `sides.mode` / `paper.size` / `tray.bin` / `job.copies` are converted to `-print-settings`.
- `sumatra.settings` overrides auto-generated settings.

**Linux/macOS (lp/CUPS)**
- `pages.range` -> `-P`.
- `pages.set` -> `-o page-set=odd|even`.
- `layout.scale` -> `fit-to-page` / `scaling=100` (`shrink` uses default behavior).
- `layout.orientation` -> `-o orientation-requested=3|4` (may be ignored by drivers).
- `color.mode` -> `-o ColorModel=Gray` / `-o ColorModel=RGB` (may be ignored by drivers).
- `sides.mode` -> `-o sides=...`.
- `paper.size` -> `-o media=...`.
- `tray.bin` -> `-o InputSlot=...` (depends on driver support).
- `sumatra.settings` is not applicable on Linux/macOS.

#### 3.2.4 Server Response (Server -> Client)
The server returns the result of each print:

**Success Response:**
```json
{
  "status": "success",
  "message": "Printed successfully"
}
```

**Failure Response:**
```json
{
  "status": "error",
  "message": "Content must be a PDF file"
}
```

### 3.3 Client Code Example

```javascript
const socket = new WebSocket('ws://localhost:1122/ws?key=123456');

socket.onopen = () => {
    console.log('Connected');
};

socket.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'printer_list') {
        console.log('Available printers:', msg.data);
        
        // Example: Actively refresh printer list
        // socket.send(JSON.stringify({ type: 'get_printers' }));

        // Send print job (grouped by feature)
        socket.send(JSON.stringify({
          printer: msg.data[0],
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
            size: "A4"
          },
          tray: {
            bin: "2"
          }
        }));
    } else {
        console.log('Response:', msg);
    }
};
```
