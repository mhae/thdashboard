# thdashboard
Temperature and Humidity Dashboard for Raspberry Pi SHT30 Sensor data written in Go.

![79755B5C-6CB6-499C-9A31-4769DCF00731](https://user-images.githubusercontent.com/6045616/205520239-915339ab-7095-4017-b430-1917bc4842de.png)

This web application parses a csv file with temperature and humidity information and generates last 24 hour and last 7 day charts.
The format of the csv file is as follows:

```Timestamp,Temperature,Humidity```

Example:

```
2021-08-29 14:59:32, 71.4, 50.5
2021-08-29 15:14:36, 71.6, 50.7
2021-08-29 15:29:36, 71.6, 50.8
```

## Running (development)
go run . -file=/tmp/crawlspace.csv

## Cross compiling for Raspberry Pi
env GOOS=linux GOARCH=arm GOARM=5 go build

