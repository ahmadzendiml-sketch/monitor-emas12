package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
)

type HistoryItem struct {
	BuyingRate         int    `json:"buying_rate"`
	SellingRate        int    `json:"selling_rate"`
	Status             string `json:"status"`
	Diff               int    `json:"diff"`
	CreatedAt          string `json:"created_at"`
	WaktuDisplay       string `json:"waktu_display"`
	DiffDisplay        string `json:"diff_display"`
	TransactionDisplay string `json:"transaction_display"`
	Jt20               string `json:"jt20"`
	Jt30               string `json:"jt30"`
	Jt40               string `json:"jt40"`
	Jt50               string `json:"jt50"`
}

type UsdIdrItem struct {
	Price string `json:"price"`
	Time  string `json:"time"`
}

type TransferJam struct {
	JamMasuk   string `json:"jam_masuk"`
	Durasi     string `json:"durasi"`
	LastUpdate string `json:"last_update"`
}

type State struct {
	History       []HistoryItem `json:"history"`
	UsdIdrHistory []UsdIdrItem  `json:"usd_idr_history"`
	TreasuryInfo  string        `json:"treasury_info"`
	TransferJam   TransferJam   `json:"transfer_jam"`
}

var (
	state      State
	stateMutex sync.RWMutex
	wsClients  = make(map[*WsConn]bool)
	wsMutex    sync.Mutex
	lastBuy    int
	shownUpd   = make(map[string]bool)
	banned     = make(map[int64]bool)
)

func InitState() {
	state = State{
		TreasuryInfo: "Belum ada info treasury.",
		TransferJam:  TransferJam{},
	}
}

func GetStateBytes() []byte {
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	b, _ := json.Marshal(state)
	return b
}

func ApiStateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(GetStateBytes())
}

type WsConn struct {
	Conn *websocket.Conn
	Send chan []byte
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ws := &WsConn{Conn: conn, Send: make(chan []byte, 8)}
	wsMutex.Lock()
	if len(wsClients) >= 500 {
		wsMutex.Unlock()
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1013, "Too many connections"))
		conn.Close()
		return
	}
	wsClients[ws] = true
	wsMutex.Unlock()
	ws.Send <- GetStateBytes()
	go func() {
		for msg := range ws.Send {
			ws.Conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()
	go func() {
		defer func() {
			ws.Conn.Close()
			wsMutex.Lock()
			delete(wsClients, ws)
			wsMutex.Unlock()
		}()
		for {
			_, msg, err := ws.Conn.ReadMessage()
			if err != nil {
				break
			}
			if string(msg) == "ping" {
				ws.Send <- []byte(`{"pong":true}`)
			}
		}
	}()
}

func BroadcastState(state []byte) {
	wsMutex.Lock()
	for c := range wsClients {
		select {
		case c.Send <- state:
		default:
		}
	}
	wsMutex.Unlock()
}

func StartFetchers() {
	go func() {
		for {
			FetchTreasury()
			time.Sleep(10 * time.Millisecond)
		}
	}()
	go func() {
		for {
			FetchUsdIdr()
			time.Sleep(200 * time.Millisecond)
		}
	}()
	go func() {
		for {
			time.Sleep(15 * time.Second)
			wsMutex.Lock()
			for c := range wsClients {
				select {
				case c.Send <- []byte(`{"ping":true}`):
				default:
				}
			}
			wsMutex.Unlock()
		}
	}()
}

func FetchTreasury() {
	req, _ := http.NewRequest("POST", "https://api.treasury.id/api/v1/antigrvty/gold/rate", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://treasury.id")
	req.Header.Set("Referer", "https://treasury.id/")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return
	}
	var buy, sell int
	switch v := data["buying_rate"].(type) {
	case string:
		buy, _ = strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	case float64:
		buy = int(v)
	}
	switch v := data["selling_rate"].(type) {
	case string:
		sell, _ = strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	case float64:
		sell = int(v)
	}
	upd, _ := data["updated_at"].(string)
	if buy == 0 || sell == 0 || upd == "" {
		return
	}
	stateMutex.Lock()
	if shownUpd[upd] {
		stateMutex.Unlock()
		return
	}
	diff := 0
	status := "➖"
	if lastBuy != 0 {
		diff = buy - lastBuy
		if buy > lastBuy {
			status = "🚀"
		} else if buy < lastBuy {
			status = "🔻"
		}
	}
	lastBuy = buy
	shownUpd[upd] = true
	if len(shownUpd) > 5000 {
		shownUpd = map[string]bool{upd: true}
	}
	buyFmt := formatRupiah(buy)
	sellFmt := formatRupiah(sell)
	diffDisplay := formatDiffDisplay(diff, status)
	h := HistoryItem{
		BuyingRate:         buy,
		SellingRate:        sell,
		Status:             status,
		Diff:               diff,
		CreatedAt:          upd,
		WaktuDisplay:       formatWaktuDisplay(upd, status),
		DiffDisplay:        diffDisplay,
		TransactionDisplay: formatTransactionDisplay(buyFmt, sellFmt, diffDisplay),
		Jt20:               calcProfit(buy, sell, 20000000, 19314000),
		Jt30:               calcProfit(buy, sell, 30000000, 28980000),
		Jt40:               calcProfit(buy, sell, 40000000, 38652000),
		Jt50:               calcProfit(buy, sell, 50000000, 48325000),
	}
	state.History = append(state.History, h)
	if len(state.History) > 1441 {
		state.History = state.History[len(state.History)-1441:]
	}
	stateMutex.Unlock()
	BroadcastState(GetStateBytes())
}

func FetchUsdIdr() {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", "https://www.google.com/finance/quote/USD-IDR", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.AddCookie(&http.Cookie{Name: "CONSENT", Value: "YES+cb.20231208-04-p0.en+FX+410"})
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}
	price := ""
	doc.Find("div.YMlKec.fxKbKc").Each(func(i int, s *goquery.Selection) {
		if price == "" {
			price = strings.TrimSpace(s.Text())
		}
	})
	if price == "" {
		return
	}
	now := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("15:04:05")
	stateMutex.Lock()
	if len(state.UsdIdrHistory) == 0 || state.UsdIdrHistory[len(state.UsdIdrHistory)-1].Price != price {
		state.UsdIdrHistory = append(state.UsdIdrHistory, UsdIdrItem{Price: price, Time: now})
		if len(state.UsdIdrHistory) > 11 {
			state.UsdIdrHistory = state.UsdIdrHistory[len(state.UsdIdrHistory)-11:]
		}
		stateMutex.Unlock()
		BroadcastState(GetStateBytes())
		return
	}
	stateMutex.Unlock()
}

