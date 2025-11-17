package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nurpe/snowops-acts/internal/config"
	"github.com/nurpe/snowops-acts/internal/model"
	"github.com/nurpe/snowops-acts/internal/repository"
)

type PDFGenerator interface {
	Generate(doc model.ActDocument) ([]byte, error)
}

type ActService struct {
	repo            *repository.ActRepository
	pdf             PDFGenerator
	vatRate         float64
	validStatuses   []string
	numberPrefix    string
	workDescription string
	now             func() time.Time
}

type GenerateActInput struct {
	ContractID  uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Principal   model.Principal
}

type GenerateActResult struct {
	FileName string
	Content  []byte
	Act      model.Act
}

func NewActService(repo *repository.ActRepository, pdf PDFGenerator, cfg *config.Config) *ActService {
	statuses := make([]string, len(cfg.Acts.ValidStatuses))
	for i, status := range cfg.Acts.ValidStatuses {
		statuses[i] = strings.ToUpper(status)
	}
	return &ActService{
		repo:            repo,
		pdf:             pdf,
		vatRate:         cfg.Acts.VATRate,
		validStatuses:   statuses,
		numberPrefix:    cfg.Acts.NumberPrefix,
		workDescription: cfg.Acts.WorkDescription,
		now:             time.Now,
	}
}

func (s *ActService) GenerateActPDF(ctx context.Context, input GenerateActInput) (*GenerateActResult, error) {
	if input.Principal.IsDriver() || input.Principal.IsToo() {
		return nil, ErrPermissionDenied
	}
	if !(input.Principal.IsAkimat() || input.Principal.IsKgu() || input.Principal.IsContractor()) {
		return nil, ErrPermissionDenied
	}
	if input.PeriodStart.IsZero() || input.PeriodEnd.IsZero() {
		return nil, fmt.Errorf("%w: period dates are required", ErrInvalidInput)
	}
	if input.PeriodStart.After(input.PeriodEnd) {
		return nil, fmt.Errorf("%w: period_start must be before or equal to period_end", ErrInvalidInput)
	}

	periodStart := dateOnly(input.PeriodStart)
	periodEnd := dateOnly(input.PeriodEnd)

	contract, err := s.repo.GetContract(ctx, input.ContractID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if input.Principal.IsContractor() && input.Principal.OrgID != contract.ContractorID {
		return nil, ErrPermissionDenied
	}

	contractStart := dateOnly(contract.StartAt)
	contractEnd := dateOnly(contract.EndAt)
	if periodStart.Before(contractStart) {
		return nil, fmt.Errorf("%w: period_start (%s) is before contract start date (%s)", ErrInvalidInput, periodStart.Format("2006-01-02"), contractStart.Format("2006-01-02"))
	}
	if periodEnd.After(contractEnd) {
		return nil, fmt.Errorf("%w: period_end (%s) is after contract end date (%s)", ErrInvalidInput, periodEnd.Format("2006-01-02"), contractEnd.Format("2006-01-02"))
	}

	endExclusive := periodEnd.Add(24 * time.Hour)
	trips, err := s.repo.ListTripsForPeriod(ctx, contract.ID, periodStart, endExclusive, s.validStatuses)
	if err != nil {
		return nil, err
	}

	var totalVolume float64
	tripIDs := make([]uuid.UUID, 0, len(trips))
	for _, trip := range trips {
		totalVolume += trip.VolumeM3
		tripIDs = append(tripIDs, trip.ID)
	}

	totalVolume = round(totalVolume, 3)
	if totalVolume <= 0 {
		return nil, ErrNoTrips
	}

	price := contract.PricePerM3
	amountWoVAT := round(totalVolume*price, 2)
	vatAmount := round(amountWoVAT*s.vatRate/100, 2)
	amountWithVAT := round(amountWoVAT+vatAmount, 2)

	paidBefore, err := s.repo.SumActs(ctx, contract.ID)
	if err != nil {
		return nil, err
	}
	budgetExceeded := false
	if contract.BudgetTotal > 0 && paidBefore+amountWoVAT > contract.BudgetTotal {
		budgetExceeded = true
	}

	now := dateOnly(s.now())
	actNumber := s.buildActNumber(contract.ID, now)

	act := model.Act{
		ContractID:      contract.ID,
		ContractorID:    contract.ContractorID,
		ActNumber:       actNumber,
		ActDate:         now,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		TotalVolumeM3:   totalVolume,
		PricePerM3:      price,
		AmountWoVAT:     amountWoVAT,
		VATRate:         s.vatRate,
		VATAmount:       vatAmount,
		AmountWithVAT:   amountWithVAT,
		Status:          "GENERATED",
		CreatedByOrgID:  input.Principal.OrgID,
		CreatedByUserID: input.Principal.UserID,
	}

	savedAct, err := s.repo.CreateAct(ctx, act, tripIDs)
	if err != nil {
		return nil, err
	}

	doc := model.ActDocument{
		Act:             *savedAct,
		Contract:        *contract,
		WorkDescription: s.workDescription,
		PaidBefore:      paidBefore,
		BudgetExceeded:  budgetExceeded,
	}

	content, err := s.pdf.Generate(doc)
	if err != nil {
		return nil, err
	}

	fileName := fmt.Sprintf("act-%s.pdf", sanitizeFileName(savedAct.ActNumber))
	return &GenerateActResult{
		FileName: fileName,
		Content:  content,
		Act:      *savedAct,
	}, nil
}

func (s *ActService) buildActNumber(contractID uuid.UUID, actDate time.Time) string {
	hash := strings.ToUpper(contractID.String())
	if len(hash) > 8 {
		hash = hash[:8]
	}
	return fmt.Sprintf("%s-%s-%s", s.numberPrefix, hash, actDate.Format("20060102"))
}

func round(value float64, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	return math.Round(value*factor) / factor
}

func dateOnly(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sanitizeFileName(input string) string {
	result := make([]rune, 0, len(input))
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, r)
		case r >= 'A' && r <= 'Z':
			result = append(result, r)
		case r >= '0' && r <= '9':
			result = append(result, r)
		case r == '-', r == '_':
			result = append(result, r)
		default:
			result = append(result, '-')
		}
	}
	return strings.Trim(string(result), "-")
}
