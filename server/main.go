package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

var data map[string]raspi = make(map[string]raspi)

type raspi struct {
	Hostname  string   `json:"hostname"`
	CpuTemp   int8     `json:"cpuTemp"`
	CpuUsage  float64  `json:"cpuUsage"`
	RAMStats  ramStats `json:"ramStats"`
	Timestamp int64    `json:"timestamp"`
}

type ramStats struct {
	Total     uint32 `json:"total"`
	Available uint32 `json:"available"`
	Used      uint32 `json:"used"`
}

func main() {
	http.HandleFunc("/data", handleData)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("Listening to connections")
	log.Fatalln(http.ListenAndServe(":80", nil))
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

		data[r.Hostname] = r
	default:
		fmt.Fprintf(w, "Only GET and POST methods are supported.")
	}
}