func formatRupiah(n int) string {
	s := strconv.Itoa(n)
	var out []byte
	cnt := 0
	for i := len(s) - 1; i >= 0; i-- {
		out = append([]byte{s[i]}, out...)
		cnt++
		if cnt%3 == 0 && i != 0 {
			out = append([]byte{'.'}, out...)
		}
	}
	return string(out)
}

func calcProfit(buy, sell, modal, pokok int) string {
	gram := float64(modal) / float64(buy)
	val := int(gram*float64(sell)) - pokok
	gramStr := fmt.Sprintf("%.4f", gram)
	if val > 0 {
		return "+" + formatRupiah(val) + "🟢➺" + gramStr + "gr"
	} else if val < 0 {
		return "-" + formatRupiah(-val) + "🔴➺" + gramStr + "gr"
	}
	return formatRupiah(0) + "➖➺" + gramStr + "gr"
}

func formatDiffDisplay(diff int, status string) string {
	if status == "🚀" {
		return "🚀+" + formatRupiah(diff)
	} else if status == "🔻" {
		return "🔻-" + formatRupiah(-diff)
	}
	return "➖tetap"
}

func formatTransactionDisplay(buy, sell, diff string) string {
	return "Harga Beli: " + buy + " Jual: " + sell + " " + diff
}

func formatWaktuDisplay(t, status string) string {
	tm, err := time.Parse("2006-01-02 15:04:05", t)
	if err != nil {
		return t + status
	}
	hari := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}
	wd := tm.Weekday()
	idx := int(wd)
	if idx == 0 {
		idx = 6
	} else {
		idx--
	}
	return fmt.Sprintf("%s %02d:%02d:%02d %s", hari[idx], tm.Hour(), tm.Minute(), tm.Second(), status)
}

func formatDuration(totalMinutes int) string {
	if totalMinutes <= 0 {
		return "0 menit"
	}
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours == 0 {
		return fmt.Sprintf("%d menit", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%d jam", hours)
	}
	return fmt.Sprintf("%d jam %d menit", hours, minutes)
}
