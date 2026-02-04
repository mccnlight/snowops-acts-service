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

	summarySheet := "Сводка"
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

	set("A1", "Тип отчета")
	set("B1", modeLabel)
	set("A2", "Организация")
	set("B2", report.Target.Name)
	set("A3", "Начало периода")
	set("B3", formatDate(report.PeriodStart))
	set("A4", "Конец периода")
	set("B4", formatDate(report.PeriodEnd))
	set("A5", "Количество рейсов")
	set("B5", report.TotalTrips)
	set("A6", "Объем снега, м3")
	set("B6", formatFloatValue(totalVolume, true))

	tableRow := 8
	set(fmt.Sprintf("A%d", tableRow), groupLabel)
	set(fmt.Sprintf("B%d", tableRow), "Количество рейсов")
	set(fmt.Sprintf("C%d", tableRow), "Объем снега, м3")

	for i, group := range report.Groups {
		row := tableRow + 1 + i
		set(fmt.Sprintf("A%d", row), group.Name)
		set(fmt.Sprintf("B%d", row), group.TripCount)
		set(fmt.Sprintf("C%d", row), formatFloatValue(sumGroupVolume(report.Mode, group), true))
	}

	_ = file.SetColWidth(sheet, "A", "A", 45)
	_ = file.SetColWidth(sheet, "B", "B", 16)
	_ = file.SetColWidth(sheet, "C", "C", 16)
	return nil
}

func (g *Generator) writeDetail(file *excelize.File, sheet string, report model.ActReport, group model.TripGroup) error {
	modeLabel, groupLabel := reportLabels(report.Mode)
	groupVolume := sumGroupVolume(report.Mode, group)

	set := func(cell string, value interface{}) {
		_ = file.SetCellValue(sheet, cell, value)
	}

	set("A1", "Тип отчета")
	set("B1", modeLabel)
	set("A2", "Организация")
	set("B2", report.Target.Name)
	set("A3", groupLabel)
	set("B3", group.Name)
	set("A4", "Начало периода")
	set("B4", formatDate(report.PeriodStart))
	set("A5", "Конец периода")
	set("B5", formatDate(report.PeriodEnd))
	set("A6", "Количество рейсов")
	set("B6", group.TripCount)
	set("A7", "Объем снега, м3")
	set("B7", formatFloatValue(groupVolume, true))

	tableRow := 9
	headers := []string{
		"Дата",
		"Номер машины",
		"Полигон",
		"Подрядчик",
		"Объем снега, м3",
	}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, tableRow)
		set(cell, header)
	}

	for i, trip := range group.Trips {
		row := tableRow + 1 + i
		set(fmt.Sprintf("A%d", row), formatDateTime(trip.EventTime))
		set(fmt.Sprintf("B%d", row), formatString(trip.Plate))
		set(fmt.Sprintf("C%d", row), formatString(trip.PolygonName))
		set(fmt.Sprintf("D%d", row), formatString(trip.ContractorName))
		set(fmt.Sprintf("E%d", row), formatFloat(trip.SnowVolumeM3))
	}

	_ = file.SetColWidth(sheet, "A", "A", 20)
	_ = file.SetColWidth(sheet, "B", "B", 16)
	_ = file.SetColWidth(sheet, "C", "D", 32)
	_ = file.SetColWidth(sheet, "E", "E", 14)
	return nil
}

func reportLabels(mode model.ReportMode) (string, string) {
	switch mode {
	case model.ReportModeLandfill:
		return "Полигон", "Подрядчик"
	case model.ReportModeContractor:
		return "Подрядчик", "Полигон"
	default:
		return "Отчет", "Группа"
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
		return "Лист"
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
		return "Лист"
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
	if trip.SnowVolumeM3 == nil {
		return 0, false
	}
	return *trip.SnowVolumeM3, true
}
