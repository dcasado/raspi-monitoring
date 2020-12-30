package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type stats struct {
	Hostname  string   `json:"hostname"`
	CpuTemp   int8     `json:"cpuTemp"`
	RAMStats  ramStats `json:"ramStats"`
	Timestamp int64    `json:"timestamp"`
}

type ramStats struct {
	Total     uint32 `json:"total"`
	Available uint32 `json:"available"`
	Used      uint32 `json:"used"`
}

func main() {
	hostname := getHostname()
	for {
		cpuTemp := readCPUTemp()
		ramStats := readRAMStats()
		body := buildBody(hostname, cpuTemp, ramStats)
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

func readRAMStats() ramStats {
	rawRamStats := readFile("/proc/meminfo")
	lines := strings.Split(rawRamStats, "\n")
	re := regexp.MustCompile(`[0-9]+`)
	totalMem64, err := strconv.ParseUint(re.FindString(lines[0]), 10, 32)
	check(err)
	totalMem := uint32(totalMem64)
	availableMem64, err := strconv.ParseUint(re.FindString(lines[2]), 10, 32)
	check(err)
	availableMem := uint32(availableMem64)
	return ramStats{totalMem, availableMem, totalMem - availableMem}
}

func buildBody(hostname string, cpuTemp int, ramStats ramStats) []byte {
	var bodyMap = stats{hostname, int8(cpuTemp), ramStats, time.Now().UnixNano() / int64(time.Millisecond)}
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
