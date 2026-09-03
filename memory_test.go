package scim

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

var testTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestMemoryStoreConformance(t *testing.T) {
	if err := CheckStore(context.Background(), func() Store { return NewMemoryStore() }); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStoreConcurrentTransactions(t *testing.T) {
	store := NewMemoryStore()
	var wait sync.WaitGroup
	for index := 0; index < 20; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			record := conformanceRecord("scope", string(rune('a'+index)), "", string(rune('A'+index)), testTime)
			if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(record) }); err != nil {
				t.Errorf("create %d: %v", index, err)
			}
		}()
	}
	wait.Wait()
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		records, err := transaction.List(Query{Scope: "scope", ResourceType: "User", Limit: 100})
		if err != nil || len(records) != 20 {
			t.Fatalf("List() = (%d, %v)", len(records), err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStoreCancellationAfterCallback(t *testing.T) {
	store := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	err := store.Transact(ctx, func(transaction Transaction) error {
		cancel()
		return transaction.Create(conformanceRecord("scope", "id", "", "member", testTime))
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("transaction cancellation = %v", err)
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		if _, err := transaction.Get("scope", "User", "id"); !errors.Is(err, ErrNotFound) {
			return errors.New("canceled transaction committed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
