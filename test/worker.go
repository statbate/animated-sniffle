package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type ServerResponse struct {
	// {"id":"1660218639317-sub-viewServerChanged:hls-07","status":200,"body":""}
	// {"subscriptionKey":"global","params":{"subscriptionKey":"clearChatMessages","params":{"senderIds":[80825678]}}}
	// {"id":"1660218639317-sub-viewServerChanged:hls-07","method":"PUT","url":"/front/clients/9gd4q9r0/subscriptions/viewServerChanged:hls-07"}
	// {"subscriptionKey":"global","params":{"subscriptionKey":"clearChatMessages","params":{"senderIds":[80825678]}}}
	// {"subscriptionKey":"newChatMessage:36500801","params":{"message":{"cacheId":"62f4ed9872be2","modelId":36500801,"senderId":36500801,"type":"lovense","details":{"lovenseDetails":{"type":"tip","status":"","text":"","detail":{"name":"land-marks","amount":15,"time":5,"power":"medium"}},"fanClubTier":null,"fanClubNumberMonthsOfSubscribed":0},"userData":{"id":36500801,"username":"DoviaClaire","gender":"female","hasAdminBadge":false,"isAdmin":false,"isSupport":false,"isModel":true,"isStudio":false,"isGold":false,"isUltimate":false,"isGreen":false,"isRegular":false,"isExGreen":false,"hasVrDevice":false,"userRanking":null,"isKing":false,"isKnight":false,"isStudioModerator":false,"isStudioAdmin":false},"createdAt":"2022-08-11T11:52:56Z","additionalData":[],"id":1740865563732962}}}
	SubscriptionKey string `json:"subscriptionKey,omitempty"`
	Params          struct {
		Message struct {
			Details struct {
				LovenseDetails struct {
					Type   string `json:"type,omitempty"`
					Detail struct {
						Name   string  `json:"name,omitempty"`
						Amount float64 `json:"amount,omitepty"`
					} `json:"detail,omitempty"`
				} `json:"lovenseDetails"`
			} `json:"details,omitempty"`
		} `json:"message,omitempty`
	} `json:"params,omitempty`
}

func startRoom(room, server, proxy string, u *url.URL) {
	// curl -vvv -X POST -H "Content-Type: application/x-www-form-urlencoded; charset=UTF-8" -H "X-Requested-With: XMLHttpRequest" -d "method=getRoomData" -d "args[]=Icehotangel"   "https://rt.bongocams.com/tools/amf.php?res=771840&t=1654437233142"

	fmt.Println("Start", room, "server", server, "proxy", proxy)

	Dialer := *websocket.DefaultDialer

	proxyMap := make(map[string]string)
	proxyMap["us"] = "aaa:port"
	proxyMap["fi"] = "bbb:port"

	if _, ok := proxyMap[proxy]; ok {
		Dialer = websocket.Dialer{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http", // or "https" depending on your proxy
				Host:   proxyMap[proxy],
				Path:   "/",
			}),
			HandshakeTimeout: 45 * time.Second, // https://pkg.go.dev/github.com/gorilla/websocket
		}
	}

	c, _, err := Dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer c.Close()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println("return")
			return
		}

		// fmt.Println(string(message))d

		m := &ServerResponse{}

		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error())
			continue
		}

		if m.Params.Message.Details.LovenseDetails.Type == "tip" {
			fmt.Println(m.Params.Message.Details.LovenseDetails.Detail.Name, " send ", m.Params.Message.Details.LovenseDetails.Detail.Amount, "tokens")
		}
	}
}
