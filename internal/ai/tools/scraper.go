package tools

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
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

var (
	reTitle   = regexp.MustCompile(`(?i)<title[^>]*>([\s\S]*?)<\/title>`)
	reMeta    = regexp.MustCompile(`(?i)<meta\s+([^>]*?)>`)
	reLink    = regexp.MustCompile(`(?i)<a\s+[^>]*?href=["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>`)
	reScripts = regexp.MustCompile(`(?i)<(script|style|noscript|svg|iframe|head)[^>]*>([\s\S]*?)<\/\1>`)
	reTags    = regexp.MustCompile(`<[^>]+>`)
	reSpacing = regexp.MustCompile(`\n{3,}`)
)

func Scrape(url string) (*ScrapeResult, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(b)
	res := &ScrapeResult{
		URL:      url,
		MetaTags: make(map[string]string),
	}

	// 1. Title
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		res.Title = cleanText(m[1])
	}

	// 2. Meta Tags
	metaMatches := reMeta.FindAllStringSubmatch(html, -1)
	for _, m := range metaMatches {
		if len(m) > 1 {
			attrs := parseAttributes(m[1])
			name := attrs["name"]
			if name == "" {
				name = attrs["property"]
			}
			content := attrs["content"]
			if name != "" && content != "" {
				res.MetaTags[name] = content
			}
		}
	}

	// 3. Links
	linkMatches := reLink.FindAllStringSubmatch(html, -1)
	for _, m := range linkMatches {
		if len(m) > 2 {
			linkURL := m[1]
			linkText := cleanText(m[2])
			if linkText != "" && !strings.HasPrefix(linkURL, "#") && !strings.HasPrefix(linkURL, "javascript:") {
				res.Links = append(res.Links, LinkInfo{
					Text: linkText,
					URL:  linkURL,
				})
			}
		}
	}

	// 4. Text Content (remove scripts, styles, metadata etc)
	txt := reScripts.ReplaceAllString(html, " ")
	
	// Convert blocks/layout tags to newlines to preserve readability
	reBlocks := regexp.MustCompile(`(?i)</?(div|p|h[1-6]|li|tr|article|section|header|footer)[^>]*>`)
	txt = reBlocks.ReplaceAllString(txt, "\n")
	
	// Remove HTML tags
	txt = reTags.ReplaceAllString(txt, "")
	
	// HTML entities decode
	txt = decodeHTMLEntities(txt)
	
	// Clean whitespace
	lines := strings.Split(txt, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	res.TextContent = strings.Join(cleaned, "\n")

	return res, nil
}

func parseAttributes(s string) map[string]string {
	attrs := make(map[string]string)
	reAttr := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*["']([^"']*)["']`)
	matches := reAttr.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
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
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&rsquo;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\"")
	s = strings.ReplaceAll(s, "&rdquo;", "\"")
	return s
}
