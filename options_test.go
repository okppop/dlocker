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

	if opts.TTL != 6*time.Second {
		t.Error("Optinos.TTL init error, current TTL:", opts.TTL)
	}

	if opts.DisableAutoRenewal != false {
		t.Error("Options.DisableAutoRenewal init error, current value:", opts.DisableAutoRenewal)
	}

	if opts.AutoRenewalInterval != 3*time.Second {
		t.Error("Options.AutoRenewalInterval init error, current value:", opts.AutoRenewalInterval)
	}

	if opts.AutoRenewalTTL != 6*time.Second {
		t.Error("Options.AutoRenewalTTL init error, current value:", opts.AutoRenewalTTL)
	}
}
