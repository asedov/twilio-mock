package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

type message struct {
	Date                string
	Sid                 string
	To                  string
	From                string
	Body                string
	AccountSid          string
	MessagingServiceSid string
	StatusCallback      string
}

type messages struct {
	sync.RWMutex
	items map[string]*message
}

type client struct {
	conn     *websocket.Conn // The websocket connection.
	clients  *clients
	messages *messages
	send     chan []byte // Buffered channel of outbound messages.
	isOpen   bool
}

type clients struct {
	sync.RWMutex
	items map[*client]bool
}

func (cs *clients) register(c *client) {
	cs.Lock()
	cs.items[c] = true
	cs.Unlock()
}

func (cs *clients) unregister(c *client) {
	cs.Lock()
	_, ok := cs.items[c]
	if ok {
		delete(cs.items, c)
	}
	cs.Unlock()
}

func (cs *clients) broadcastAdd(m *message) {
	jsn, _ := json.Marshal(&struct {
		Action string      `json:"action"`
		Id     *string     `json:"id"`
		Data   interface{} `json:"data"`
	}{
		Action: "add",
		Id:     &m.Sid,
		Data:   m,
	})

	cs.RLock()
	for c := range cs.items {
		if c.isOpen {
			c.send <- jsn
		}
	}
	cs.RUnlock()
}

func (cs *clients) broadcastDel(id string) {
	jsn, _ := json.Marshal(&struct {
		Action string `json:"action"`
		Id     string `json:"id"`
	}{
		Action: "del",
		Id:     id,
	})

	cs.RLock()
	for c := range cs.items {
		if c.isOpen {
			c.send <- jsn
		}
	}
	cs.RUnlock()
}

func (c *client) close() {
	if c.isOpen {
		c.isOpen = false

		close(c.send)

		err := c.conn.Close()
		if err != nil {
			log.Println("closer error", err)
		}
	}

	c.clients.unregister(c)
}

func (c *client) closeHandler(code int, text string) error {
	c.close()
	return nil
}

func (c *client) pongHandler(appData string) error {
	return nil
}

func (c *client) writer() {
	ticker := time.NewTicker(time.Second * 5)

	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return
			}
		case <-ticker.C:
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}

func (c *client) reader() {
	defer c.close()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		data := &struct {
			Action string `json:"action"`
			Id     string `json:"id"`
		}{}
		err = json.Unmarshal(msg, data)
		if err != nil {
			log.Print(err)
			continue
		}

		if data.Action == "remove" {
			c.messages.del(data.Id)
			c.clients.broadcastDel(data.Id)
		}
	}
}

func (ms *messages) add(m *message) {
	ms.Lock()
	ms.items[m.Sid] = m
	ms.Unlock()
}

func (ms *messages) del(id string) {
	ms.Lock()
	_, ok := ms.items[id]
	if ok {
		delete(ms.items, id)
	}
	ms.Unlock()
}

func (m *message) callback(status string) {
	if m.StatusCallback != "" {
		//go func(msg *message) {
		//	time.Sleep(time.Second)
		//	callbackStatus(msg, "queued")
		//}(msg)
	}
}

func ws(w http.ResponseWriter, r *http.Request, u *websocket.Upgrader, m *messages, c *clients) {
	con, err := u.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	cl := &client{
		conn:     con,
		clients:  c,
		messages: m,
		send:     make(chan []byte, 32),
		isOpen:   true,
	}

	con.SetCloseHandler(cl.closeHandler)
	con.SetPongHandler(cl.pongHandler)

	go cl.reader()
	go cl.writer()

	c.register(cl)

	m.RLock()
	jsn, _ := json.Marshal(&struct {
		Action string      `json:"action"`
		Data   interface{} `json:"data"`
	}{
		"sync",
		m.items,
	})
	m.RUnlock()

	cl.send <- jsn
}

func postMessages(w http.ResponseWriter, r *http.Request, m *messages, c *clients) {

	err := r.ParseMultipartForm(20 * 1024 * 1024)
	if err != nil {
		err = r.ParseForm()
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse request: %s", err), http.StatusBadRequest)
			return
		}
	}

	msg := &message{
		Date:                time.Now().Format(time.RFC1123Z),
		Sid:                 fmt.Sprintf("MG-twilio-mock-%d", time.Now().UnixNano()),
		AccountSid:          mux.Vars(r)["account_sid"],
		To:                  r.FormValue("To"),
		From:                r.FormValue("From"),
		Body:                r.FormValue("Body"),
		MessagingServiceSid: r.FormValue("MessagingServiceSid"),
		StatusCallback:      r.FormValue("StatusCallback"),
	}

	m.add(msg)
	c.broadcastAdd(msg)

	type messageResponseMedia struct {
		Media string `json:"media"`
	}

	type messageResponse struct {
		Sid                 string               `json:"sid"`
		DateCreated         string               `json:"date_created"`
		SateUpdated         string               `json:"date_updated"`
		DateSent            *string              `json:"date_sent"`
		AccountSid          string               `json:"account_sid"`
		To                  string               `json:"to"`
		From                string               `json:"from"`
		MessagingServiceSid *string              `json:"messaging_service_sid"`
		Body                string               `json:"body"`
		Status              string               `json:"status"`
		NumSegments         string               `json:"num_segments"`
		NumMedia            string               `json:"num_media"`
		Direction           string               `json:"direction"`
		ApiVersion          string               `json:"api_version"`
		Price               *float64             `json:"price"`
		PriceUnit           string               `json:"price_unit"`
		ErrorCode           *string              `json:"error_code"`
		ErrorMessage        *string              `json:"error_message"`
		Uri                 string               `json:"uri"`
		SubresourceUris     messageResponseMedia `json:"subresource_uris"`
	}

	rsp := messageResponse{
		Sid:         msg.Sid,
		DateCreated: msg.Date,
		SateUpdated: msg.Date,
		AccountSid:  msg.AccountSid,
		To:          msg.To,
		From:        msg.From,
		Body:        msg.Body,
		Status:      "accepted",
		NumSegments: "1",
		NumMedia:    "0",
		Direction:   "outbound-api",
		ApiVersion:  "2010-04-01",
		PriceUnit:   "USD",
		Uri:         fmt.Sprintf("/2010-04-01/Accounts/%s/Messages/%s.json", msg.AccountSid, msg.Sid),
		SubresourceUris: messageResponseMedia{
			Media: fmt.Sprintf("/2010-04-01/Accounts/%s/Messages/%s/Media.json", msg.AccountSid, msg.Sid),
		},
	}

	if msg.MessagingServiceSid != "" {
		rsp.Status = "queued"
		rsp.MessagingServiceSid = &msg.MessagingServiceSid
	} else {
		msg.callback("queued")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Twilio-Concurrent-Requests", "1")
	w.Header().Set("Twilio-Request-Id", fmt.Sprintf("RQ-twilio-mock-%d", time.Now().UnixNano()))
	w.Header().Set("Twilio-Request-Duration", "0.001")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(rsp)
}

func main() {
	port := flag.Int("port", 80, "specify port")
	host := flag.String("host", "0.0.0.0", "specify host")
	flag.Parse()

	c := &clients{items: make(map[*client]bool)}
	m := &messages{items: make(map[string]*message)}
	u := &websocket.Upgrader{}
	r := mux.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws(w, r, u, m, c)
	}).Methods("GET")

	r.HandleFunc("/2010-04-01/Accounts/{account_sid}/Messages.json", func(w http.ResponseWriter, r *http.Request) {
		postMessages(w, r, m, c)
	}).Methods("POST")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))

	log.Printf("Running on http://%s:%d/", *host, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), r))
}
