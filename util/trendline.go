package util

import "math"

type Trendline struct {
	Slope     float64
	Intercept float64
	Num       uint
	XS, XE    float64
	YS, YE    float64
	sumX      float64
	sumX2     float64
	sumY      float64
	sumXY     float64
}

func (tl *Trendline) Add(x, y float64) {
	if tl.Num == 0 {
		tl.XS = x
	}
	tl.XE = x
	tl.Num++
	tl.sumX += x
	tl.sumX2 += x * x
	tl.sumY += y
	tl.sumXY += x * y
}

func (tl *Trendline) Calc() {
	tl.Slope = (tl.sumXY - ((tl.sumX * tl.sumY) / float64(tl.Num))) / (tl.sumX2 - ((tl.sumX * tl.sumX) / float64(tl.Num)))
	tl.Intercept = (tl.sumY / float64(tl.Num)) - (tl.Slope * (tl.sumX / float64(tl.Num)))
	tl.YS = tl.Slope*tl.XS + tl.Intercept
	tl.YS = math.Round(tl.YS*100) / 100
	tl.YE = tl.Slope*tl.XE + tl.Intercept
	tl.YE = math.Round(tl.YE*100) / 100
}
