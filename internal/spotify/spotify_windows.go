//go:build windows
package spotify

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procEnumWindows    = user32.NewProc("EnumWindows")
	procGetClassNameW  = user32.NewProc("GetClassNameW")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")

	mu       sync.Mutex
	lastCall time.Time
	cachedTr string
	updating bool
)

func GetSpotifyTrack() string {
	tr, active := getEnumWindowsTrack()
	if tr != "" {
		return tr
	}
	if !active {
		return ""
	}

	mu.Lock()
	now := time.Now()
	if !updating && now.Sub(lastCall) > 3*time.Second {
		updating = true
		lastCall = now
		go func() {
			val := getSMTC()
			mu.Lock()
			cachedTr = val
			updating = false
			mu.Unlock()
		}()
	}
	res := cachedTr
	mu.Unlock()
	return res
}

var (
	enumMu       sync.Mutex
	enumTrack    string
	enumActive   bool
	enumCallback uintptr
)

func init() {
	enumCallback = syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var cName [256]uint16
		_, _, _ = procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&cName[0])), 256)
		cls := syscall.UTF16ToString(cName[:])

		isSp := cls == "SpotifyMainWindow"
		isBr := cls == "Chrome_WidgetWin_1" || cls == "MozillaWindowClass" || cls == "ApplicationFrameWindow"
		if isSp || isBr {
			enumActive = true
		}

		var wTitle [512]uint16
		_, _, _ = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&wTitle[0])), 512)
		t := syscall.UTF16ToString(wTitle[:])
		if t == "" {
			return 1
		}

		if isSp {
			if t != "Spotify" && t != "Spotify Free" && t != "Spotify Premium" {
				enumTrack = t
				return 0
			}
		}

		if !isBr {
			return 1
		}

		if idx := strings.Index(t, " - Spotify"); idx != -1 {
			info := strings.TrimSpace(t[:idx])
			if info != "" && info != "Spotify" && !strings.Contains(strings.ToLower(info), "web player") {
				p := strings.Split(info, " - ")
				if len(p) == 2 {
					enumTrack = p[1] + " - " + p[0]
				} else {
					enumTrack = info
				}
				return 0
			}
		}

		// spotify web player uses a stupid bullet character
		if strings.Contains(t, " \u2022 ") {
			info := t
			if last := strings.LastIndex(t, " - "); last != -1 {
				info = strings.TrimSpace(t[:last])
			}
			p := strings.SplitN(info, " \u2022 ", 2)
			if len(p) == 2 {
				enumTrack = strings.TrimSpace(p[1]) + " - " + strings.TrimSpace(p[0])
				return 0
			}
		}

		return 1
	})
}

func getEnumWindowsTrack() (string, bool) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumTrack = ""
	enumActive = false
	_, _, _ = procEnumWindows.Call(enumCallback, 0)
	return enumTrack, enumActive
}

func getSMTC() string {
	script := `try {
    Add-Type -AssemblyName System.Runtime.WindowsRuntime
    [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager,Windows.Media.Control,ContentType=WindowsRuntime] | Out-Null
    [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionMediaProperties,Windows.Media.Control,ContentType=WindowsRuntime] | Out-Null
    
    $asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { 
        $_.Name -eq 'AsTask' -and 
        $_.GetParameters().Count -eq 1 -and 
        $_.GetParameters()[0].ParameterType.Name -like 'IAsyncOperation*' 
    })[0]

    function Await($WinRtTask, $ResultType) {
        $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
        $netTask = $asTask.Invoke($null, @($WinRtTask))
        $netTask.Wait(-1) | Out-Null
        return $netTask.Result
    }

    $SessionManager = Await ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync()) ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager])
    $Sessions = $SessionManager.GetSessions()
    $sp = $null
    $br = $null
    foreach ($s in $Sessions) {
        $app = $s.SourceAppUserModelId.ToLower()
        if ($app -like '*spotify*') {
            $sp = $s
            break
        }
        if ($app -like '*chrome*' -or $app -like '*brave*' -or $app -like '*edge*' -or $app -like '*firefox*') {
            if (-not $br) {
                $br = $s
            }
        }
    }
    $target = $sp
    if (-not $target) {
        $target = $br
    }
    if ($target) {
        $Props = Await ($target.TryGetMediaPropertiesAsync()) ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionMediaProperties])
        if ($Props.Title) {
            Write-Output "$($Props.Artist) - $($Props.Title)"
        }
    }
} catch {
    # silent
}`
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
