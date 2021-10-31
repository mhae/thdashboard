package main

// (C) Copyright 2021 by Michael Haeuptle

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
	"strconv"
	"strings"
	"time"

	"mhae.thdashboard/util"
)

//go:embed static/echarts.min.js
var content embed.FS

//go:embed static/dashboard.html
var dash_html string

var csvFilename = flag.String("file", "", "Path to csv file with timestamp, temperature and humidity")
var fetchCurrentHumidity = flag.Bool("ch", true, "Fetch current humidity from the Internet")
var lastNd = flag.Uint("lastn", 7, "Last n days")

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Stats struct {
	LastN                 uint
	CurrentHumidity       string
	LastHumidity          string
	LastTemperature       string
	LastTimestamp         string
	FirstTimestamp        string
	LastNTimestamp        []string
	LastNTemperature      []string
	LastNHumidity         []string
	LastDayTimestamp      []string
	LastDayTemperature    []string
	LastDayHumidity       []string
	LastNTemperatureTrend util.Trendline
	LastNHumidityTrend    util.Trendline
	dailyTemperatureAgg   []*util.AggValues
	dailyHumidityAgg      []*util.AggValues
}

func NewStats(lastn uint) *Stats {
	stats := &Stats{}
	stats.LastN = lastn
	stats.LastNTimestamp = make([]string, 0)
	stats.LastNTemperature = make([]string, 0)
	stats.LastNHumidity = make([]string, 0)
	stats.dailyTemperatureAgg = make([]*util.AggValues, 0)
	stats.dailyHumidityAgg = make([]*util.AggValues, 0)
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

func (stats *Stats) GenerateNulls() template.JS {
	var sb strings.Builder
	for i := 0; i < len(stats.LastNTimestamp)-2; i++ {
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString("null")

	}
	return template.JS(sb.String())
}

func (stats *Stats) getDailyMinMaxs(mimmax []*util.AggValues, min bool) template.JS {
	var sb strings.Builder
	p := 0
	for _, ts := range stats.LastNTimestamp {
		if p == len(mimmax) {
			sb.WriteString("null,")
			continue
		}
		if min {
			if ts == mimmax[p].MinTs {
				sb.WriteString(fmt.Sprintf("%.2f,", mimmax[p].Min))
				p++
			} else {
				sb.WriteString("null,")
			}
		} else {
			if ts == mimmax[p].MaxTs {
				sb.WriteString(fmt.Sprintf("%.2f,", mimmax[p].Max))
				p++
			} else {
				sb.WriteString("null,")
			}
		}
	}

	s := sb.String()
	return template.JS(s[:len(s)-1])
}

func (stats *Stats) GetDailyTemperatureMins() template.JS {
	return stats.getDailyMinMaxs(stats.dailyTemperatureAgg, true)
}

func (stats *Stats) GetDailyTemperatureMaxs() template.JS {
	return stats.getDailyMinMaxs(stats.dailyTemperatureAgg, false)
}

func (stats *Stats) GetDailyHumidityMins() template.JS {
	return stats.getDailyMinMaxs(stats.dailyHumidityAgg, true)
}

func (stats *Stats) GetDailyHumidityMaxs() template.JS {
	return stats.getDailyMinMaxs(stats.dailyHumidityAgg, false)
}

func (stats *Stats) getDailyMinMaxScatter(mms []*util.AggValues,
	filter func(mm *util.AggValues) (ts string, v float32)) template.JS {
	var sb strings.Builder
	for _, mm := range mms {
		var ts, v = filter(mm)
		sb.WriteString(fmt.Sprintf("['%s',%.2f],", ts, v))
	}

	s := sb.String()
	return template.JS(s[:len(s)-1])
}

func (stats *Stats) GetDailyTemperatureMinsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyTemperatureAgg, util.Min)
}

func (stats *Stats) GetDailyTemperatureMaxsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyTemperatureAgg, util.Max)
}

func (stats *Stats) GetDailyHumidityMinsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyHumidityAgg, util.Min)
}

func (stats *Stats) GetDailyHumidityMaxsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyHumidityAgg, util.Max)
}

func (s *Stats) process(scanner *util.Scanner) {
	var lastDayStart time.Time
	currentDay := 0
	lastN := s.LastN

	for {
		line, _, err := scanner.Line()
		check(err)

		if line == "" {
			continue
		}

		data := strings.Split(line, ",")
		data[1] = strings.TrimSpace(data[1])
		data[2] = strings.TrimSpace(data[2])
		date, err := time.Parse("2006-01-02 15:04:05", data[0])
		check(err)

		if s.LastTimestamp == "" {
			s.LastTimestamp = data[0]
			s.LastTemperature = data[1]
			s.LastHumidity = data[2]
			currentDay = date.Day()
			lastDayStart = date.Add(-time.Hour * 24)
			s.dailyTemperatureAgg = append(s.dailyTemperatureAgg, util.NewAggValues())
			s.dailyHumidityAgg = append(s.dailyHumidityAgg, util.NewAggValues())
		}

		s.FirstTimestamp = data[0]

		if currentDay != date.Day() {
			lastN--
			if lastN == 0 {
				break // Done
			}
			s.dailyTemperatureAgg = append(s.dailyTemperatureAgg, util.NewAggValues())
			s.dailyHumidityAgg = append(s.dailyHumidityAgg, util.NewAggValues())

			currentDay = date.Day()
		}

		// Process lastN
		s.LastNTimestamp = append(s.LastNTimestamp, data[0])
		s.LastNTemperature = append(s.LastNTemperature, data[1])
		s.LastNHumidity = append(s.LastNHumidity, data[2])

		// Process trendline
		temp, err := strconv.ParseFloat(data[1], 32)
		if err == nil {
			s.LastNTemperatureTrend.Add(float64(date.Second()), temp)
		}
		hum, err := strconv.ParseFloat(data[2], 32)
		if err == nil {
			s.LastNHumidityTrend.Add(float64(date.Second()), hum)
		}

		// Process daily min/max
		s.dailyTemperatureAgg[len(s.dailyTemperatureAgg)-1].Update(float32(temp), data[0])
		s.dailyHumidityAgg[len(s.dailyHumidityAgg)-1].Update(float32(hum), data[0])

		// Process last day
		if date.Unix() >= lastDayStart.Unix() {
			s.LastDayTimestamp = append(s.LastDayTimestamp, data[0])
			s.LastDayTemperature = append(s.LastDayTemperature, data[1])
			s.LastDayHumidity = append(s.LastDayHumidity, data[2])
		}
	}

	util.ReverseSlice(s.LastNTimestamp)
	util.ReverseSlice(s.LastNTemperature)
	util.ReverseSlice(s.LastNHumidity)

	util.ReverseSlice(s.LastDayTimestamp)
	util.ReverseSlice(s.LastDayTemperature)
	util.ReverseSlice(s.LastDayHumidity)

	util.ReverseSlice(s.dailyTemperatureAgg)
	util.ReverseSlice(s.dailyHumidityAgg)

	s.LastNHumidityTrend.Calc()
	s.LastNTemperatureTrend.Calc()

	if *fetchCurrentHumidity {
		s.CurrentHumidity = getCurrentHumidity()
	} else {
		s.CurrentHumidity = "N/A"
	}
}

// Fetches current humidity from the Internet
func getCurrentHumidity() string {
	resp, err := http.Get("https://forecast7.com/en/40d48n104d90/windsor/")
	check(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	check(err)
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
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(*csvFilename)
	check(err)
	defer f.Close()

	fi, err := f.Stat()
	check(err)

	scanner := util.NewScanner(f, int(fi.Size()))
	stats := NewStats(*lastNd)
	stats.process(scanner)

	t := template.Must(template.New("view.html").Parse(dash_html))
	t.Execute(w, stats)
}

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
