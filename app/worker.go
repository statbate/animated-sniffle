package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"net/url"
	"strings"
	"time"
	"encoding/base64"
)

var uptime = time.Now().Unix()

type AuthResponse struct {
	Status    string `json:"status"`
	LocalData struct {
		DataKey string `json:"dataKey"`
	} `json:"localData"`
	UserData struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Location    string `json:"location"`
		Chathost    string `json:"chathost"`
		IsRu        bool   `json:"isRu"`
	} `json:"userData"`
}

type ServerResponse struct {
	TS   int64               `json:"ts"`
	Type string              `json:"type"`
	Body jsoniter.RawMessage `json:"body"`
}

type DonateResponse struct {
	F struct {
		Username string `json:"username"`
	} `json:"f"`
	A int64 `json:"a"`
}

func mapRooms() {

	data := make(map[string]*Info)

	for {
		select {
		case m := <-rooms.Add:
			data[m.room] = &Info{Server: m.Server, Proxy: m.Proxy, Rid: m.Rid, Start: m.Start, Last: m.Last, Online: m.Online, Income: m.Income, Dons: m.Dons, Tips: m.Tips, ch: m.ch}

		case s := <-rooms.Json:
			j, err := json.Marshal(data)
			if err == nil {
				s = string(j)
			}
			rooms.Json <- s

		case <-rooms.Count:
			rooms.Count <- len(data)

		case key := <-rooms.Del:
			delete(data, key)

		case room := <-rooms.Check:
			if _, ok := data[room]; !ok {
				room = ""
			}
			rooms.Check <- room

		case room := <-rooms.Stop:
			if _, ok := data[room]; ok {
				close(data[room].ch)
			}
		}
	}
}

func announceCount() {
	for {
		time.Sleep(30 * time.Second)
		rooms.Count <- 0
		l := <-rooms.Count
		msg, err := json.Marshal(struct {
			Chanel string `json:"chanel"`
			Count  int    `json:"count"`
		}{
			Chanel: "bongacams",
			Count:  l,
		})
		if err == nil {
			socketServer <- msg
		}
	}
}

func reconnectRoom(workerData Info) {
	time.Sleep(5 * time.Second)
	fmt.Println("reconnect:", workerData.room, workerData.Proxy)
	startRoom(workerData)
}

func xWorker(workerData Info, u url.URL) {
	fmt.Println("Start", workerData.room, "server", workerData.Server, "proxy", workerData.Proxy)

	rooms.Add <- workerData

	defer func() {
		rooms.Del <- workerData.room
	}()

	b64, err := base64.StdEncoding.DecodeString(workerData.Params)
	if err != nil {
		fmt.Println(err, workerData.room)
		return
	}

	v := struct {
		Chathost    string `json:"chathost"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Location    string `json:"location"`
		IsRu        bool   `json:"isRu"`
		DataKey     string `json:"dataKey"`
	}{}
	
	if err := json.Unmarshal(b64, &v); err != nil {
		fmt.Println("exit: no amf parms", workerData.room, err)
		return
	}

	Dialer := *websocket.DefaultDialer

	if _, ok := conf.Proxy[workerData.Proxy]; ok {
		Dialer = websocket.Dialer{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http", // or "https" depending on your proxy
				Host:   conf.Proxy[workerData.Proxy],
				Path:   "/",
			}),
			HandshakeTimeout: 45 * time.Second, // https://pkg.go.dev/github.com/gorilla/websocket
		}
	}

	c, _, err := Dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	defer c.Close()

	c.SetReadDeadline(time.Now().Add(60 * time.Second))

	if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"joinRoom","args":["%s",{"username":"%s","displayName":"%s","location":"%s","chathost":"%s","isRu":%t,"isPerformer":false,"hasStream":false,"isLogged":false,"isPayable":false,"showType":"public"},"%s"]}`, 1, v.Chathost, v.Username, v.DisplayName, v.Location, v.Chathost, v.IsRu, v.DataKey))); err != nil {
		fmt.Println(err.Error())
		return
	}

	_, message, err := c.ReadMessage()
	if err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	slog <- saveLog{workerData.Rid, time.Now().Unix(), string(message)}

	if string(message) == `{"id":1,"result":{"audioAvailable":false,"freeShow":false},"error":null}` {
		fmt.Println("room offline, exit", workerData.room)
		return
	}

	if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"ChatModule.connect","args":["public-chat"]}`, 2))); err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	_, message, err = c.ReadMessage()
	if err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	slog <- saveLog{workerData.Rid, time.Now().Unix(), string(message)}

	quit := make(chan struct{})
	pid := 3

	defer func() {
		close(quit)
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"ping"}`, pid))); err != nil {
					fmt.Println(err.Error(), workerData.room)
					rooms.Stop <- workerData.room
					return
				}
				pid++
				break
			}
		}
	}()

	dons := make(map[string]struct{})
	
	timeout := time.NewTicker(60 * 60 * 8 * time.Second)
	defer timeout.Stop()
	
	var income int64
	income = 0

	for {
		select {
		case <-timeout.C:
			fmt.Println("too_long exit:", workerData.room)
			return
		case <-workerData.ch:
			fmt.Println("Exit room:", workerData.room)
			return
		default:
		}

		c.SetReadDeadline(time.Now().Add(30 * time.Minute))
		_, message, err := c.ReadMessage()
		if err != nil {
			if income > 1 && websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				go reconnectRoom(workerData)
			}
			return
		}

		now := time.Now().Unix()
		slog <- saveLog{workerData.Rid, now, string(message)}
		
		if now > workerData.Last+60*60 {
			fmt.Println("no_tips exit:", workerData.room)
			return
		}

		m := &ServerResponse{}
		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error(), workerData.room)
			continue
		}

		if m.Type == "ServerMessageEvent:PERFORMER_STATUS_CHANGE" && string(m.Body) == `"offline"` {
			fmt.Println(m.Type, workerData.room)
			return
		}

		if m.Type == "ServerMessageEvent:ROOM_CLOSE" {
			fmt.Println(m.Type, workerData.room)
			return
		}

		if m.Type == "ServerMessageEvent:INCOMING_TIP" {
			d := &DonateResponse{}
			if err = json.Unmarshal(m.Body, d); err == nil {

				workerData.Tips++
				if _, ok := dons[d.F.Username]; !ok {
					dons[d.F.Username] = struct{}{}
					workerData.Dons++
				}

				save <- saveData{workerData.room, strings.ToLower(d.F.Username), workerData.Rid, d.A, now}

				income += d.A
				
				workerData.Last = now
				workerData.Income += d.A
				rooms.Add <- workerData
				
				fmt.Println(d.F.Username, "send", d.A, "tokens to", workerData.room)
			}
		}
	}
}
