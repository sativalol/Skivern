package tools

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"skyvern/internal/config"
)

type ScrapeResult struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	MetaTags    map[string]string `json:"meta_tags"`
	Links       []LinkInfo        `json:"links"`
	TextContent string            `json:"text_content"`
	RawHTML     string            `json:"raw_html,omitempty"`
}

type LinkInfo struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type ScrapeOpts struct {
	ProxyURL  string
	UserAgent string
	Timeout   time.Duration
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
}

var (
	proxyPool     []string
	socks4Proxies []string
	socks5Proxies []string
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

func loadAssetProxies() {
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

var (
	reTitle   = regexp.MustCompile(`(?i)<title[^>]*>([\s\S]*?)<\/title>`)
	reMeta    = regexp.MustCompile(`(?i)<meta\s+([^>]*?)>`)
	reLink    = regexp.MustCompile(`(?i)<a\s+[^>]*?href=["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>`)
	reScripts = regexp.MustCompile(`(?i)<(script|style|noscript|svg|iframe|head)[^>]*>([\s\S]*?)<\/\1>`)
	reTags    = regexp.MustCompile(`<[^>]+>`)
	reSpacing = regexp.MustCompile(`\n{3,}`)
)

func Scrape(url string) (*ScrapeResult, error) {
	return ScrapeWithOptions(url, ScrapeOpts{})
}

func ScrapeWithOptions(u string, opts ScrapeOpts) (*ScrapeResult, error) {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = userAgents[rand.Intn(len(userAgents))]
	}

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")

	t := opts.Timeout
	if t <= 0 {
		t = 20 * time.Second
	}

	trans := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	pStr := opts.ProxyURL
	if pStr == "" && len(proxyPool) > 0 {
		pStr = proxyPool[rand.Intn(len(proxyPool))]
	}

	if pStr == "" {
		loadAssetProxies()
		var all []string
		all = append(all, socks5Proxies...)
		all = append(all, socks4Proxies...)
		if len(all) > 0 {
			pStr = all[rand.Intn(len(all))]
		}
	}

	if pStr != "" {
		if pURL, err := url.Parse(pStr); err == nil {
			trans.Proxy = http.ProxyURL(pURL)
		}
	}

	cli := &http.Client{
		Timeout:   t,
		Transport: trans,
	}

	res, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http bad status: %d", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	html := string(b)
	out := &ScrapeResult{
		URL:      u,
		MetaTags: make(map[string]string),
	}

	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		out.Title = cleanText(m[1])
	}

	for _, m := range reMeta.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			attrs := parseAttributes(m[1])
			name := attrs["name"]
			if name == "" {
				name = attrs["property"]
			}
			content := attrs["content"]
			if name != "" && content != "" {
				out.MetaTags[name] = content
			}
		}
	}

	for _, m := range reLink.FindAllStringSubmatch(html, -1) {
		if len(m) > 2 {
			linkURL := m[1]
			linkText := cleanText(m[2])
			if linkText != "" && !strings.HasPrefix(linkURL, "#") && !strings.HasPrefix(linkURL, "javascript:") {
				out.Links = append(out.Links, LinkInfo{
					Text: linkText,
					URL:  linkURL,
				})
			}
		}
	}

	txt := reScripts.ReplaceAllString(html, " ")
	reBlocks := regexp.MustCompile(`(?i)</?(div|p|h[1-6]|li|tr|article|section|header|footer)[^>]*>`)
	txt = reBlocks.ReplaceAllString(txt, "\n")
	txt = reTags.ReplaceAllString(txt, "")
	txt = decodeHTMLEntities(txt)

	lines := strings.Split(txt, "\n")
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			cleaned = append(cleaned, l)
		}
	}
	out.TextContent = strings.Join(cleaned, "\n")

	return out, nil
}

func parseAttributes(s string) map[string]string {
	attrs := make(map[string]string)
	reAttr := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*["']([^"']*)["']`)
	for _, m := range reAttr.FindAllStringSubmatch(s, -1) {
		if len(m) > 2 {
			attrs[strings.ToLower(m[1])] = m[2]
		}
	}
	return attrs
}

func cleanText(s string) string {
	s = reTags.ReplaceAllString(s, "")
	s = decodeHTMLEntities(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func decodeHTMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&rsquo;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\"")
	s = strings.ReplaceAll(s, "&rdquo;", "\"")
	return s
}
