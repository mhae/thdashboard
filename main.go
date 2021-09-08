package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"mhae.thdashboard/util"
)

//go:embed static/echarts.min.js
var content embed.FS

//go:embed static/dashboard.html
var dash_html string

var csvFilename = flag.String("file", "", "Path to csv file with timestamp, temperature and humidity")

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func ReverseSlice(s interface{}) {
	size := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, size-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

type Stats struct {
	CurrentHumidity    string
	LastHumidity       string
	LastTemperature    string
	LastTimestamp      string
	LastNTimestamp     []string
	LastNTemperature   []string
	LastNHumidity      []string
	LastDayTimestamp   []string
	LastDayTemperature []string
	LastDayHumidity    []string
}

func NewStats() *Stats {
	stats := &Stats{}
	stats.LastNTimestamp = make([]string, 0)
	stats.LastNTemperature = make([]string, 0)
	stats.LastNHumidity = make([]string, 0)
	return stats
}

func formatTimestamps(timestamps []string) string {
	var b bytes.Buffer
	for i, d := range timestamps {
		if i != 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("'%s'", d))
	}
	return b.String()
}

func (stats *Stats) GetLastNTimestamp() template.JS {
	return template.JS(formatTimestamps(stats.LastNTimestamp))
}

func (stats *Stats) GetLastNTemperature() template.JS {
	return template.JS(strings.Join(stats.LastNTemperature, ","))
}

func (stats *Stats) GetLastNHumidity() template.JS {
	return template.JS(strings.Join(stats.LastNHumidity, ","))
}

func (stats *Stats) GetLastDayTimestamp() template.JS {
	return template.JS(formatTimestamps(stats.LastDayTimestamp))
}

func (stats *Stats) GetLastDayTemperature() template.JS {
	return template.JS(strings.Join(stats.LastDayTemperature, ","))
}

func (stats *Stats) GetLastDayHumidity() template.JS {
	return template.JS(strings.Join(stats.LastDayHumidity, ","))
}

func (stats *Stats) GetLastTimestamp() string {
	return strings.Replace(stats.LastTimestamp, " ", "\n", 0)
}

func (s *Stats) process(scanner *util.Scanner) {
	lastN := 7
	var lastDayStart time.Time
	currentDay := 0

	for {
		line, _, err := scanner.Line()
		if err != nil {
			fmt.Println("Error:", err)
			break
		}
		if line == "" {
			continue
		}

		data := strings.Split(line, ",")
		data[1] = strings.TrimSpace(data[1])
		data[2] = strings.TrimSpace(data[2])
		date, err := time.Parse("2006-01-02 15:04:05", data[0])
		if err != nil {
			fmt.Println("Error:", err)
			break
		}

		if s.LastTimestamp == "" {
			s.LastTimestamp = data[0]
			s.LastTemperature = data[1]
			s.LastHumidity = data[2]
			currentDay = date.Day()

			lastDayStart = date.Add(-time.Hour * 24)
		}

		if currentDay != date.Day() {
			lastN--
			if lastN == 0 {
				break
			}
			currentDay = date.Day()
		}

		s.LastNTimestamp = append(s.LastNTimestamp, data[0])
		s.LastNTemperature = append(s.LastNTemperature, data[1])
		s.LastNHumidity = append(s.LastNHumidity, data[2])

		if date.Unix() >= lastDayStart.Unix() {
			s.LastDayTimestamp = append(s.LastDayTimestamp, data[0])
			s.LastDayTemperature = append(s.LastDayTemperature, data[1])
			s.LastDayHumidity = append(s.LastDayHumidity, data[2])
		}
	}

	ReverseSlice(s.LastNTimestamp)
	ReverseSlice(s.LastNTemperature)
	ReverseSlice(s.LastNHumidity)

	ReverseSlice(s.LastDayTimestamp)
	ReverseSlice(s.LastDayTemperature)
	ReverseSlice(s.LastDayHumidity)

	s.CurrentHumidity = getCurrentHumidity()
}

func getCurrentHumidity() string {
	resp, err := http.Get("https://forecast7.com/en/40d48n104d90/windsor/")
	check(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	pos := bytes.Index(body, []byte("Windsor, CO"))
	if pos == -1 {
		println("Current weather info download failed - no 'Winsdor, CO' in response")
		return "N/A"
	}
	pos = bytes.Index(body, []byte("humidity: "))
	if pos == -1 {
		println("Current weather info download failed - no 'humidity: ' in response")
		return "N/A"
	}
	pos += 10
	end := bytes.Index(body[pos:], []byte("<"))
	return string(body[pos : pos+end])
}

func handler(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(*csvFilename)
	check(err)
	fi, err := f.Stat()

	scanner := util.NewScanner(f, int(fi.Size()))
	stats := NewStats()
	stats.process(scanner)

	t := template.Must(template.New("view.html").Parse(dash_html))
	t.Execute(w, stats)
}

// https://forecast7.com/en/40d48n104d90/windsor/

func main() {
	flag.Parse()
	if csvFilename == nil || *csvFilename == "" {
		panic("File to csv file needs to be specified")
	}
	println(*csvFilename)

	http.HandleFunc("/", handler)
	// http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(content))))
	http.Handle("/static/", http.FileServer(http.FS(content)))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
