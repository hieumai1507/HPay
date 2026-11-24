package auth

import (
	"runtime"
	"sync"
	"testing"
)

// P2: concurrent argon2 derivations are bounded by the CPU count, and every
// caller still gets a correct answer.
func TestHashConcurrencyIsBounded(t *testing.T) {
	hash, err := HashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	hashPeak.Store(0)

	n := runtime.NumCPU()*4 + 4
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- VerifyPassword(hash, "123456")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("verify under load: %v", err)
		}
	}
	if peak := int(hashPeak.Load()); peak > runtime.NumCPU() {
		t.Fatalf("peak concurrent hashes %d exceeds CPU count %d", peak, runtime.NumCPU())
	}
	if peak := hashPeak.Load(); peak < 1 {
		t.Fatal("no hash was tracked")
	}
}
