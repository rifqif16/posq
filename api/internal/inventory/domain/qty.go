package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	QtyScale  int64 = 1000
	MaxQtyAbs int64 = 999_999_999 * QtyScale
)

var qtyPattern = regexp.MustCompile(`^([+-]?)(\d{1,9})(?:\.(\d{1,3}))?$`)

var ErrInvalidQty = errors.New("jumlah tidak valid")

func ParseQty(s string) (int64, error) {
	m := qtyPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, ErrInvalidQty
	}
	whole, _ := strconv.ParseInt(m[2], 10, 64)
	frac := int64(0)
	if m[3] != "" {
		padded := m[3] + strings.Repeat("0", 3-len(m[3]))
		frac, _ = strconv.ParseInt(padded, 10, 64)
	}
	n := whole*QtyScale + frac
	if m[1] == "-" {
		n = -n
	}
	return n, nil
}

func FormatQty(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	return fmt.Sprintf("%s%d.%03d", sign, n/QtyScale, n%QtyScale)
}
