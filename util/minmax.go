package util

import (
	"math"
	"time"
)

type MinMax struct {
	Min   float32
	MinTs time.Time
	Max   float32
	MaxTs time.Time
}

func NewMinMax() *MinMax {
	return &MinMax{Min: math.MaxFloat32}
}

func Max(mm MinMax) (time.Time, float32) {
	return mm.MaxTs, mm.Max
}

func Min(mm MinMax) (time.Time, float32) {
	return mm.MinTs, mm.Min
}

func (mm *MinMax) Update(v float32, ts time.Time) {
	if v <= mm.Min { // equal check needed to track last occurrence
		mm.Min = v
		mm.MinTs = ts
	} else if v >= mm.Max {
		mm.Max = v
		mm.MaxTs = ts
	}
}
