package tools

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"skyvern/internal/spoofer"
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

var (
	reTitle   = regexp.MustCompile(`(?i)<title[^>]*>([\s\S]*?)<\/title>`)
	reMeta    = regexp.MustCompile(`(?i)<meta\s+([^>]*?)>`)
	reLink    = regexp.MustCompile(`(?i)<a\s+[^>]*?href=["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>`)
	reTags    = regexp.MustCompile(`<[^>]+>`)
	reSpacing = regexp.MustCompile(`\n{3,}`)

	reCleanList = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script[^>]*>([\s\S]*?)<\/script>`),
		regexp.MustCompile(`(?i)<style[^>]*>([\s\S]*?)<\/style>`),
		regexp.MustCompile(`(?i)<noscript[^>]*>([\s\S]*?)<\/noscript>`),
		regexp.MustCompile(`(?i)<svg[^>]*>([\s\S]*?)<\/svg>`),
		regexp.MustCompile(`(?i)<iframe[^>]*>([\s\S]*?)<\/iframe>`),
		regexp.MustCompile(`(?i)<head[^>]*>([\s\S]*?)<\/head>`),
	}
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

	spoofer.SetHeaders(req, opts.UserAgent)

	t := opts.Timeout
	if t <= 0 {
		t = 20 * time.Second
	}

	trans := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	spoofer.SetupTransport(trans, opts.ProxyURL)

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

	txt := html
	for _, re := range reCleanList {
		txt = re.ReplaceAllString(txt, " ")
	}
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
