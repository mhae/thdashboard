package util

import "math"

type MinMax struct {
	Min   float32
	MinTs string
	Max   float32
	MaxTs string
}

func NewMinMax() *MinMax {
	return &MinMax{Min: math.MaxFloat32}
}

func Max(mm MinMax) (string, float32) {
	return mm.MaxTs, mm.Max
}

func Min(mm MinMax) (string, float32) {
	return mm.MinTs, mm.Min
}

func (mm *MinMax) Update(v float32, ts string) {
	if v <= mm.Min { // equal check needed to track last occurrence
		mm.Min = v
		mm.MinTs = ts
	} else if v >= mm.Max {
		mm.Max = v
		mm.MaxTs = ts
	}
}
