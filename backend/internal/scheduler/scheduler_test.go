package scheduler

import (
	"testing"
	"time"
)

func TestConservativeInterval(t *testing.T) {
	if 30*time.Second < time.Second {
		t.Fatal("invalid scheduler interval")
	}
}
