package main

import (
	"math"
	"testing"
)

func testVal(t *testing.T, n float64, s string) string {
	n_string := Eng(n)
	if len(n_string) != 12 {
		t.Errorf("|%v| %v\n", n_string, n)
	}
	if s != "" && s != n_string {
		t.Logf("<%v>\n", s)
		t.Errorf("|%v| %v\n", n_string, n)
	}
	if !t.Failed() {
		t.Logf("«%v» %v\n", n_string, n)
	}
	return n_string
}

func TestEng(t *testing.T) {
	testVal(t, 0.0, "    0.00e+00")

	testVal(t, math.MaxInt64, "    9.22e+18")
	testVal(t, -math.MaxInt64, "   -9.22e+18")
	testVal(t, math.MaxInt32, "    2.15e+09")
	testVal(t, -math.MaxInt32, "   -2.15e+09")

	testVal(t, 1.0/math.MaxInt64, "  108.42e-21")
	testVal(t, 1.0/math.MaxInt32, "  465.66e-12")

	testVal(t, -1.0/math.MaxInt64, " -108.42e-21")
	testVal(t, -1.0/math.MaxInt32, " -465.66e-12")

	testVal(t, math.MaxFloat64, " 179.77e+306")
	testVal(t, -math.MaxFloat64, "-179.77e+306")

	testVal(t, math.Sqrt(math.SmallestNonzeroFloat64), "   2.22e-162")
}
