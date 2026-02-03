package service

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/nurpe/snowops-acts/internal/config"
    "github.com/nurpe/snowops-acts/internal/model"
    "github.com/nurpe/snowops-acts/internal/repository"
)

type ExcelGenerator interface {
    Generate(report model.ActReport) ([]byte, error)
}

type ActService struct {
	repo          *repository.ReportRepository
	excel         ExcelGenerator
}

type GenerateReportInput struct {
    Mode        model.ReportMode
    TargetID    uuid.UUID
    PeriodStart time.Time
    PeriodEnd   time.Time
    Principal   model.Principal
}

type GenerateReportResult struct {
    FileName string
    Content  []byte
}

func NewActService(repo *repository.ReportRepository, excel ExcelGenerator, cfg *config.Config) *ActService {
    return &ActService{
        repo:          repo,
        excel:         excel,
	}
}

func (s *ActService) GenerateReport(ctx context.Context, input GenerateReportInput) (*GenerateReportResult, error) {
    if input.Principal.IsDriver() {
        return nil, ErrPermissionDenied
    }
    if input.TargetID == uuid.Nil {
        return nil, fmt.Errorf("%w: target_id is required", ErrInvalidInput)
    }
    if input.PeriodStart.IsZero() || input.PeriodEnd.IsZero() {
        return nil, fmt.Errorf("%w: period dates are required", ErrInvalidInput)
    }

    periodStart := dateOnly(input.PeriodStart)
    periodEnd := dateOnly(input.PeriodEnd)
    if periodStart.After(periodEnd) {
        return nil, fmt.Errorf("%w: period_start must be before or equal to period_end", ErrInvalidInput)
    }

    endExclusive := periodEnd.Add(24 * time.Hour)

	var target *model.Organization
	var groups []model.TripGroup
	var landfillPolygonIDs []uuid.UUID

    switch input.Mode {
    case model.ReportModeContractor:
        if !(input.Principal.IsAkimat() || input.Principal.IsKgu() || input.Principal.IsContractor()) {
            return nil, ErrPermissionDenied
        }
        if input.Principal.IsContractor() && input.Principal.OrgID != input.TargetID {
            return nil, ErrPermissionDenied
        }

        org, err := s.repo.GetOrganization(ctx, input.TargetID)
        if err != nil {
            if err == gorm.ErrRecordNotFound {
                return nil, ErrNotFound
            }
            return nil, err
        }
        target = org

		polygons, err := s.repo.ListPolygons(ctx)
		if err != nil {
			return nil, err
		}
		counts, err := s.repo.EventCountsByPolygon(ctx, input.TargetID, periodStart, endExclusive)
		if err != nil {
			return nil, err
		}
		groups = mergeGroups(polygons, counts)

    case model.ReportModeLandfill:
        if !(input.Principal.IsAkimat() || input.Principal.IsKgu() || input.Principal.IsLandfill()) {
            return nil, ErrPermissionDenied
        }

        polygonID, polygonName, err := s.repo.GetPolygon(ctx, input.TargetID)
        if err != nil {
            if err == gorm.ErrRecordNotFound {
                return nil, ErrNotFound
            }
            return nil, err
        }
        target = &model.Organization{
            ID:   polygonID,
            Name: polygonName,
            Type: "LANDFILL",
        }

		landfillPolygonIDs = []uuid.UUID{polygonID}
		contractors, err := s.repo.ListContractors(ctx)
		if err != nil {
			return nil, err
		}
		counts, err := s.repo.EventCountsByContractor(ctx, polygonIDs, periodStart, endExclusive)
		if err != nil {
			return nil, err
		}
		groups = mergeGroups(contractors, counts)

    default:
        return nil, fmt.Errorf("%w: invalid report mode", ErrInvalidInput)
    }

	totalTrips := int64(0)
	for _, group := range groups {
		totalTrips += group.TripCount
	}
	for i := range groups {
		switch input.Mode {
		case model.ReportModeContractor:
			if groups[i].ID == uuid.Nil {
				continue
			}
			trips, err := s.repo.ListEventsByPolygon(ctx, input.TargetID, groups[i].ID, periodStart, endExclusive)
			if err != nil {
				return nil, err
			}
			groups[i].Trips = trips
		case model.ReportModeLandfill:
			if groups[i].ID == uuid.Nil {
				continue
			}
			trips, err := s.repo.ListEventsByContractor(ctx, groups[i].ID, landfillPolygonIDs, periodStart, endExclusive)
			if err != nil {
				return nil, err
			}
			groups[i].Trips = trips
		}
	}

	report := model.ActReport{
		Mode:        input.Mode,
		Target:      *target,
		PeriodStart: periodStart,
        PeriodEnd:   periodEnd,
        TotalTrips:  totalTrips,
        Groups:      groups,
    }

    content, err := s.excel.Generate(report)
    if err != nil {
        return nil, err
    }

    fileName := s.buildFileName(report)
    return &GenerateReportResult{
        FileName: fileName,
        Content:  content,
    }, nil
}

func (s *ActService) buildFileName(report model.ActReport) string {
    mode := strings.ToLower(string(report.Mode))
    target := sanitizeFileName(report.Target.Name)
    if target == "" {
        target = report.Target.ID.String()
    }
    period := fmt.Sprintf("%s-%s", report.PeriodStart.Format("20060102"), report.PeriodEnd.Format("20060102"))
    return fmt.Sprintf("acts-%s-%s-%s.xlsx", mode, target, period)
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

func mergeGroups(base []model.TripGroup, counts []model.TripGroup) []model.TripGroup {
	result := make([]model.TripGroup, 0, len(base)+len(counts))
	index := make(map[uuid.UUID]int, len(base))

	for _, group := range base {
		result = append(result, group)
		index[group.ID] = len(result) - 1
	}

	for _, group := range counts {
		if pos, ok := index[group.ID]; ok {
			result[pos].TripCount = group.TripCount
			if result[pos].Name == "" {
				result[pos].Name = group.Name
			}
			continue
		}
		result = append(result, group)
		index[group.ID] = len(result) - 1
	}

	return result
}
