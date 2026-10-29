package importer

import (
	"errors"
	"strconv"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// feed подает n отзывов с ключами "0".."n-1"
func feed(t *testing.T, size int, seed uint64, n int) *Sampler {
	t.Helper()
	s, err := NewSampler(size, seed)
	if err != nil {
		t.Fatalf("NewSampler: %v", err)
	}
	for i := 0; i < n; i++ {
		s.Add(storage.NewReview{SourceKey: strconv.Itoa(i)})
	}
	return s
}

func keys(items []storage.NewReview) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.SourceKey
	}
	return result
}

func TestSampler_SmallStreamKeepsEverything(t *testing.T) {
	s := feed(t, 10, 1, 4)
	if len(s.Items()) != 4 || s.Seen() != 4 {
		t.Errorf("в выборке %d отзывов из %d предложенных, ожидалось 4 из 4", len(s.Items()), s.Seen())
	}
}

func TestSampler_LimitsSizeAndKeepsUnique(t *testing.T) {
	s := feed(t, 10, 1, 1000)
	if len(s.Items()) != 10 || s.Seen() != 1000 {
		t.Fatalf("в выборке %d отзывов из %d предложенных, ожидалось 10 из 1000", len(s.Items()), s.Seen())
	}

	unique := make(map[string]bool)
	for _, key := range keys(s.Items()) {
		unique[key] = true
	}
	if len(unique) != 10 {
		t.Errorf("в выборке повторы: уникальных ключей %d из 10", len(unique))
	}
}

func TestSampler_DeterministicForSameSeed(t *testing.T) {
	first := keys(feed(t, 10, 7, 1000).Items())
	second := keys(feed(t, 10, 7, 1000).Items())

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("выборки с одним seed различаются: %v и %v", first, second)
		}
	}
}

func TestSampler_SeedChangesSample(t *testing.T) {
	first := keys(feed(t, 10, 1, 1000).Items())
	second := keys(feed(t, 10, 2, 1000).Items())

	same := true
	for i := range first {
		if first[i] != second[i] {
			same = false
		}
	}
	if same {
		t.Errorf("выборки с разными seed совпали: %v", first)
	}
}

func TestNewSampler_InvalidSize(t *testing.T) {
	for _, size := range []int{0, -1} {
		if _, err := NewSampler(size, 1); !errors.Is(err, ErrInvalidSampleSize) {
			t.Errorf("размер %d: ожидалась ErrInvalidSampleSize, получено: %v", size, err)
		}
	}
}
