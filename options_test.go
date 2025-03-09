package dlocker

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOptionsComplete(t *testing.T) {
	opts := Options{}
	opts = opts.complete()

	_, err := uuid.Parse(opts.Key)
	if err != nil {
		t.Error("Options.Key init error:", err)
	}

	if opts.ValueGeneratorFunc == nil {
		t.Error("Options.ValueGeneratorFunc is nil")
	}

	if opts.RetryInterval != 50*time.Millisecond {
		t.Error("Options.RetryInterval init error, current RetryInterval:", opts.RetryInterval)
	}

	if opts.TTL != 3*time.Second {
		t.Error("Optinos.TTL init error, current TTL:", opts.TTL)
	}
}
