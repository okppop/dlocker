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
	opts = opts.complete()

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

func TestLockerOptions_complete(t *testing.T) {
	tests := []struct {
		name          string
		input         LockerOptions
		expectedError bool
		expected      LockerOptions
	}{
		{
			name: "Missing Key should panic",
			input: LockerOptions{
				Key: "",
			},
			expectedError: true,
		},
		{
			name: "Custom values should be used when specified",
			input: LockerOptions{
				Key:                 "test-key",
				ValueGeneratorFunc:  func() string { return "custom" },
				RetryInterval:       100 * time.Millisecond,
				TTL:                 10 * time.Second,
				AutoRenewalInterval: 3 * time.Second,
				AutoRenewalTTL:      8 * time.Second,
			},
			expectedError: false,
			expected: LockerOptions{
				Key:                 "test-key",
				ValueGeneratorFunc:  func() string { return "custom" },
				RetryInterval:       100 * time.Millisecond,
				TTL:                 10 * time.Second,
				AutoRenewalInterval: 3 * time.Second,
				AutoRenewalTTL:      8 * time.Second,
			},
		},
		{
			name: "AutoRenewalInterval and AutoRenewalTTL should be set to half TTL and TTL if not specified",
			input: LockerOptions{
				Key: "test-key",
				TTL: 6 * time.Second,
			},
			expectedError: false,
			expected: LockerOptions{
				Key:                 "test-key",
				ValueGeneratorFunc:  defaultValueGenerator,
				RetryInterval:       50 * time.Millisecond,
				TTL:                 6 * time.Second,
				AutoRenewalInterval: 3 * time.Second,
				AutoRenewalTTL:      6 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedError {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("expected panic, but did not panic")
					}
				}()
			}
			got := tt.input.complete()

			if !compareLockerOptions(got, tt.expected) {
				t.Errorf("got %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func compareLockerOptions(a, b LockerOptions) bool {
	return a.Key == b.Key &&
		(a.ValueGeneratorFunc == nil && b.ValueGeneratorFunc == nil || a.ValueGeneratorFunc != nil && b.ValueGeneratorFunc != nil) &&
		a.RetryInterval == b.RetryInterval &&
		a.TTL == b.TTL &&
		a.AutoRenewalInterval == b.AutoRenewalInterval &&
		a.AutoRenewalTTL == b.AutoRenewalTTL
}
