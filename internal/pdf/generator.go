package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/nurpe/snowops-acts/internal/model"
)

type Generator struct {
	fontName string
}

func NewGenerator() (*Generator, error) {
	if len(notoSansFont) == 0 {
		return nil, fmt.Errorf("font data is empty")
	}
	return &Generator{fontName: "NotoSans"}, nil
}

func (g *Generator) Generate(doc model.ActDocument) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	pdf.AddUTF8FontFromBytes(g.fontName, "", notoSansFont)
	pdf.AddUTF8FontFromBytes(g.fontName, "B", notoSansFont)

	pdf.SetFont(g.fontName, "B", 14)
	pdf.CellFormat(0, 10, "АКТ выполненных работ (форма Р-1)", "", 1, "C", false, 0, "")

	pdf.SetFont(g.fontName, "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Акт № %s от %s", doc.Act.ActNumber, formatDate(doc.Act.ActDate)), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Договор № %s (%s — %s)", doc.Contract.Name, formatDate(doc.Contract.StartAt), formatDate(doc.Contract.EndAt)), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	addOrgBlock(pdf, g.fontName, "Заказчик", doc.Contract.Customer)
	pdf.Ln(2)
	addOrgBlock(pdf, g.fontName, "Исполнитель", doc.Contract.Contractor)
	pdf.Ln(4)

	pdf.SetFont(g.fontName, "B", 12)
	pdf.CellFormat(0, 8, "Период работ", "", 1, "L", false, 0, "")
	pdf.SetFont(g.fontName, "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("с %s по %s", formatDate(doc.Act.PeriodStart), formatDate(doc.Act.PeriodEnd)), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(g.fontName, "B", 12)
	pdf.CellFormat(0, 8, "Объём и стоимость", "", 1, "L", false, 0, "")
	pdf.SetFont(g.fontName, "", 10)

	headers := []string{"Наименование работ", "Ед. изм.", "Кол-во", "Цена без НДС", "Сумма без НДС"}
	colWidths := []float64{110, 25, 30, 45, 45}
	drawTableRow(pdf, g.fontName, headers, colWidths, true)

	row := []string{
		doc.WorkDescription,
		"м³",
		formatAmount(doc.Act.TotalVolumeM3, 3),
		formatAmount(doc.Act.PricePerM3, 2),
		formatAmount(doc.Act.AmountWoVAT, 2),
	}
	drawTableRow(pdf, g.fontName, row, colWidths, false)

	pdf.Ln(2)
	pdf.SetFont(g.fontName, "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("НДС (%.2f%%): %s тг.", doc.Act.VATRate, formatAmount(doc.Act.VATAmount, 2)), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Всего с НДС: %s тг.", formatAmount(doc.Act.AmountWithVAT, 2)), "", 1, "R", false, 0, "")

	paidAfter := doc.PaidBefore + doc.Act.AmountWoVAT
	pdf.CellFormat(0, 6, fmt.Sprintf("Оплачено ранее: %s тг., после акта: %s тг. из бюджета %s тг.",
		formatAmount(doc.PaidBefore, 2),
		formatAmount(paidAfter, 2),
		formatAmount(doc.Contract.BudgetTotal, 2),
	), "", 1, "L", false, 0, "")

	if doc.BudgetExceeded {
		pdf.SetTextColor(200, 0, 0)
		pdf.MultiCell(0, 6, "Внимание: сумма акта превышает доступный бюджет контракта.", "", "L", false)
		pdf.SetTextColor(0, 0, 0)
	}

	pdf.Ln(4)
	pdf.SetFont(g.fontName, "B", 12)
	pdf.CellFormat(0, 8, "Подписи сторон", "", 1, "L", false, 0, "")
	pdf.SetFont(g.fontName, "", 11)

	signatureBlock(pdf, g.fontName, "Заказчик", doc.Contract.Customer.HeadFullName)
	signatureBlock(pdf, g.fontName, "Исполнитель", doc.Contract.Contractor.HeadFullName)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addOrgBlock(pdf *gofpdf.Fpdf, fontName, title string, org model.Organization) {
	pdf.SetFont(fontName, "B", 11)
	pdf.CellFormat(0, 6, title, "", 1, "L", false, 0, "")
	pdf.SetFont(fontName, "", 10)
	lines := []string{
		org.Name,
		fmt.Sprintf("БИН: %s", safeValue(org.BIN)),
		fmt.Sprintf("Адрес: %s", safeValue(org.Address)),
		fmt.Sprintf("Телефон: %s", safeValue(org.Phone)),
	}
	for _, line := range lines {
		pdf.MultiCell(0, 5, line, "", "L", false)
	}
}

func drawTableRow(pdf *gofpdf.Fpdf, fontName string, cols []string, widths []float64, header bool) {
	style := ""
	if header {
		style = "B"
	}
	pdf.SetFont(fontName, style, 10)
	for i, col := range cols {
		border := "1"
		align := "L"
		if i > 1 {
			align = "R"
		}
		pdf.CellFormat(widths[i], 8, col, border, 0, align, false, 0, "")
	}
	pdf.Ln(-1)
}

func signatureBlock(pdf *gofpdf.Fpdf, fontName, label, head string) {
	pdf.SetFont(fontName, "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("%s: ______________________ /%s/", label, safeValue(head)), "", 1, "L", false, 0, "")
}

func safeValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func formatAmount(value float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, value)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("02.01.2006")
}
