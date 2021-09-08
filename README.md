# thdashboard
Temperature and Humidity Dashboard for Raspberry Pi SHT30 Sensor written in Go.

![1BBF1064-3588-4A58-97F2-2942626AEAB4](https://user-images.githubusercontent.com/6045616/132441382-21c437c9-133d-46b6-9abd-f44f7b32a159.png)

This web application parses a csv file with temperature and humidity information and generates last 24 hour and last 7 day charts.
The format of the csv file is as follows:

```Timestamp,Temperature,Humidity```

Example:

```
2021-08-29 14:59:32, 71.4, 50.5
2021-08-29 15:14:36, 71.6, 50.7
2021-08-29 15:29:36, 71.6, 50.8
```

## Running
go run main.go -file=/tmp/crawlspace.csv
