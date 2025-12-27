package analyze

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	StatusOK      = "ok"
	StatusPartial = "partial"
	StatusError   = "error"
	StatusBreach  = "breach"
)

type Options struct {
	Project  string
	Service  string
	Start    string
	End      string
	Last     time.Duration
	OutDir   string
	Format   []string
	Explain  bool
	Timezone *time.Location
	MaxSLOs  int
	Only     *regexp.Regexp
}

type Reader interface {
	ListServiceLevelObjectives(ctx context.Context, serviceName string, max int) ([]SLO, error)
	FetchCompliance(ctx context.Context, project string, sloName string, start, end time.Time) (float64, error)
}

type SLO struct {
	Name        string
	DisplayName string
	Goal        float64
	RollingDays int64
	Calendar    *string
	SLIType     string
	SLIMethod   string
}

func Run(ctx context.Context, reader Reader, opts Options) (Result, Sources, string, error) {
	if opts.Project == "" {
		return Result{}, Sources{}, "", errors.New("--project is required")
	}
	if opts.Service == "" {
		return Result{}, Sources{}, "", errors.New("--service is required")
	}

	serviceName, serviceID, err := NormalizeService(opts.Project, opts.Service)
	if err != nil {
		return Result{}, Sources{}, "", err
	}
	start, end, err := ResolveWindow(opts.Start, opts.End, opts.Last, time.Now().UTC())
	if err != nil {
		return Result{}, Sources{}, "", err
	}
	if opts.Timezone == nil {
		opts.Timezone = time.UTC
	}
	if opts.MaxSLOs <= 0 {
		opts.MaxSLOs = 50
	}

	outDir := opts.OutDir
	if outDir == "" {
		stamp := start.In(time.UTC).Format("20060102-150405")
		outDir = filepath.Join("out", "margin-analyze", fmt.Sprintf("%s-%s", stamp, sanitizeSegment(serviceID)))
	}

	slos, err := reader.ListServiceLevelObjectives(ctx, serviceName, opts.MaxSLOs)
	if err != nil {
		return Result{}, Sources{}, outDir, err
	}

	slos = filterSLOs(slos, opts.Only)
	sort.Slice(slos, func(i, j int) bool {
		if slos[i].DisplayName == slos[j].DisplayName {
			return slos[i].Name < slos[j].Name
		}
		return slos[i].DisplayName < slos[j].DisplayName
	})

	result := Result{
		SchemaVersion: SchemaVersion,
		Project:       opts.Project,
		Service:       serviceID,
		Window: Window{
			Start:           start,
			End:             end,
			DurationSeconds: int64(end.Sub(start).Seconds()),
		},
	}

	var errorsList []string
	for _, slo := range slos {
		item := SLOResult{
			SLOResourceName:   slo.Name,
			SLOID:             extractSLOID(slo.Name),
			DisplayName:       slo.DisplayName,
			Goal:              round4(slo.Goal),
			RollingPeriodDays: slo.RollingDays,
			CalendarPeriod:    slo.Calendar,
		}

		supported, supportNote := supportedSLO(slo)
		if !supported {
			item.Status = StatusPartial
			item.Error = supportNote
			if opts.Explain {
				item.Explain = &Explain{
					Formula: budgetFormula(),
					Notes:   []string{supportNote},
				}
			}
			result.SLOs = append(result.SLOs, item)
			errorsList = append(errorsList, fmt.Sprintf("%s: %s", slo.DisplayName, supportNote))
			continue
		}

		compliance, err := reader.FetchCompliance(ctx, opts.Project, slo.Name, start, end)
		if err != nil {
			item.Status = StatusError
			item.Error = err.Error()
			result.SLOs = append(result.SLOs, item)
			errorsList = append(errorsList, fmt.Sprintf("%s: %s", slo.DisplayName, err.Error()))
			continue
		}

		allowedBad, bad, consumed, notes := ComputeBudget(slo.Goal, compliance)
		item.Compliance = round4(compliance)
		item.BadFraction = round4(bad)
		item.AllowedBadFraction = round4(allowedBad)
		item.ConsumedPercentOfBudget = round4(consumed)
		item.Status = StatusOK

		if allowedBad <= 0 {
			item.Status = StatusPartial
			note := "goal is 100%; cannot compute allowed bad fraction"
			item.Error = note
			notes = append(notes, note)
			errorsList = append(errorsList, fmt.Sprintf("%s: %s", slo.DisplayName, note))
		}
		if allowedBad > 0 && consumed > 100 {
			item.Status = StatusBreach
			note := "error budget exceeded in window"
			item.Error = note
			notes = append(notes, note)
		}

		if opts.Explain {
			item.Explain = &Explain{
				Formula: budgetFormula(),
				Notes:   notes,
			}
		}

		result.SLOs = append(result.SLOs, item)
	}

	result.Errors = errorsList
	result.Status = overallStatus(result.SLOs, errorsList)

	sources := Sources{
		Project:     opts.Project,
		Service:     serviceID,
		ServiceName: serviceName,
		Start:       start.Format(time.RFC3339),
		End:         end.Format(time.RFC3339),
	}
	for _, slo := range slos {
		sources.SLOs = append(sources.SLOs, slo.Name)
	}

	return result, sources, outDir, nil
}

func overallStatus(slos []SLOResult, errorsList []string) string {
	status := StatusOK
	for _, slo := range slos {
		if slo.Status == StatusBreach {
			return StatusBreach
		}
		if slo.Status == StatusPartial || slo.Status == StatusError {
			status = StatusPartial
		}
	}
	if status == StatusOK && len(errorsList) > 0 {
		status = StatusPartial
	}
	return status
}

func supportedSLO(slo SLO) (bool, string) {
	if slo.SLIType != "request-based" {
		return false, fmt.Sprintf("unsupported SLI type %q", slo.SLIType)
	}
	if slo.SLIMethod != "good-total-ratio" && slo.SLIMethod != "distribution-cut" {
		return false, fmt.Sprintf("unsupported SLI method %q", slo.SLIMethod)
	}
	return true, ""
}

func budgetFormula() string {
	return "allowedBad = 1 - goal; bad = 1 - compliance; consumedPercent = (bad / allowedBad) * 100"
}

func filterSLOs(slos []SLO, re *regexp.Regexp) []SLO {
	if re == nil {
		return slos
	}
	var out []SLO
	for _, slo := range slos {
		if re.MatchString(slo.DisplayName) || re.MatchString(slo.Name) {
			out = append(out, slo)
		}
	}
	return out
}

func extractSLOID(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return name
	}
	return parts[len(parts)-1]
}

func sanitizeSegment(input string) string {
	var out []rune
	for _, r := range strings.ToLower(input) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		} else if r == '.' {
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "service"
	}
	return string(out)
}
