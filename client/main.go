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
	CpuUsage  float64  `json:"cpuUsage"`
	RAMStats  ramStats `json:"ramStats"`
	Timestamp int64    `json:"timestamp"`
}

type ramStats struct {
	Total     uint32 `json:"total"`
	Available uint32 `json:"available"`
	Used      uint32 `json:"used"`
}

var cpuUsage float64

func main() {
	hostname := getHostname()
	for {
		cpuTemp := readCPUTemp()
		go calculateCPUUsage()
		ramStats := readRAMStats()
		body := buildBody(hostname, cpuTemp, cpuUsage, ramStats)
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
	file, err := os.Open(path)
	check(err)
	content := make([]byte, 1024)
	numBytes, err := file.Read(content)
	check(err)
	file.Close()
	return string(content[:numBytes])
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

func calculateCPUUsage() {
	for {
		idle0, total0 := readCPUStats()
		time.Sleep(3 * time.Second)
		idle1, total1 := readCPUStats()
		idleTicks := float64(idle1 - idle0)
		totalTicks := float64(total1 - total0)
		cpuUsage = 100 * (totalTicks - idleTicks) / totalTicks
	}
}

func readCPUStats() (idle, total uint64) {
	rawCPUStats := readFile("/proc/stat")
	lines := strings.Split(rawCPUStats, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					log.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

func buildBody(hostname string, cpuTemp int, cpuUsage float64, ramStats ramStats) []byte {
	var bodyMap = stats{hostname, int8(cpuTemp), cpuUsage, ramStats, time.Now().UnixNano() / int64(time.Millisecond)}
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
