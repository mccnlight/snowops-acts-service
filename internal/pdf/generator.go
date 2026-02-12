package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/nurpe/snowops-acts/internal/model"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(report model.ActReport) ([]byte, error) {
	p := gofpdf.New("P", "mm", "A4", "")
	if err := configureUnicodeFont(p); err != nil {
		return nil, err
	}
	p.SetTitle("SnowOps Acts Report", false)
	p.SetAuthor("snowops-acts-service", false)
	p.SetMargins(10, 10, 10)
	p.SetAutoPageBreak(true, 10)
	p.AddPage()

	p.SetFont("Unicode", "", 14)
	p.Cell(0, 8, "Act report")
	p.Ln(10)

	p.SetFont("Unicode", "", 11)
	p.Cell(0, 6, fmt.Sprintf("Mode: %s", string(report.Mode)))
	p.Ln(6)
	p.Cell(0, 6, fmt.Sprintf("Organization: %s", report.Target.Name))
	p.Ln(6)
	p.Cell(0, 6, fmt.Sprintf("Period: %s - %s", formatDate(report.PeriodStart), formatDate(report.PeriodEnd)))
	p.Ln(6)
	p.Cell(0, 6, fmt.Sprintf("Total trips: %d", report.TotalTrips))
	p.Ln(6)
	p.Cell(0, 6, fmt.Sprintf("Total volume (m3): %.2f", sumReportVolume(report)))
	p.Ln(10)

	p.SetFont("Unicode", "", 10)
	p.CellFormat(70, 7, groupLabel(report.Mode), "1", 0, "L", false, 0, "")
	p.CellFormat(32, 7, "Trips", "1", 0, "C", false, 0, "")
	p.CellFormat(32, 7, "Volume (m3)", "1", 1, "R", false, 0, "")

	p.SetFont("Unicode", "", 9)
	for _, group := range report.Groups {
		p.CellFormat(70, 6, trim(group.Name, 36), "1", 0, "L", false, 0, "")
		p.CellFormat(32, 6, fmt.Sprintf("%d", group.TripCount), "1", 0, "C", false, 0, "")
		p.CellFormat(32, 6, fmt.Sprintf("%.2f", sumGroupVolume(report.Mode, group)), "1", 1, "R", false, 0, "")
	}

	for _, group := range report.Groups {
		if len(group.Trips) == 0 {
			continue
		}
		p.AddPage()
		p.SetFont("Unicode", "", 12)
		p.Cell(0, 8, fmt.Sprintf("%s: %s", groupLabel(report.Mode), group.Name))
		p.Ln(10)

		p.SetFont("Unicode", "", 9)
		p.CellFormat(36, 7, "Date time", "1", 0, "L", false, 0, "")
		p.CellFormat(30, 7, "Plate", "1", 0, "L", false, 0, "")
		p.CellFormat(50, 7, relatedLabel(report.Mode), "1", 0, "L", false, 0, "")
		p.CellFormat(24, 7, "Volume", "1", 1, "R", false, 0, "")

		p.SetFont("Unicode", "", 8)
		for _, trip := range group.Trips {
			p.CellFormat(36, 6, formatDateTime(trip.EventTime), "1", 0, "L", false, 0, "")
			p.CellFormat(30, 6, trim(strPtr(trip.Plate), 15), "1", 0, "L", false, 0, "")
			p.CellFormat(50, 6, trim(relatedName(report.Mode, trip), 28), "1", 0, "L", false, 0, "")
			p.CellFormat(24, 6, fmt.Sprintf("%.2f", floatPtr(trip.SnowVolumeM3)), "1", 1, "R", false, 0, "")
		}
	}

	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func configureUnicodeFont(p *gofpdf.Fpdf) error {
	fontPath, err := findUnicodeFontPath()
	if err != nil {
		return err
	}
	p.AddUTF8Font("Unicode", "", fontPath)
	if p.Err() {
		return errors.New("failed to register UTF-8 font for PDF")
	}
	return nil
}

func findUnicodeFontPath() (string, error) {
	candidates := make([]string, 0, 8)
	if v := os.Getenv("PDF_FONT_PATH"); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates,
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\segoeui.ttf`,
		`/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`,
		`/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf`,
		`/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf`,
	)

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return filepath.Clean(path), nil
		}
	}
	return "", errors.New("unicode font not found: set PDF_FONT_PATH to a .ttf font with Cyrillic support")
}

func groupLabel(mode model.ReportMode) string {
	if mode == model.ReportModeLandfill {
		return "Contractor"
	}
	return "Landfill"
}

func relatedLabel(mode model.ReportMode) string {
	if mode == model.ReportModeLandfill {
		return "Contractor"
	}
	return "Landfill"
}

func relatedName(mode model.ReportMode, trip model.TripDetail) string {
	if mode == model.ReportModeLandfill {
		return strPtr(trip.ContractorName)
	}
	return strPtr(trip.PolygonName)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func strPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func floatPtr(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func trim(v string, max int) string {
	runes := []rune(v)
	if len(runes) <= max {
		return v
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func sumGroupVolume(mode model.ReportMode, group model.TripGroup) float64 {
	total := 0.0
	for _, trip := range group.Trips {
		if trip.SnowVolumeM3 != nil {
			total += *trip.SnowVolumeM3
		}
	}
	if mode == model.ReportModeLandfill || mode == model.ReportModeContractor {
		return total
	}
	return total
}

func sumReportVolume(report model.ActReport) float64 {
	total := 0.0
	for _, group := range report.Groups {
		total += sumGroupVolume(report.Mode, group)
	}
	return total
}
