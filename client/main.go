package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type raspi struct {
	Hostname  string `json:"hostname"`
	CpuTemp   int8   `json:"cpuTemp"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	hostname := getHostname()
	for {
		cpuTemp := readCPUTemp()
		body := buildBody(hostname, cpuTemp)
		sendRequest(body)
		time.Sleep(5 * time.Second)
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func readFile(path string) string {
	dat, err := ioutil.ReadFile(path)
	check(err)
	return string(dat)
}

func getHostname() string {
	hostname := strings.TrimSuffix(readFile("/etc/hostname"), "\n")
	log.Println(hostname)
	return hostname
}

func readCPUTemp() int {
	rawTemp := strings.TrimSuffix(readFile("/sys/class/thermal/thermal_zone0/temp"), "\n")
	temp, err := strconv.Atoi(rawTemp)
	check(err)
	return temp / 1000
}

func buildBody(hostname string, cpuTemp int) []byte {
	var bodyMap = raspi{hostname, int8(cpuTemp), time.Now().UnixNano() / int64(time.Millisecond)}
	encodedBody, err := json.Marshal(&bodyMap)
	check(err)
	return encodedBody
}

func sendRequest(body []byte) {
	var serverURL = os.Getenv("SERVER_URL")
	resp, err := http.Post(serverURL+"/data", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
}
