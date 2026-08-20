package report

import (
	"bytes"
	"testing"
	"time"
)

func TestGeneratePDF(t *testing.T) {
	data := GeneratePDF(RecoveryReport{Asset: "Demo ERP", Assessment: "VERIFIED", Confidence: "HIGH", GeneratedAt: time.Now()})
	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) || !bytes.Contains(data, []byte("controlled recovery drill")) {
		t.Fatal("invalid recovery PDF")
	}
}
