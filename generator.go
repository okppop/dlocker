package lockerd

import (
	"os"

	"github.com/google/uuid"
)

// defaultValueGenerator produce string like that:
// hostname_fbe262ca-d136-4293-b1fd-644b49c7b548
func defaultValueGenerator() string {
	return getHostName() + "_" + uuid.New().String()
}

func getHostName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown-host"
	}

	if hostname == "" || hostname == "localhost" {
		return "unknown-host"
	}

	return hostname
}
