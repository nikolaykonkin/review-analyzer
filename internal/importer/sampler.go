package importer

import (
	"errors"
	"math/rand/v2"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// ErrInvalidSampleSize означает, что размер выборки меньше единицы
var ErrInvalidSampleSize = errors.New("размер выборки должен быть не меньше 1")

// Sampler выбирает из потока отзывов равномерную случайную выборку
// фиксированного размера (reservoir sampling, алгоритм R): память зависит
// только от размера выборки, а не от длины потока
// При одинаковых seed и порядке отзывов выборка одинакова
type Sampler struct {
	size  int
	rng   *rand.Rand
	seen  int
	items []storage.NewReview
}

// NewSampler создает выборщика на size отзывов с заданным seed
func NewSampler(size int, seed uint64) (*Sampler, error) {
	if size < 1 {
		return nil, ErrInvalidSampleSize
	}
	return &Sampler{
		size:  size,
		rng:   rand.New(rand.NewPCG(seed, seed)),
		items: make([]storage.NewReview, 0, size),
	}, nil
}

// Add предлагает очередной отзыв выборке
func (s *Sampler) Add(review storage.NewReview) {
	s.seen++
	if len(s.items) < s.size {
		s.items = append(s.items, review)
		return
	}
	// Отзыв с номером seen (с единицы) заменяет случайный элемент выборки
	// с вероятностью size/seen: так каждый отзыв потока остается в выборке
	// с одинаковой вероятностью
	if j := s.rng.IntN(s.seen); j < s.size {
		s.items[j] = review
	}
}

// Items возвращает текущую выборку
func (s *Sampler) Items() []storage.NewReview {
	return s.items
}

// Seen возвращает число отзывов, предложенных выборке
func (s *Sampler) Seen() int {
	return s.seen
}
