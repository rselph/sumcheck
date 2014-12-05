package main

import "math"
import "fmt"

func Eng(n float64) string {
	if n == 0.0 {
		return "    0.00e+00"
	}

	negative := n < 0.0
	if negative {
		n = -n
	}

	power := int(math.Floor(math.Log10(n)/3.0) * 3.0)
	factor := math.Pow10(power)

	var sign string
	if negative {
		sign = "-"
	}
	num := fmt.Sprintf("%3.2fe%+03d", n/factor, power)

	final := fmt.Sprintf("%12s", sign+num)

	return final
}
