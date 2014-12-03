package main

import "math"
import "fmt"

func Eng_int64(n int64) string {
	return Eng(float64(n))
}

func Eng(n float64) string {
	if n == 0.0 {
		return "   0.00E+00"
	}

	negative := n < 0.0
	if negative {
		n = -n
	}

	power := uint64(math.Floor(math.Log10(n)/3.0) * 3.0)
	factor := math.Exp(float64(power) * math.Log(10))

	var sign string
	if negative {
		sign = "-"
	}
	num := fmt.Sprintf("%3.2fE%+03d", n/factor, power)

	final := fmt.Sprintf("%11s", sign+num)

	return final
}
