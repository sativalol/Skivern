package spoofer

import (
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"skyvern/internal/config"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
}

var (
	socks4Proxies []string
	socks5Proxies []string
	proxyPool     []string
	proxiesLoaded bool
	proxyMu       sync.Mutex
)

func init() {
	if p := os.Getenv("SKYVERN_PROXY_POOL"); p != "" {
		for _, x := range strings.Split(p, ",") {
			x = strings.TrimSpace(x)
			if x != "" {
				proxyPool = append(proxyPool, x)
			}
		}
	}
}

func GetRandomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func LoadProxies() {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if proxiesLoaded {
		return
	}
	proxiesLoaded = true

	s5Path := config.ResolvePath("internal/assets/proxies/socks5.txt")
	if b, err := os.ReadFile(s5Path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				socks5Proxies = append(socks5Proxies, "socks5://"+line)
			}
		}
	}

	s4Path := config.ResolvePath("internal/assets/proxies/socks4.txt")
	if b, err := os.ReadFile(s4Path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				socks4Proxies = append(socks4Proxies, "socks4://"+line)
			}
		}
	}
}

func GetRandomProxy() string {
	if len(proxyPool) > 0 {
		return proxyPool[rand.Intn(len(proxyPool))]
	}
	LoadProxies()
	var all []string
	all = append(all, socks5Proxies...)
	all = append(all, socks4Proxies...)
	if len(all) == 0 {
		return ""
	}
	return all[rand.Intn(len(all))]
}

func SetupTransport(trans *http.Transport, explicitProxy string) {
	pStr := explicitProxy
	if pStr == "" {
		pStr = GetRandomProxy()
	}
	if pStr != "" {
		if pURL, err := url.Parse(pStr); err == nil {
			trans.Proxy = http.ProxyURL(pURL)
		}
	}
}

func SetHeaders(req *http.Request, explicitUA string) {
	ua := explicitUA
	if ua == "" {
		ua = GetRandomUA()
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
}
