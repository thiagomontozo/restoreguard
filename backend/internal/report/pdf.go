package report

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type RecoveryReport struct {
	Asset, Source, Snapshot, Sandbox, Assessment, Confidence, Limitations string
	SnapshotAge, RPO, RTO, RPOResult, RTOResult                           string
	GeneratedAt                                                           time.Time
	Validations, Evidence                                                 []string
}

func GeneratePDF(r RecoveryReport) []byte {
	lines := []string{"Recovery Verification Report", "Generated: " + r.GeneratedAt.UTC().Format(time.RFC3339), "Protected asset: " + r.Asset, "Backup source: " + r.Source, "Snapshot: " + r.Snapshot, "Snapshot age: " + r.SnapshotAge, "Sandbox: " + r.Sandbox, "Measured RPO: " + r.RPO + " (" + r.RPOResult + ")", "Measured RTO: " + r.RTO + " (" + r.RTOResult + ")", "Assessment: " + r.Assessment, "Recovery confidence: " + r.Confidence, "Validations: " + strings.Join(r.Validations, ", "), "Evidence: " + strings.Join(r.Evidence, ", "), "Limitations: " + r.Limitations, "This report represents the results of a controlled recovery drill under the tested conditions.", "It does not guarantee recovery under every production disaster scenario."}
	var content strings.Builder
	content.WriteString("BT /F1 16 Tf 50 790 Td ")
	for i, line := range lines {
		size := 16
		if i > 0 {
			size = 10
		}
		content.WriteString(fmt.Sprintf("/F1 %d Tf (%s) Tj 0 -24 Td ", size, pdfEscape(line)))
	}
	content.WriteString("ET")
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String())}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)
	return out.Bytes()
}
func pdfEscape(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return ' '
		}
		return r
	}, value)
	return strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(value)
}
