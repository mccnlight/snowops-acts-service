package excel

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/nurpe/snowops-acts/internal/model"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(report model.ActReport) ([]byte, error) {
	file := excelize.NewFile()

	summarySheet := "Summary"
	file.SetSheetName("Sheet1", summarySheet)
	if err := g.writeSummary(file, summarySheet, report); err != nil {
		return nil, err
	}

	usedNames := map[string]struct{}{summarySheet: {}}
	for _, group := range report.Groups {
		sheetName := buildSheetName(report.Mode, group.Name, group.ID, usedNames)
		usedNames[sheetName] = struct{}{}

		file.NewSheet(sheetName)
		if err := g.writeDetail(file, sheetName, report, group); err != nil {
			return nil, err
		}
	}

	file.SetActiveSheet(0)
	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *Generator) writeSummary(file *excelize.File, sheet string, report model.ActReport) error {
	modeLabel, groupLabel := reportLabels(report.Mode)
	totalVolume := sumReportVolume(report)

	set := func(cell string, value interface{}) {
		_ = file.SetCellValue(sheet, cell, value)
	}

	set("A1", "Report Type")
	set("B1", modeLabel)
	set("A2", "Target")
	set("B2", report.Target.Name)
	set("A3", "Target ID")
	set("B3", report.Target.ID.String())
	set("A4", "Period Start")
	set("B4", formatDate(report.PeriodStart))
	set("A5", "Period End")
	set("B5", formatDate(report.PeriodEnd))
	set("A6", "Total Trips")
	set("B6", report.TotalTrips)
	set("A7", "Total Volume M3")
	set("B7", formatFloatValue(totalVolume, true))

	tableRow := 9
	set(fmt.Sprintf("A%d", tableRow), "ID")
	set(fmt.Sprintf("B%d", tableRow), groupLabel)
	set(fmt.Sprintf("C%d", tableRow), "Trip Count")
	set(fmt.Sprintf("D%d", tableRow), "Volume M3")

	for i, group := range report.Groups {
		row := tableRow + 1 + i
		set(fmt.Sprintf("A%d", row), group.ID.String())
		set(fmt.Sprintf("B%d", row), group.Name)
		set(fmt.Sprintf("C%d", row), group.TripCount)
		set(fmt.Sprintf("D%d", row), formatFloatValue(sumGroupVolume(report.Mode, group), true))
	}

	_ = file.SetColWidth(sheet, "A", "A", 38)
	_ = file.SetColWidth(sheet, "B", "B", 45)
	_ = file.SetColWidth(sheet, "C", "C", 16)
	_ = file.SetColWidth(sheet, "D", "D", 16)
	return nil
}

