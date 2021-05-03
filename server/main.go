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
	db = createDB()
	defer db.Close()
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
		data := readAllData()
		for _, element := range data {
			if now-element.Timestamp > 60000 && element.Up {
				updateUp(element.Hostname, false)
				title := "Raspberry Pi is down"
				message := fmt.Sprintf("Raspberry Pi %s is down!!", element.Hostname)
				sendGotifyNotification(title, 8, message)
				// If raspi has been down for more than a week, remove it
			} else if now-element.Timestamp > 60000*60*24*7 && !element.Up {
				removeData(element.Hostname)
				title := "Raspberry Pi removed from database"
				message := fmt.Sprintf("Stale Raspberry Pi %s has been removed from database", element.Hostname)
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
	create table data (hostname text not null primary key, cpu_temp integer, cpu_usage float, ram_total integer, ram_available integer, ram_used integer, timestamp integer, up integer);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return db
	}
	return db
}

func readAllData() []raspi {
	rows, err := db.Query("select hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up from data")
	if err != nil {
		log.Fatal(err)
	}
	var data []raspi
	for rows.Next() {
		raspiData := raspi{}
		err = rows.Scan(&raspiData.Hostname, &raspiData.CpuTemp, &raspiData.CpuUsage, &raspiData.RAMStats.Total, &raspiData.RAMStats.Available, &raspiData.RAMStats.Used, &raspiData.Timestamp, &raspiData.Up)
		if err != nil {
			log.Fatal(err)
		}
		data = append(data, raspiData)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	rows.Close()
	return data
}

func readRaspiData(hostname string) raspi {
	stmt, err := db.Prepare("select hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up from data where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	var data raspi
	err = stmt.QueryRow(hostname).Scan(&data.Hostname, &data.CpuTemp, &data.CpuUsage, &data.RAMStats.Total, &data.RAMStats.Available, &data.RAMStats.Used, &data.Timestamp, &data.Up)
	if err != nil {
		if err == sql.ErrNoRows {
			// there were no rows, but otherwise no error occurred
		} else {
			log.Fatal(err)
		}
	}
	stmt.Close()
	return data
}

func writeData(data raspi) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("update data set cpu_temp=?, cpu_usage=?, ram_total=?, ram_available=?, ram_used=?, timestamp=?, up=? where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	updateResult, err := stmt.Exec(data.CpuTemp, data.CpuUsage, data.RAMStats.Total, data.RAMStats.Available, data.RAMStats.Used, data.Timestamp, data.Up, data.Hostname)
	if err != nil {
		log.Fatal(err)
	}
	stmt.Close()
	rows, err := updateResult.RowsAffected()
	if rows == 0 {
		stmt, err := tx.Prepare("insert into data(hostname, cpu_temp, cpu_usage, ram_total, ram_available, ram_used, timestamp, up) values(?,?,?,?,?,?,?,?)")
		if err != nil {
			log.Fatal(err)
		}
		_, err = stmt.Exec(data.Hostname, data.CpuTemp, data.CpuUsage, data.RAMStats.Total, data.RAMStats.Available, data.RAMStats.Used, data.Timestamp, data.Up)
		if err != nil {
			log.Fatal(err)
		}
		stmt.Close()
	}
	// End transaction
	tx.Commit()
}

func removeData(hostname string) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("delete from data where hostname=?")
	if err != nil {
		log.Fatal(err)
	}
	deleteResult, err := stmt.Exec(hostname)
	if err != nil {
		log.Fatal(err)
	}
	_, err = deleteResult.RowsAffected()
	// End transaction
	tx.Commit()
}

func updateUp(hostname string, up bool) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("update data set up=? where hostname=?")
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

func handleData(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		data := readAllData()
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

		lastRaspiUpdate := readRaspiData(r.Hostname)
		if !lastRaspiUpdate.Up && r.Up {
			title := "Raspberry Pi is back online"
			message := fmt.Sprintf("Raspberry Pi %s is running again!!", r.Hostname)
			sendGotifyNotification(title, 8, message)
		}
		writeData(r)
	default:
		fmt.Fprintf(w, "Only GET and POST methods are supported.")
	}
}
