package main

import (
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed ui/index.html
var indexHTML []byte

//go:embed ui/vendor
var vendorFS embed.FS

func main() {
	runtime.LockOSThread()
	enableHighDPI()

	page, err := serveUI()
	if err != nil {
		alert("Lumina 视界", "本地界面启动失败："+err.Error())
		return
	}

	dataDir := filepath.Join(os.Getenv("AppData"), "Lumina", "webview")
	_ = os.MkdirAll(dataDir, 0o755)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:    false,
		DataPath: dataDir,
		WindowOptions: webview2.WindowOptions{
			Title:  "Lumina 视界",
			Width:  1440,
			Height: 900,
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		alert("Lumina 视界", "没有找到 Microsoft Edge WebView2 运行时。请先安装它，然后再打开本程序。")
		return
	}

	w.SetSize(1440, 900, webview2.HintNone)
	w.SetSize(1100, 740, webview2.HintMin)
	hwnd := windows.HWND(uintptr(w.Window()))
	styleWindow(hwnd)
	applyIcon(hwnd)

	_ = w.Bind("savePhoto", savePhoto)
	_ = w.Bind("openAlbumFolder", openAlbumFolder)
	w.Navigate(page)
	w.Run()
	w.Destroy()
}

func serveUI() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy", "camera=(self)")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("/vendor/", serveVendor)

	ln, err := net.Listen("tcp", "127.0.0.1:18765")
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
	}
	go func() { _ = http.Serve(ln, mux) }()
	return "http://" + ln.Addr().String() + "/", nil
}

func serveVendor(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/vendor/")
	if name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(vendorFS, "ui/vendor/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".mjs"), strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".wasm"):
		w.Header().Set("Content-Type", "application/wasm")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}

func savePhoto(dataURL string) (string, error) {
	comma := strings.IndexByte(dataURL, ',')
	if comma < 0 || comma > 80 {
		return "", errors.New("图片数据无效")
	}
	raw, err := base64.StdEncoding.DecodeString(dataURL[comma+1:])
	if err != nil || len(raw) == 0 || len(raw) > 40<<20 {
		return "", errors.New("图片数据无效")
	}
	dir, err := albumDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	now := time.Now()
	name := fmt.Sprintf("Lumina_%s_%03d.jpg", now.Format("20060102_150405"), now.Nanosecond()/1e6)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func openAlbumFolder() error {
	dir, err := albumDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return exec.Command("explorer", dir).Start()
}

func albumDir() (string, error) {
	pic, err := windows.KnownFolderPath(windows.FOLDERID_Pictures, 0)
	if err != nil || pic == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", errors.New("找不到图片目录")
		}
		pic = filepath.Join(home, "Pictures")
	}
	return filepath.Join(pic, "Lumina"), nil
}

func enableHighDPI() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("SetProcessDpiAwarenessContext")
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if r, _, _ := proc.Call(^uintptr(3)); r == 0 {
		_, _, _ = user32.NewProc("SetProcessDPIAware").Call()
	}
}

func styleWindow(hwnd windows.HWND) {
	var dark uint32 = 1
	_ = windows.DwmSetWindowAttribute(hwnd, 20, unsafe.Pointer(&dark), 4)
	caption := uint32(0x000B0D0C)
	_ = windows.DwmSetWindowAttribute(hwnd, 35, unsafe.Pointer(&caption), 4)
	text := uint32(0x00E6F0F4)
	_ = windows.DwmSetWindowAttribute(hwnd, 36, unsafe.Pointer(&text), 4)
	border := uint32(0x000B0D0C)
	_ = windows.DwmSetWindowAttribute(hwnd, 34, unsafe.Pointer(&border), 4)

	gdi := windows.NewLazySystemDLL("gdi32.dll")
	brush, _, _ := gdi.NewProc("CreateSolidBrush").Call(uintptr(0x000B0D0C))
	if brush != 0 {
		user32 := windows.NewLazySystemDLL("user32.dll")
		// GCLP_HBRBACKGROUND = -10
		_, _, _ = user32.NewProc("SetClassLongPtrW").Call(uintptr(hwnd), ^uintptr(9), brush)
	}
}

func applyIcon(hwnd windows.HWND) {
	var hinst windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &hinst); err != nil {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	// IMAGE_ICON = 1, LR_DEFAULTSIZE | LR_SHARED = 0x8040
	h, _, _ := user32.NewProc("LoadImageW").Call(uintptr(hinst), 1, 1, 0, 0, 0x8040)
	if h == 0 {
		return
	}
	const wmSetIcon = 0x0080
	send := user32.NewProc("SendMessageW")
	_, _, _ = send.Call(uintptr(hwnd), wmSetIcon, 1, h)
	_, _, _ = send.Call(uintptr(hwnd), wmSetIcon, 0, h)
}

func alert(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONWARNING)
}
