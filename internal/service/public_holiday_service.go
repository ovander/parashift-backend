package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg/httpx"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

const (
	// govAPIBase is the base URL for the official French government public holiday API.
	// Docs: https://www.data.gouv.fr/dataservices/jours-feries
	// Endpoint: GET /jours-feries/{zone}/{year}.json
	// Response:  {"YYYY-MM-DD": "Nom du jour", ...}  — names already in French.
	govAPIBase = "https://calendrier.api.gouv.fr/jours-feries"

	// DefaultZone is the metropolitan France zone used for all stores.
	// Other valid zones: alsace-moselle, guadeloupe, guyane, la-reunion,
	// martinique, mayotte, nouvelle-caledonie, etc.
	DefaultZone = "metropole"
)

// PublicHolidayService fetches and caches French public holidays.
// Holidays are fetched once per year from the government API and persisted in
// the database so subsequent checks are instant and work offline.
type PublicHolidayService struct {
	repo       repo.PublicHolidayRepository
	client     *http.Client
	logger     *logrus.Entry
	govAPIBase string // overridable for tests
}

// NewPublicHolidayService creates a new PublicHolidayService.
func NewPublicHolidayService(repo repo.PublicHolidayRepository, logger *logrus.Entry) *PublicHolidayService {
	return &PublicHolidayService{
		repo:       repo,
		client:     &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
		govAPIBase: govAPIBase,
	}
}

// SetGovAPIBase overrides the government API base URL. Intended for tests only.
func (s *PublicHolidayService) SetGovAPIBase(base string) {
	s.govAPIBase = base
}

// IsHoliday returns true if date is a public holiday in the given zone.
// It ensures the year is synced from the API before checking.
func (s *PublicHolidayService) IsHoliday(ctx context.Context, date time.Time, zone string) (bool, string, error) {
	if zone == "" {
		zone = DefaultZone
	}
	// Ensure this year's holidays are in the DB.
	if err := s.EnsureYear(ctx, date.Year(), zone); err != nil {
		// Non-fatal: log and continue — blocking on API failure would break scheduling.
		s.logger.WithError(err).Warnf("could not sync public holidays for %d/%s — proceeding without check", date.Year(), zone)
		return false, "", nil
	}
	h, err := s.repo.GetByDate(ctx, date, zone)
	if err != nil {
		return false, "", err
	}
	if h == nil {
		return false, "", nil
	}
	return true, h.Name, nil
}

// EnsureYear fetches holidays for the given year/zone from the government API
// and upserts them into the database if they are not already present.
func (s *PublicHolidayService) EnsureYear(ctx context.Context, year int, zone string) error {
	existing, err := s.repo.ListByYear(ctx, year, zone)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		// Already cached for this year.
		return nil
	}
	return s.fetchAndStore(ctx, year, zone)
}

// ListByYear returns all public holidays for a year, fetching from the API if needed.
func (s *PublicHolidayService) ListByYear(ctx context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
	if zone == "" {
		zone = DefaultZone
	}
	if err := s.EnsureYear(ctx, year, zone); err != nil {
		s.logger.WithError(err).Warnf("could not sync public holidays for %d/%s", year, zone)
	}
	return s.repo.ListByYear(ctx, year, zone)
}

// fetchAndStore calls the French government API and persists the result.
// URL format:  GET {base}/{zone}/{year}.json
// Response:    {"YYYY-MM-DD": "Nom du jour", ...}  — names already in French.
func (s *PublicHolidayService) fetchAndStore(ctx context.Context, year int, zone string) error {
	url := fmt.Sprintf("%s/%s/%d.json", s.govAPIBase, zone, year)
	s.logger.WithField("url", url).Info("fetching public holidays from government API")

	// Resilient fetch: retry transient failures (5xx/429/network) with backoff
	// and cap the response body (OBS-4). On exhaustion the error is returned so
	// the caller degrades gracefully (holiday checks fall back to "not a holiday").
	body, err := httpx.GetWithRetry(ctx, s.client, url,
		map[string]string{"Accept": "application/json"}, httpx.Options{})
	if err != nil {
		return fmt.Errorf("holiday API request failed: %w", err)
	}

	// The government API returns a flat map: {"YYYY-MM-DD": "Nom du jour"}.
	// Names are already in French — no translation step needed.
	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed to decode holiday API response: %w", err)
	}

	holidays := make([]*model.PublicHoliday, 0, len(raw))
	for dateStr, name := range raw {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			s.logger.Warnf("skipping invalid date from API: %q", dateStr)
			continue
		}
		holidays = append(holidays, &model.PublicHoliday{
			Date: d,
			Zone: zone,
			Name: name,
		})
	}

	if err := s.repo.UpsertBatch(ctx, holidays); err != nil {
		return fmt.Errorf("failed to store holidays: %w", err)
	}

	s.logger.WithField("count", len(holidays)).Infof("stored public holidays for %d/%s", year, zone)
	return nil
}
