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

func TestEng1(t *testing.T) {
	testVal(t, 0.0, "    0.00E+00")

	testVal(t, math.MaxInt64, "    9.22E+18")
	testVal(t, -math.MaxInt64, "   -9.22E+18")
	testVal(t, math.MaxInt32, "    2.15E+09")
	testVal(t, -math.MaxInt32, "   -2.15E+09")

	testVal(t, 1.0/math.MaxInt64, "  108.42E-21")
	testVal(t, 1.0/math.MaxInt32, "  465.66E-12")

	testVal(t, -1.0/math.MaxInt64, " -108.42E-21")
	testVal(t, -1.0/math.MaxInt32, " -465.66E-12")

	testVal(t, math.MaxFloat64, " 179.77E+306")
	testVal(t, -math.MaxFloat64, "-179.77E+306")

	testVal(t, math.Sqrt(math.SmallestNonzeroFloat64), "   2.22E-162")
}
