package gigachat

import (
	"regexp"
	"testing"
)

func TestNewRqUID_ValidUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	first, err := newRqUID()
	if err != nil {
		t.Fatalf("newRqUID: %v", err)
	}
	if !re.MatchString(first) {
		t.Errorf("RqUID = %q, не соответствует формату UUID v4", first)
	}

	second, err := newRqUID()
	if err != nil {
		t.Fatalf("newRqUID: %v", err)
	}
	if first == second {
		t.Error("два вызова newRqUID вернули одинаковые значения")
	}
}
