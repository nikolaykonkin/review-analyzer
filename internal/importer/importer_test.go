package importer

import (
	"context"
	"errors"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/migrate"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
	"github.com/nikolaykonkin/review-analyzer/internal/testutil"
	"github.com/nikolaykonkin/review-analyzer/migrations"
)

// fakeStore - хранилище в памяти
// failOn - номер вызова (с единицы), на котором возвращается ошибка; ноль означает "никогда"
type fakeStore struct {
	keys   map[string]bool
	failOn int
	calls  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{keys: make(map[string]bool)}
}

func (f *fakeStore) InsertReview(_ context.Context, review storage.NewReview) (int64, bool, error) {
	f.calls++
	if f.failOn != 0 && f.calls == f.failOn {
		return 0, false, errors.New("сбой хранилища")
	}
	if f.keys[review.SourceKey] {
		return 0, false, nil
	}
	f.keys[review.SourceKey] = true
	return int64(f.calls), true, nil
}

func reviewsWithKeys(keys ...string) []storage.NewReview {
	reviews := make([]storage.NewReview, len(keys))
	for i, key := range keys {
		reviews[i] = storage.NewReview{SourceKey: key}
	}
	return reviews
}

func TestInsert_CountsInsertedAndDuplicates(t *testing.T) {
	store := newFakeStore()

	result, err := Insert(context.Background(), store, reviewsWithKeys("a", "b", "a", "c", "b"))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if result.Inserted != 3 || result.Duplicates != 2 {
		t.Errorf("result = %+v, ожидалось 3 вставленных и 2 дубликата", result)
	}
}

func TestInsert_StopsOnError(t *testing.T) {
	store := newFakeStore()
	store.failOn = 2

	result, err := Insert(context.Background(), store, reviewsWithKeys("a", "b", "c"))
	if err == nil {
		t.Fatal("ожидалась ошибка хранилища")
	}
	if result.Inserted != 1 {
		t.Errorf("Inserted = %d, ожидалось 1 (до сбоя)", result.Inserted)
	}
	if store.calls != 2 {
		t.Errorf("вызовов хранилища %d, ожидалось 2: после ошибки загрузка должна остановиться", store.calls)
	}
}

func TestInsert_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Insert(ctx, newFakeStore(), reviewsWithKeys("a"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ожидалась context.Canceled, получено: %v", err)
	}
}

// Интеграционный тест: повторная загрузка демонстрационного файла
// в реальную базу не создает дубликатов
func TestInsert_IsIdempotentWithRealStorage(t *testing.T) {
	pool := testutil.NewPool(t)
	ctx := context.Background()

	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatalf("загрузка миграций: %v", err)
	}
	if _, err := migrate.Up(ctx, pool, ms); err != nil {
		t.Fatalf("применение миграций: %v", err)
	}
	repo := storage.New(pool)
	reviews, _ := readSampleFile(t)

	first, err := Insert(ctx, repo, reviews)
	if err != nil {
		t.Fatalf("первая загрузка: %v", err)
	}
	if first.Inserted != len(reviews) || first.Duplicates != 0 {
		t.Errorf("первая загрузка: %+v, ожидалось %d вставленных и 0 дубликатов", first, len(reviews))
	}

	second, err := Insert(ctx, repo, reviews)
	if err != nil {
		t.Fatalf("повторная загрузка: %v", err)
	}
	if second.Inserted != 0 || second.Duplicates != len(reviews) {
		t.Errorf("повторная загрузка: %+v, ожидалось 0 вставленных и %d дубликатов", second, len(reviews))
	}
}
