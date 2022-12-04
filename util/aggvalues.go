package util

import "math"

// Aggregate values (min, max, avg) which tracks timestamps of min and max.
type AggValues struct {
	Min   float32
	MinTs string
	Max   float32
	MaxTs string
	sum   float32
	count uint32
}

func NewAggValues() *AggValues {
	return &AggValues{Min: math.MaxFloat32}
}

// func Max(av *AggValues) (string, float32) {
// 	return av.MaxTs, av.Max
// }

// func Min(av *AggValues) (string, float32) {
// 	return av.MinTs, av.Min
// }

func (av *AggValues) Avg() float32 {
	return av.sum / float32(av.count)
}

// Updates the min, max, avg with value v for timestamp ts
func (av *AggValues) Update(v float32, ts string) {
	if v <= av.Min { // equal check needed to track last occurrence
		av.Min = v
		av.MinTs = ts
	} else if v >= av.Max {
		av.Max = v
		av.MaxTs = ts
	}
	av.sum += v
	av.count++
}
