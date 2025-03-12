package lockerd

import (
	"testing"
	"time"
)

func TestLockerOptionsComplete(t *testing.T) {
	key := "lock_2000"
	opts := LockerOptions{
		Key: key,
	}
	opts, err := opts.complete()
	if err != nil {
		t.Error("LockerOptions complete error:", err)
	}

	switch {
	case opts.Key != key:
		t.Error("Key error")
	case opts.ValueGeneratorFunc == nil:
		t.Error("ValueGeneratorFunc error")
	case opts.RetryInterval != 50*time.Millisecond:
		t.Error("RetryInterval error")
	case opts.TTL != 5*time.Second:
		t.Error("TTL error")
	case opts.AutoRenewalInterval != opts.TTL/2:
		t.Error("AutoRenewalInterval error")
	case opts.AutoRenewalTTL != opts.TTL:
		t.Error("AutoRenewalTTL error")
	}
}
