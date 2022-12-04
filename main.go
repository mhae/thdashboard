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
var noCurrentHumidity = flag.Bool("noch", false, "Don't fetch current humidity")

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Stats struct {
	LastN                  uint
	CurrentHumidity        string
	LastHumidity           string
	LastTemperature        string
	LastTimestamp          string
	FirstTimestamp         string
	LastNTemperatureTrend  util.Trendline
	LastNHumidityTrend     util.Trendline
	dailyTemperatureMinMax []util.MinMax
	dailyHumidityMinMax    []util.MinMax
	LastDay                *LastN
	Last7Days              *LastN
	Last30Days             *LastN
}

func NewStats() *Stats {
	stats := &Stats{}
	stats.LastN = 31 // max of how far to go back

	stats.LastDay = NewLastN(time.Duration(24)*time.Hour, 1)
	stats.Last7Days = NewLastN(time.Duration(7*24)*time.Hour, 1)
	stats.Last30Days = NewLastN(time.Duration(30*24)*time.Hour, 3600)

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

func formatTimes(timestamps []time.Time) string {
	var b bytes.Buffer
	for i, d := range timestamps {
		if i != 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("'%s'", d.Format("2006-01-02 15:04:05")))
	}
	return b.String()
}

func (stats *Stats) GetLastTimestamp() string {
	return strings.Replace(stats.LastTimestamp, " ", "\n", 0)
}

func (stats *Stats) GetLast7dTimestamps() template.JS {
	return template.JS(formatTimes(stats.Last7Days.Timestamps))
}

func (stats *Stats) GetLast7dTemperatures() template.JS {
	return template.JS(strings.Join(stats.Last7Days.Temperatures, ","))
}

func (stats *Stats) GetLast7dHumidities() template.JS {
	return template.JS(strings.Join(stats.Last7Days.Humidities, ","))
}

func (stats *Stats) GetLast30dTimestamps() template.JS {
	return template.JS(formatTimes(stats.Last30Days.Timestamps))
}

func (stats *Stats) GetLast30dTemperatures() template.JS {
	return template.JS(strings.Join(stats.Last30Days.Temperatures, ","))
}

func (stats *Stats) GetLast30dHumidities() template.JS {
	return template.JS(strings.Join(stats.Last30Days.Humidities, ","))
}

func (stats *Stats) GetLastDayTimestamps() template.JS {
	return template.JS(formatTimes(stats.LastDay.Timestamps))
}

func (stats *Stats) GetLastDayTemperatures() template.JS {
	return template.JS(strings.Join(stats.LastDay.Temperatures, ","))
}

func (stats *Stats) GetLastDayHumidities() template.JS {
	return template.JS(strings.Join(stats.LastDay.Humidities, ","))
}

func (stats *Stats) GenerateNulls() template.JS {
	var sb strings.Builder
	for i := 0; i < len(stats.Last7Days.Timestamps)-2; i++ {
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString("null")

	}
	return template.JS(sb.String())
}

func (stats *Stats) getDailyMinMaxs(mimmax []util.MinMax, min bool) template.JS {
	var sb strings.Builder
	p := 0
	for _, ts := range stats.Last7Days.Timestamps {
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
	return stats.getDailyMinMaxs(stats.dailyTemperatureMinMax, true)
}

func (stats *Stats) GetDailyTemperatureMaxs() template.JS {
	return stats.getDailyMinMaxs(stats.dailyTemperatureMinMax, false)
}

func (stats *Stats) GetDailyHumidityMins() template.JS {
	return stats.getDailyMinMaxs(stats.dailyHumidityMinMax, true)
}

func (stats *Stats) GetDailyHumidityMaxs() template.JS {
	return stats.getDailyMinMaxs(stats.dailyHumidityMinMax, false)
}

func (stats *Stats) getDailyMinMaxScatter(mms []util.MinMax,
	filter func(mm util.MinMax) (ts time.Time, v float32)) template.JS {
	var sb strings.Builder
	for _, mm := range mms {
		var ts, v = filter(mm)
		sb.WriteString(fmt.Sprintf("['%s',%.2f],", ts.Format("2006-01-02 15:04:05"), v))
	}

	s := sb.String()
	return template.JS(s[:len(s)-1])
}

func (stats *Stats) GetDailyTemperatureMinsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyTemperatureMinMax, util.Min)
}

func (stats *Stats) GetDailyTemperatureMaxsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyTemperatureMinMax, util.Max)
}

func (stats *Stats) GetDailyHumidityMinsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyHumidityMinMax, util.Min)
}

func (stats *Stats) GetDailyHumidityMaxsScatter() template.JS {
	return stats.getDailyMinMaxScatter(stats.dailyHumidityMinMax, util.Max)
}

func (s *Stats) process(scanner *util.Scanner) {
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
		temp, err := strconv.ParseFloat(data[1], 32)
		check(err)
		hum, err := strconv.ParseFloat(data[2], 32)
		check(err)

		if s.LastTimestamp == "" {
			s.LastTimestamp = data[0]
			s.LastTemperature = data[1]
			s.LastHumidity = data[2]
			currentDay = date.Day()
			s.dailyTemperatureMinMax = append(s.dailyTemperatureMinMax, *util.NewMinMax())
			s.dailyHumidityMinMax = append(s.dailyHumidityMinMax, *util.NewMinMax())
		}

		s.FirstTimestamp = data[0]

		if currentDay != date.Day() {
			lastN--
			if lastN == 0 {
				break // Done
			}
			// fmt.Println(lastN, currentDay, s.DailyTemperatureMinMax[len(s.DailyTemperatureMinMax)-1], s.DailyHumidityMinMax[len(s.DailyHumidityMinMax)-1])
			s.dailyTemperatureMinMax = append(s.dailyTemperatureMinMax, *util.NewMinMax())
			s.dailyHumidityMinMax = append(s.dailyHumidityMinMax, *util.NewMinMax())

			currentDay = date.Day()
		}

		// Process trendline
		s.LastNTemperatureTrend.Add(float64(date.Second()), temp)
		s.LastNHumidityTrend.Add(float64(date.Second()), hum)

		s.LastDay.Add(date, temp, hum)
		s.Last7Days.Add(date, temp, hum)
		s.Last30Days.Add(date, temp, hum)

		// Process daily min/max
		s.dailyTemperatureMinMax[len(s.dailyTemperatureMinMax)-1].Update(float32(temp), date)
		s.dailyHumidityMinMax[len(s.dailyHumidityMinMax)-1].Update(float32(hum), date)
	}

	util.ReverseSlice(s.dailyTemperatureMinMax)
	util.ReverseSlice(s.dailyHumidityMinMax)

	util.ReverseSlice(s.LastDay.Timestamps)
	util.ReverseSlice(s.LastDay.Temperatures)
	util.ReverseSlice(s.LastDay.Humidities)

	util.ReverseSlice(s.Last7Days.Timestamps)
	util.ReverseSlice(s.Last7Days.Temperatures)
	util.ReverseSlice(s.Last7Days.Humidities)

	util.ReverseSlice(s.Last30Days.Timestamps)
	util.ReverseSlice(s.Last30Days.Temperatures)
	util.ReverseSlice(s.Last30Days.Humidities)

	s.LastNHumidityTrend.Calc()
	s.LastNTemperatureTrend.Calc()

	if *noCurrentHumidity {
		s.CurrentHumidity = "N/A"
	} else {
		s.CurrentHumidity = getCurrentHumidity()
	}

}

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
	println(*noCurrentHumidity)

	// f, err := os.Open(*csvFilename)
	// check(err)
	// defer f.Close()
	// fi, err := f.Stat()
	// check(err)
	// scanner := util.NewScanner(f, int(fi.Size()))
	// last := NewLastN(time.Duration(30*24)*time.Hour, 1)
	// // last7 := NewLastN(time.Duration(24)*time.Hour, 1)
	// for {
	// 	// for i := 0; i < 100; i++ {
	// 	line, _, err := scanner.Line()
	// 	if err == io.EOF {
	// 		break
	// 	}

	// 	if line == "" {
	// 		continue
	// 	}

	// 	data := strings.Split(line, ",")
	// 	data[1] = strings.TrimSpace(data[1])
	// 	data[2] = strings.TrimSpace(data[2])
	// 	date, err := time.Parse("2006-01-02 15:04:05", data[0])
	// 	check(err)
	// 	temp, err := strconv.ParseFloat(data[1], 32)
	// 	if err != nil {
	// 		panic("Can't parse tempature")
	// 	}
	// 	hum, err := strconv.ParseFloat(data[2], 32)
	// 	if err != nil {
	// 		panic("Can't parse humidity")
	// 	}
	// 	last.Add(date, temp, hum)
	// }

	http.HandleFunc("/", handler)
	// http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(content))))
	http.Handle("/static/", http.FileServer(http.FS(content)))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