func (g *Generator) writeDetail(file *excelize.File, sheet string, report model.ActReport, group model.TripGroup) error {
	modeLabel, groupLabel := reportLabels(report.Mode)
	groupVolume := sumGroupVolume(report.Mode, group)

	set := func(cell string, value interface{}) {
		_ = file.SetCellValue(sheet, cell, value)
	}

	set("A1", "Report Type")
	set("B1", modeLabel)
	set("A2", "Target")
	set("B2", report.Target.Name)
	set("A3", "Target ID")
	set("B3", report.Target.ID.String())
	set("A4", groupLabel)
	set("B4", group.Name)
	set("A5", "Group ID")
	set("B5", group.ID.String())
	set("A6", "Period Start")
	set("B6", formatDate(report.PeriodStart))
	set("A7", "Period End")
	set("B7", formatDate(report.PeriodEnd))
	set("A8", "Trip Count")
	set("B8", group.TripCount)
	set("A9", "Total Volume M3")
	set("B9", formatFloatValue(groupVolume, true))

	tableRow := 11
	headers := []string{
		"Trip ID",
		"Entry At",
		"Exit At",
		"Status",
		"Polygon ID",
		"Polygon Name",
		"Contractor ID",
		"Contractor Name",
		"Vehicle Plate",
		"Detected Plate",
		"Volume Entry",
		"Volume Exit",
		"Volume M3",
	}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, tableRow)
		set(cell, header)
	}

	for i, trip := range group.Trips {
		row := tableRow + 1 + i
		set(fmt.Sprintf("A%d", row), trip.ID.String())
		set(fmt.Sprintf("B%d", row), formatDateTime(trip.EntryAt))
		set(fmt.Sprintf("C%d", row), formatOptionalTime(trip.ExitAt))
		set(fmt.Sprintf("D%d", row), trip.Status)
		set(fmt.Sprintf("E%d", row), formatUUID(trip.PolygonID))
		set(fmt.Sprintf("F%d", row), formatString(trip.PolygonName))
		set(fmt.Sprintf("G%d", row), formatUUID(trip.ContractorID))
		set(fmt.Sprintf("H%d", row), formatString(trip.ContractorName))
		set(fmt.Sprintf("I%d", row), formatString(trip.VehiclePlateNumber))
		set(fmt.Sprintf("J%d", row), formatString(trip.DetectedPlateNumber))
		set(fmt.Sprintf("K%d", row), formatFloat(trip.DetectedVolumeEntry))
		set(fmt.Sprintf("L%d", row), formatFloat(trip.DetectedVolumeExit))
		set(fmt.Sprintf("M%d", row), formatFloatValue(computeTripVolume(report.Mode, trip)))
	}

	_ = file.SetColWidth(sheet, "A", "A", 36)
	_ = file.SetColWidth(sheet, "B", "C", 20)
	_ = file.SetColWidth(sheet, "D", "D", 14)
	_ = file.SetColWidth(sheet, "E", "G", 36)
	_ = file.SetColWidth(sheet, "F", "H", 28)
	_ = file.SetColWidth(sheet, "I", "J", 16)
	_ = file.SetColWidth(sheet, "K", "M", 14)
	return nil
}

func reportLabels(mode model.ReportMode) (string, string) {
	switch mode {
	case model.ReportModeLandfill:
		return "Landfill", "Contractor"
	case model.ReportModeContractor:
		return "Contractor", "Landfill"
	default:
		return "Unknown", "Group"
	}
}

func buildSheetName(mode model.ReportMode, name string, id uuid.UUID, used map[string]struct{}) string {
	_, groupLabel := reportLabels(mode)
	base := fmt.Sprintf("%s - %s", groupLabel, strings.TrimSpace(name))
	if base == strings.TrimSpace(groupLabel)+" -" || strings.TrimSpace(name) == "" {
		base = fmt.Sprintf("%s - %s", groupLabel, id.String())
	}
	base = sanitizeSheetName(base)

	if len(base) > 31 {
		base = base[:31]
	}

	nameCandidate := base
	counter := 2
	for {
		if _, exists := used[nameCandidate]; !exists {
			return nameCandidate
		}
		suffix := fmt.Sprintf("-%d", counter)
		trimmed := base
		if len(trimmed)+len(suffix) > 31 {
			trimmed = trimmed[:31-len(suffix)]
		}
		nameCandidate = trimmed + suffix
		counter++
	}
}

func sanitizeSheetName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sheet"
	}

	replacer := strings.NewReplacer(
		"[", "-",
		"]", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"/", "-",
		"\\", "-",
	)
	value = replacer.Replace(value)
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sheet"
	}
	return value
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

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatDateTime(*t)
}

func formatUUID(id *uuid.UUID) string {
	if id == nil || *id == uuid.Nil {
		return ""
	}
	return id.String()
}

func formatString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.3f", *value)
}

func formatFloatValue(value float64, ok bool) string {
	if !ok {
		return ""
	}
	return fmt.Sprintf("%.3f", value)
}

func sumGroupVolume(mode model.ReportMode, group model.TripGroup) float64 {
	total := 0.0
	for _, trip := range group.Trips {
		if volume, ok := computeTripVolume(mode, trip); ok {
			total += volume
		}
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

func computeTripVolume(_ model.ReportMode, trip model.TripDetail) (float64, bool) {
	if trip.TotalVolumeM3 != nil {
		return *trip.TotalVolumeM3, true
	}
	if trip.DetectedVolumeEntry != nil {
		return *trip.DetectedVolumeEntry, true
	}
	if trip.DetectedVolumeExit != nil {
		return *trip.DetectedVolumeExit, true
	}
	return 0, false
}
