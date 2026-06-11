package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type tgt struct {
	Name string
	OS   string
	Arch string
	Ext  string
}

func main() {
	tgts := []tgt{
		{"Windows (x64)", "windows", "amd64", ".exe"},
		{"Windows (32-bit)", "windows", "386", ".exe"},
		{"macOS (Apple Silicon)", "darwin", "arm64", ""},
		{"macOS (Intel)", "darwin", "amd64", ""},
		{"Linux (x64)", "linux", "amd64", ""},
		{"Android (Termux / arm64)", "android", "arm64", ""},
	}

	fmt.Println("SKYVERN")
	fmt.Println("Select your target build platform:")
	for i, t := range tgts {
		fmt.Printf("[%d] %s (GOOS=%s GOARCH=%s)\n", i+1, t.Name, t.OS, t.Arch)
	}
	fmt.Printf("[%d] Build for Current Host (%s/%s)\n", len(tgts)+1, runtime.GOOS, runtime.GOARCH)
	fmt.Print("\nEnter choice (q to quit): ")

	rd := bufio.NewReader(os.Stdin)
	in, _ := rd.ReadString('\n')
	in = strings.TrimSpace(in)

	if strings.ToLower(in) == "q" {
		fmt.Println("Cancelled.")
		return
	}

	t := tgt{
		Name: "Current Host",
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
	if runtime.GOOS == "windows" {
		t.Ext = ".exe"
	}

	if in != "" && in != fmt.Sprintf("%d", len(tgts)+1) {
		var v int
		_, err := fmt.Sscanf(in, "%d", &v)
		if err != nil || v < 1 || v > len(tgts) {
			fmt.Println("invalid selection")
			return
		}
		t = tgts[v-1]
	}

	out := "skyvern" + t.Ext
	fmt.Printf("\nBuilding for %s (%s/%s)...\n", t.Name, t.OS, t.Arch)

	c := exec.Command("go", "build", "-ldflags=-s -w", "-o", out, "main.go")
	c.Env = append(os.Environ(),
		"GOOS="+t.OS,
		"GOARCH="+t.Arch,
	)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		fmt.Printf("\n[!] build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[+] build successful: %s\n", out)
}
