package fun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"skyvern/internal/manager"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	bolt "go.etcd.io/bbolt"
)

type Stock struct {
	Symbol       string
	Price        float64
	OpenPrice    float64
	PriceHistory []float64
}

type UserPortfolio struct {
	Cash     float64            `json:"cash"`
	Holdings map[string]float64 `json:"holdings"` // sym -> shares
}

type YFQuoteResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol                     string  `json:"symbol"`
			LongName                   string  `json:"longName"`
			RegularMarketPrice         float64 `json:"regularMarketPrice"`
			RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
			RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
			RegularMarketOpen          float64 `json:"regularMarketOpen"`
		} `json:"result"`
	} `json:"quoteResponse"`
}

type YFChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol        string  `json:"symbol"`
				PreviousClose float64 `json:"previousClose"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

var (
	yfCrumb  string
	yfJar, _ = cookiejar.New(nil)
	yfClient = &http.Client{
		Timeout: 5 * time.Second,
		Jar:     yfJar,
	}
)

func init() {
	manager.RegisterHelp("stock", []manager.HelpPage{
		{
			Command:     "Market Watchlist",
			Syntax:      ".stock [list]",
			Description: "View live prices and changes for major stocks and cryptos.",
		},
		{
			Command:     "View Stock Detail",
			Syntax:      ".stock view <symbol>",
			Description: "Inspect a stock or crypto's current price, open price, holders, and view its 1d chart.",
		},
		{
			Command:     "Buy Shares",
			Syntax:      ".stock buy <symbol> <shares>",
			Description: "Purchase shares using your portfolio cash balance.",
		},
		{
			Command:     "Sell Shares",
			Syntax:      ".stock sell <symbol> <shares>",
			Description: "Sell your shares of a symbol back to the market.",
		},
		{
			Command:     "Portfolio Overview",
			Syntax:      ".stock <portfolio|port|bal>",
			Description: "Show your cash balance, owned shares, their current value, and your net worth.",
		},
	})
}

var StockCmd = &manager.Command{
	Trigger:     "stock",
	Aliases:     []string{"shares", "stocks", "stonks"},
	Name:        "stock",
	Description: "Trade actual stocks and cryptos with real-time charts",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return listStocks(ctx)
		}

		switch strings.ToLower(ctx.Args[0]) {
		case "list":
			return listStocks(ctx)
		case "view":
			return viewStock(ctx)
		case "buy":
			return buyStock(ctx)
		case "sell":
			return sellStock(ctx)
		case "portfolio", "port", "bal":
			return viewPortfolio(ctx)
		default:
			return viewStock(ctx)
		}
	},
}

func initYFinanceSession() error {
	req, err := http.NewRequest("GET", "https://fc.yahoo.com", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := yfClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	req, err = http.NewRequest("GET", "https://query2.finance.yahoo.com/v1/test/getcrumb", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err = yfClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get crumb: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	yfCrumb = string(bytes.TrimSpace(body))
	return nil
}

func fetchYFQuote(symbols []string) (*YFQuoteResponse, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("empty symbols")
	}

	if yfCrumb == "" {
		_ = initYFinanceSession()
	}

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v7/finance/quote?symbols=%s", strings.Join(symbols, ","))
	if yfCrumb != "" {
		url += fmt.Sprintf("&crumb=%s", yfCrumb)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := yfClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		yfCrumb = ""
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	var res YFQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if len(res.QuoteResponse.Result) == 0 {
		return nil, fmt.Errorf("no quote results")
	}
	return &res, nil
}

func getQuoteWithRetry(symbols []string) (*YFQuoteResponse, error) {
	res, err := fetchYFQuote(symbols)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") || yfCrumb == "") {
	
		if err = initYFinanceSession(); err == nil {
			return fetchYFQuote(symbols)
		}
	}
	return res, err
}

func fetchYFChart(symbol string) ([]float64, float64, error) {
	if yfCrumb == "" {
		_ = initYFinanceSession()
	}

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=15m", symbol)
	if yfCrumb != "" {
		url += fmt.Sprintf("&crumb=%s", yfCrumb)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := yfClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		yfCrumb = ""
		return nil, 0, fmt.Errorf("http error %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("http error %d", resp.StatusCode)
	}

	var res YFChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, 0, err
	}
	if len(res.Chart.Result) == 0 {
		return nil, 0, fmt.Errorf("no chart result")
	}

	resObj := res.Chart.Result[0]
	if len(resObj.Indicators.Quote) == 0 {
		return nil, 0, fmt.Errorf("no chart quote data")
	}

	rawClose := resObj.Indicators.Quote[0].Close
	var history []float64
	for _, val := range rawClose {
		if val > 0 && !math.IsNaN(val) {
			history = append(history, val)
		}
	}
	return history, resObj.Meta.PreviousClose, nil
}

func getChartWithRetry(symbol string) ([]float64, float64, error) {
	history, prevClose, err := fetchYFChart(symbol)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") || yfCrumb == "") {
		if err = initYFinanceSession(); err == nil {
			return fetchYFChart(symbol)
		}
	}
	return history, prevClose, err
}

func getPort(tx *bolt.Tx, uid string) *UserPortfolio {
	b := tx.Bucket([]byte("GlobalConfig"))
	v := b.Get([]byte("stocks:portfolio:" + uid))
	var p UserPortfolio
	if v == nil {
		return &UserPortfolio{
			Cash:     10000.00,
			Holdings: make(map[string]float64),
		}
	}
	if err := json.Unmarshal(v, &p); err != nil {
		return &UserPortfolio{
			Cash:     10000.00,
			Holdings: make(map[string]float64),
		}
	}
	if p.Holdings == nil {
		p.Holdings = make(map[string]float64)
	}
	return &p
}

func savePort(tx *bolt.Tx, uid string, p *UserPortfolio) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return tx.Bucket([]byte("GlobalConfig")).Put([]byte("stocks:portfolio:"+uid), b)
}

func countHolders(tx *bolt.Tx, sym string) int {
	b := tx.Bucket([]byte("GlobalConfig"))
	c := b.Cursor()
	prefix := []byte("stocks:portfolio:")
	count := 0
	for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
		var p UserPortfolio
		if json.Unmarshal(v, &p) == nil {
			if shares, ok := p.Holdings[sym]; ok && shares > 0 {
				count++
			}
		}
	}
	return count
}

func getChangeStr(cur, open float64) string {
	diff := cur - open
	pct := (diff / open) * 100.0
	if diff >= 0 {
		return fmt.Sprintf("🟢 ▲ %.2f%%", pct)
	}
	return fmt.Sprintf("🔴 ▼ %.2f%%", math.Abs(pct))
}

func listStocks(ctx *manager.CommandContext) error {
	watchlist := []string{"AAPL", "MSFT", "NVDA", "TSLA", "BTC-USD", "ETH-USD"}
	res, err := getQuoteWithRetry(watchlist)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("[!] Failed to fetch stock data: %v", err))
	}

	desc := ""
	for _, s := range res.QuoteResponse.Result {
		change := s.RegularMarketPrice - s.RegularMarketPreviousClose
		pct := (change / s.RegularMarketPreviousClose) * 100.0
		var chg string
		if change >= 0 {
			chg = fmt.Sprintf("🟢 ▲ %.2f%%", pct)
		} else {
			chg = fmt.Sprintf("🔴 ▼ %.2f%%", math.Abs(pct))
		}
		name := s.LongName
		if name == "" {
			name = s.Symbol
		}
		desc += fmt.Sprintf("**%s** — %s: **$%.2f** (%s)\n", s.Symbol, name, s.RegularMarketPrice, chg)
	}

	emb := &discordgo.MessageEmbed{
		Color:       0x2b2d31,
		Title:       "Live Stock Market",
		Description: desc,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use .stock view <symbol> to see charts | Quotes by YFinance",
		},
	}
	return ctx.Respond(emb)
}

func viewStock(ctx *manager.CommandContext) error {
	sym := ""
	if len(ctx.Args) > 1 {
		sym = strings.ToUpper(ctx.Args[1])
	} else if len(ctx.Args) == 1 {
		sym = strings.ToUpper(ctx.Args[0])
		if sym == "LIST" || sym == "PORTFOLIO" || sym == "PORT" || sym == "BAL" || sym == "BUY" || sym == "SELL" {
			return ctx.SendHelp("stock")
		}
	} else {
		return ctx.SendHelp("stock")
	}

	res, err := getQuoteWithRetry([]string{sym})
	if err != nil || len(res.QuoteResponse.Result) == 0 {
		return ctx.Reply(fmt.Sprintf("[!] Stock %s not found.", sym))
	}
	s := res.QuoteResponse.Result[0]

	history, prevClose, err := getChartWithRetry(sym)
	if err != nil || len(history) == 0 {
		history = []float64{s.RegularMarketPreviousClose, s.RegularMarketPrice}
		prevClose = s.RegularMarketPreviousClose
	}

	change := s.RegularMarketPrice - s.RegularMarketPreviousClose
	pct := (change / s.RegularMarketPreviousClose) * 100.0
	var chgEmoji string
	if change >= 0 {
		chgEmoji = fmt.Sprintf("🟢 ▲ %.2f%%", pct)
	} else {
		chgEmoji = fmt.Sprintf("🔴 ▼ %.2f%%", math.Abs(pct))
	}

	name := s.LongName
	if name == "" {
		name = s.Symbol
	}

	var holders int
	_ = ctx.DB.View(func(tx *bolt.Tx) error {
		holders = countHolders(tx, sym)
		return nil
	})

	emb := &discordgo.MessageEmbed{
		Color: 0x2b2d31,
		Title: fmt.Sprintf("%s — %s", s.Symbol, name),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Price", Value: fmt.Sprintf("$%.2f %s", s.RegularMarketPrice, chgEmoji), Inline: true},
			{Name: "Open", Value: fmt.Sprintf("$%.2f", s.RegularMarketOpen), Inline: true},
			{Name: "Prev Close", Value: fmt.Sprintf("$%.2f", prevClose), Inline: true},
			{Name: "Holders (Guild)", Value: strconv.Itoa(holders), Inline: true},
			{Name: "Recent Events", Value: "No recent events.", Inline: false},
		},
		Image: &discordgo.MessageEmbedImage{
			URL: "attachment://chart.png",
		},
	}

	stockObj := &Stock{
		Symbol:       s.Symbol,
		Price:        s.RegularMarketPrice,
		OpenPrice:    prevClose,
		PriceHistory: history,
	}

	chartBytes := drawChart(stockObj)
	_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{emb},
		Files: []*discordgo.File{
			{
				Name:        "chart.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(chartBytes),
			},
		},
	})
	return err
}

func buyStock(ctx *manager.CommandContext) error {
	if len(ctx.Args) < 3 {
		return ctx.SendHelp("stock")
	}

	sym := strings.ToUpper(ctx.Args[1])
	sh, err := strconv.ParseFloat(ctx.Args[2], 64)
	if err != nil || sh <= 0 || math.IsNaN(sh) || math.IsInf(sh, 0) {
		return ctx.Reply("[!] Enter a valid positive number of shares.")
	}

	res, err := getQuoteWithRetry([]string{sym})
	if err != nil || len(res.QuoteResponse.Result) == 0 {
		return ctx.Reply(fmt.Sprintf("[!] Stock %s not found.", sym))
	}
	s := res.QuoteResponse.Result[0]

	uid := ctx.AuthorID()
	var cash float64
	var owned float64

	err = ctx.DB.Update(func(tx *bolt.Tx) error {
		p := getPort(tx, uid)
		cost := s.RegularMarketPrice * sh
		if p.Cash < cost {
			return fmt.Errorf("insufficient funds (Need $%.2f, balance $%.2f)", cost, p.Cash)
		}

		p.Cash -= cost
		p.Holdings[sym] += sh

		cash = p.Cash
		owned = p.Holdings[sym]

		return savePort(tx, uid, p)
	})

	if err != nil {
		return ctx.Reply(fmt.Sprintf("[!] Buy failed: %v", err))
	}

	return ctx.Reply(fmt.Sprintf("[+] Bought **%.4f** shares of **%s** @ **$%.2f**.\nBalance: **$%.2f** | Owned: **%.4f** shares.", sh, sym, s.RegularMarketPrice, cash, owned))
}

func sellStock(ctx *manager.CommandContext) error {
	if len(ctx.Args) < 3 {
		return ctx.SendHelp("stock")
	}

	sym := strings.ToUpper(ctx.Args[1])
	sh, err := strconv.ParseFloat(ctx.Args[2], 64)
	if err != nil || sh <= 0 || math.IsNaN(sh) || math.IsInf(sh, 0) {
		return ctx.Reply("[!] Enter a valid positive number of shares.")
	}

	res, err := getQuoteWithRetry([]string{sym})
	if err != nil || len(res.QuoteResponse.Result) == 0 {
		return ctx.Reply(fmt.Sprintf("[!] Stock %s not found.", sym))
	}
	s := res.QuoteResponse.Result[0]

	uid := ctx.AuthorID()
	var cash float64
	var owned float64

	err = ctx.DB.Update(func(tx *bolt.Tx) error {
		p := getPort(tx, uid)
		n := p.Holdings[sym]
		if n < sh {
			return fmt.Errorf("insufficient shares (Owned %.4f, selling %.4f)", n, sh)
		}

		p.Holdings[sym] -= sh
		p.Cash += s.RegularMarketPrice * sh

		if p.Holdings[sym] <= 0 {
			delete(p.Holdings, sym)
		}

		cash = p.Cash
		owned = p.Holdings[sym]

		return savePort(tx, uid, p)
	})

	if err != nil {
		return ctx.Reply(fmt.Sprintf("[!] Sell failed: %v", err))
	}

	return ctx.Reply(fmt.Sprintf("[+] Sold **%.4f** shares of **%s** @ **$%.2f**.\nBalance: **$%.2f** | Owned: **%.4f** shares.", sh, sym, s.RegularMarketPrice, cash, owned))
}

func viewPortfolio(ctx *manager.CommandContext) error {
	uid := ctx.AuthorID()
	var p *UserPortfolio

	err := ctx.DB.View(func(tx *bolt.Tx) error {
		p = getPort(tx, uid)
		return nil
	})
	if err != nil {
		return ctx.Reply("[!] Database error.")
	}

	var symbols []string
	for sym, sh := range p.Holdings {
		if sh > 0 {
			symbols = append(symbols, sym)
		}
	}

	prices := make(map[string]float64)
	prevCloses := make(map[string]float64)
	if len(symbols) > 0 {
		res, err := getQuoteWithRetry(symbols)
		if err == nil {
			for _, item := range res.QuoteResponse.Result {
				prices[item.Symbol] = item.RegularMarketPrice
				prevCloses[item.Symbol] = item.RegularMarketPreviousClose
			}
		}
	}

	desc := ""
	total := p.Cash
	for sym, sh := range p.Holdings {
		if sh <= 0 {
			continue
		}
		prc, ok := prices[sym]
		if !ok {
			prc = 0.0
		}
		val := sh * prc
		total += val

		prev := prevCloses[sym]
		chg := ""
		if prev > 0 {
			chg = getChangeStr(prc, prev)
		}

		desc += fmt.Sprintf("**%s** — %.4f shares ($%.2f) %s\n", sym, sh, val, chg)
	}

	if desc == "" {
		desc = "*You do not own any stocks.*"
	}

	emb := &discordgo.MessageEmbed{
		Color:       0x2b2d31,
		Title:       fmt.Sprintf("💼 %s's Portfolio", ctx.AuthorTag()),
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Cash Balance", Value: fmt.Sprintf("$%.2f", p.Cash), Inline: true},
			{Name: "Net Worth", Value: fmt.Sprintf("$%.2f", total), Inline: true},
		},
	}

	return ctx.Respond(emb)
}

func absVal(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, col color.Color) {
	dx := absVal(x1 - x0)
	dy := absVal(y1 - y0)
	sx, sy := 1, 1
	if x0 >= x1 {
		sx = -1
	}
	if y0 >= y1 {
		sy = -1
	}
	err := dx - dy

	for {
		for io := -1; io <= 1; io++ {
			for jo := -1; jo <= 1; jo++ {
				cx, cy := x0+io, y0+jo
				if cx >= 0 && cx < img.Bounds().Dx() && cy >= 0 && cy < img.Bounds().Dy() {
					img.Set(cx, cy, col)
				}
			}
		}

		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func drawChart(s *Stock) []byte {
	w, h := 400, 150
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	bg := color.RGBA{0x1e, 0x1f, 0x22, 0xff}
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)

	history := s.PriceHistory
	if len(history) < 2 {
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		return buf.Bytes()
	}

	minVal, maxVal := history[0], history[0]
	for _, val := range history {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	diff := maxVal - minVal
	if diff == 0 {
		diff = 1.0
	}

	var pts []image.Point
	for i, val := range history {
		x := i * w / (len(history) - 1)
		y := h - 15 - int((val-minVal)/diff*float64(h-30))
		pts = append(pts, image.Pt(x, y))
	}

	var lineCol color.Color
	if s.Price >= s.OpenPrice {
		lineCol = color.RGBA{0x23, 0xa5, 0x5a, 0xff}
	} else {
		lineCol = color.RGBA{0xf2, 0x3f, 0x43, 0xff}
	}

	for i := 0; i < len(pts)-1; i++ {
		drawLine(img, pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y, lineCol)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
