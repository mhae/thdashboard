package main

import (
	"fmt"
	"time"
)

type LastN struct {
	Timestamps     []time.Time
	Temperatures   []string
	Humidities     []string
	n              time.Duration
	granularity    int64
	end            time.Time
	sumTemperature float64
	sumHumidity    float64
	sumCount       uint
	sumBucket      int64
}

// Last n buckets of specified granularity in seconds
func NewLastN(n time.Duration, granularity int64) *LastN {
	return &LastN{n: n, granularity: granularity, Timestamps: make([]time.Time, 0), Temperatures: make([]string, 0), Humidities: make([]string, 0)}
}

func (ln *LastN) Add(timestamp time.Time, temperature float64, humidity float64) {
	if ln.end.IsZero() {
		ln.end = timestamp
		ln.end = ln.end.Add(-ln.n)
		// println(timestamp.String(), ln.end.String())
	}
	if !ln.end.Before(timestamp) {
		return
	}

	// fmt.Printf("%s %.2f %.2f\n", timestamp.String(), temperature, humidity)

	bucket := timestamp.Unix() / ln.granularity
	if ln.sumBucket != bucket {
		if ln.sumCount != 0 {
			bucketTs := time.Unix(ln.sumBucket*ln.granularity, 0).UTC()
			ln.Timestamps = append(ln.Timestamps, bucketTs)
			ln.Temperatures = append(ln.Temperatures, fmt.Sprintf("%.2f", ln.sumTemperature/float64(ln.sumCount)))
			ln.Humidities = append(ln.Humidities, fmt.Sprintf("%.2f", ln.sumHumidity/float64(ln.sumCount)))
			// println("bucket ", bucketTs.String(), ln.Temperatures[len(ln.Temperatures)-1], ln.Humidities[len(ln.Humidities)-1])
		}
		ln.sumBucket = bucket
		ln.sumCount = 0
		ln.sumTemperature = 0
		ln.sumHumidity = 0
	}
	ln.sumTemperature += temperature
	ln.sumHumidity += humidity
	ln.sumCount++
}
