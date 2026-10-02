package pagination

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestKeyCursorRoundTripWithSeparatorsInKey(t *testing.T) {
	id := uuid.New()
	key, gotID, err := Decode(Encode("kopi \x01 susu", id))
	if err != nil || key != "kopi \x01 susu" || gotID != id {
		t.Fatalf("%q %v %v", key, gotID, err)
	}
}

func TestTimeCursorRoundTripKeepsMicroseconds(t *testing.T) {
	id := uuid.New()
	at := time.Date(2026, 10, 1, 8, 15, 22, 123456000, time.FixedZone("WIB", 7*3600))
	gotAt, gotID, err := DecodeTime(EncodeTime(at, id))
	if err != nil || !gotAt.Equal(at) || gotID != id {
		t.Fatalf("%v %v %v", gotAt, gotID, err)
	}
}

func TestCursorRejectsGarbage(t *testing.T) {
	id := uuid.New()
	for _, bad := range []string{"!!!", "YWJj", Encode("a", id)[:5]} {
		if _, _, err := Decode(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	for _, bad := range []string{"!!!", Encode("bukan-waktu", id), Encode("", id)} {
		if _, _, err := DecodeTime(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := [][4]int{{0, 50, 100, 50}, {-5, 50, 100, 50}, {1, 50, 100, 1}, {100, 50, 100, 100}, {101, 50, 100, 100}}
	for _, c := range cases {
		if got := Clamp(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("Clamp(%d)=%d want %d", c[0], got, c[3])
		}
	}
}
