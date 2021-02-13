package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var data map[string]*raspi = make(map[string]*raspi)
var gotifyURL = os.Getenv("GOTIFY_URL")
var gotifyToken = os.Getenv("GOTIFY_TOKEN")

type raspi struct {
	Hostname  string   `json:"hostname"`
	CpuTemp   int8     `json:"cpuTemp"`
	CpuUsage  float64  `json:"cpuUsage"`
	RAMStats  ramStats `json:"ramStats"`
	Timestamp int64    `json:"timestamp"`
	Up        bool     `json:"up"`
}

type ramStats struct {
	Total     uint32 `json:"total"`
	Available uint32 `json:"available"`
	Used      uint32 `json:"used"`
}

type gotifyMessage struct {
	Message  string `json:"message"`
	Priority int8   `json:"priority"`
	Title    string `json:"title"`
}

func main() {
	go checkLastTimestamp()
	http.HandleFunc("/data", handleData)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("Listening to connections")
	log.Fatalln(http.ListenAndServe(":80", nil))
}

func checkLastTimestamp() {
	for {
		now := time.Now().UTC().UnixNano() / int64(time.Millisecond)
		for key, element := range data {
			if now-element.Timestamp > 60000 && element.Up {
				element.Up = false
				title := "Raspberry Pi is down"
				message := fmt.Sprintf("Raspberry Pi %s is down!!", key)
				sendGotifyNotification(title, 8, message)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func sendGotifyNotification(title string, priority int8, message string) {
	gotifyMessage := gotifyMessage{message, priority, title}
	encodedBody, err := json.Marshal(&gotifyMessage)
	if err != nil {
		panic(err)
	}
	_, err = http.Post(gotifyURL+"/message?token="+gotifyToken, "application/json", bytes.NewBuffer(encodedBody))
	if err != nil {
		panic(err)
	}
}

func handleData(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, `%v`, string(jsonData))
	case "POST":
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		var r raspi
		if err := json.Unmarshal(b, &r); err != nil {
			panic(err)
		}

		lastRaspiUpdate := data[r.Hostname]
		if lastRaspiUpdate != nil && !lastRaspiUpdate.Up && r.Up {
			title := "Raspberry Pi is back online"
			message := fmt.Sprintf("Raspberry Pi %s is running again!!", r.Hostname)
			sendGotifyNotification(title, 8, message)
		}
		data[r.Hostname] = &r
	default:
		fmt.Fprintf(w, "Only GET and POST methods are supported.")
	}
}
