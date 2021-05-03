package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var gotifyURL = os.Getenv("GOTIFY_URL")
var gotifyToken = os.Getenv("GOTIFY_TOKEN")

var dbFilePath = os.Getenv("DB_FILE_PATH")

var db *sql.DB

type postDeviceDTO struct {
	Hostname string   `json:"hostname"`
	CpuTemp  int8     `json:"cpuTemp"`
	CpuUsage float64  `json:"cpuUsage"`
	RAMStats ramStats `json:"ramStats"`
}

type deviceRow struct {
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
	db = createDB()
	defer db.Close()
	go checkLastTimestamp()
	http.HandleFunc("/devices", handleDevices)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("Listening to connections")
	log.Fatalln(http.ListenAndServe(":80", nil))
}

func checkLastTimestamp() {
	for {
		now := time.Now().UTC().UnixNano()
		devices := getAllDevices()
		for _, device := range devices {
			if now-device.Timestamp > int64(time.Minute) && device.Up {
				updateDeviceUp(device.Hostname, false)
				title := "Device is down"
				message := fmt.Sprintf("Device %s is down!!", device.Hostname)
				sendGotifyNotification(title, 8, message)
				// If device has been down for more than a week, remove it
			} else if now-device.Timestamp > int64(time.Hour)*24*7 && !device.Up {
				removeDevice(device.Hostname)
				title := "Device removed from database"
				message := fmt.Sprintf("Stale Device %s has been removed from database", device.Hostname)
				sendGotifyNotification(title, 3, message)
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

func createDB() *sql.DB {
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		log.Fatal(err)
	}
	sqlStmt := `
	create table devices (hostname text not null primary key, cpu_temp integer, cpu_usage float, ram_total integer, ram_available integer, ram_used integer, timestamp integer, up integer);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%s", err)
		return db
	}
	return db
}

func getAllDevices() []deviceRow {
	rows, err := db.Query("select hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up from devices")
	if err != nil {
		log.Fatal(err)
	}
	var devices []deviceRow
	for rows.Next() {
		device := deviceRow{}
		err = rows.Scan(&device.Hostname, &device.CpuTemp, &device.CpuUsage, &device.RAMStats.Total, &device.RAMStats.Available, &device.RAMStats.Used, &device.Timestamp, &device.Up)
		if err != nil {
			log.Fatal(err)
		}
		devices = append(devices, device)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	rows.Close()
	return devices
}

func getDeviceByHostname(hostname string) *deviceRow {
	stmt, err := db.Prepare("select hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up from devices where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	var device deviceRow
	err = stmt.QueryRow(hostname).Scan(&device.Hostname, &device.CpuTemp, &device.CpuUsage, &device.RAMStats.Total, &device.RAMStats.Available, &device.RAMStats.Used, &device.Timestamp, &device.Up)
	defer stmt.Close()
	if err != nil {
		if err == sql.ErrNoRows {
			// there were no rows, but otherwise no error occurred
			return nil
		} else {
			log.Fatal(err)
		}
	}
	return &device
}

func updateOrInsertDevice(device deviceRow) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("update devices set cpu_temp=?, cpu_usage=?, ram_total=?, ram_available=?, ram_used=?, timestamp=?, up=? where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	updateResult, err := stmt.Exec(device.CpuTemp, device.CpuUsage, device.RAMStats.Total, device.RAMStats.Available, device.RAMStats.Used, device.Timestamp, device.Up, device.Hostname)
	if err != nil {
		log.Fatal(err)
	}
	stmt.Close()
	rows, err := updateResult.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	if rows == 0 {
		stmt, err := tx.Prepare("insert into devices(hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up) values(?,?,?,?,?,?,?,?)")
		if err != nil {
			log.Fatal(err)
		}
		_, err = stmt.Exec(device.Hostname, device.CpuTemp, device.CpuUsage, device.RAMStats.Total, device.RAMStats.Available, device.RAMStats.Used, device.Timestamp, device.Up)
		if err != nil {
			log.Fatal(err)
		}
		stmt.Close()
	}
	// End transaction
	tx.Commit()
}

func removeDevice(hostname string) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("delete from devices where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	deleteResult, err := stmt.Exec(hostname)
	if err != nil {
		log.Fatal(err)
	}
	_, err = deleteResult.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	// End transaction
	tx.Commit()
}

func updateDeviceUp(hostname string, up bool) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("update devices set up=? where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec(up, hostname)
	if err != nil {
		log.Fatal(err)
	}
	stmt.Close()
	// End transaction
	tx.Commit()
}

func handleDevices(w http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		devices := getAllDevices()
		jsonData, _ := json.Marshal(devices)
		fmt.Fprintf(w, `%v`, string(jsonData))
	case "POST":
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic(err)
		}
		var deviceDTO postDeviceDTO
		if err := json.Unmarshal(body, &deviceDTO); err != nil {
			panic(err)
		}

		lastDeviceUpdate := getDeviceByHostname(deviceDTO.Hostname)
		if lastDeviceUpdate != nil && !lastDeviceUpdate.Up {
			title := "Device is back online"
			message := fmt.Sprintf("Device %s is running again!!", deviceDTO.Hostname)
			sendGotifyNotification(title, 8, message)
		}
		deviceRow := convertFromRequestToRow(deviceDTO)
		updateOrInsertDevice(deviceRow)
	default:
		fmt.Fprintf(w, "Only GET and POST methods are supported.")
	}
}

func convertFromRequestToRow(deviceDTO postDeviceDTO) deviceRow {
	return deviceRow{
		deviceDTO.Hostname,
		deviceDTO.CpuTemp,
		deviceDTO.CpuUsage,
		deviceDTO.RAMStats,
		time.Now().UTC().UnixNano(),
		true,
	}
}
