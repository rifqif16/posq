package application

import "errors"

var ErrModifierGroupInUse = errors.New("grup modifier masih dipakai produk")

type InvalidModifierGroupError struct{ Index int }

func (e *InvalidModifierGroupError) Error() string { return "grup modifier tidak valid" }
