package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("cursor tidak valid")

func Encode(key string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key + "\x00" + id.String()))
}

func Decode(cursor string) (string, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	i := strings.LastIndexByte(string(raw), 0)
	if i < 0 {
		return "", uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(string(raw[i+1:]))
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return string(raw[:i]), id, nil
}

func EncodeTime(at time.Time, id uuid.UUID) string {
	return Encode(at.UTC().Format(time.RFC3339Nano), id)
}

func DecodeTime(cursor string) (time.Time, uuid.UUID, error) {
	key, id, err := Decode(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, key)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return at, id, nil
}

func Clamp(limit, def, max int) int {
	switch {
	case limit <= 0:
		return def
	case limit > max:
		return max
	}
	return limit
}
