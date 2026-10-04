package extract

import "strconv"

func itoa(n int) string             { return strconv.Itoa(n) }
func itoa64(n int64) string         { return strconv.FormatInt(n, 10) }
func formatNumber(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
