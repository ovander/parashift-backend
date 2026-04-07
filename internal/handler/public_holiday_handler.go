package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// PublicHolidayHandler exposes public holiday data to the frontend.
type PublicHolidayHandler struct {
	svc *service.PublicHolidayService
}

// NewPublicHolidayHandler creates a new PublicHolidayHandler.
func NewPublicHolidayHandler(svc *service.PublicHolidayService) *PublicHolidayHandler {
	return &PublicHolidayHandler{svc: svc}
}

// holidayResponse is the JSON shape returned to the frontend.
type holidayResponse struct {
	Date string `json:"date"` // YYYY-MM-DD
	Name string `json:"name"`
	Zone string `json:"zone"`
}

// List returns all public holidays for a given year and zone.
// Route: GET /public-holidays?year=YYYY&zone=BE
// zone defaults to "BE" (Belgium) if omitted.
func (h *PublicHolidayHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		yearStr = strconv.Itoa(time.Now().Year())
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		pkg.WriteError(w, apierror.BadRequest("year must be a valid 4-digit year (e.g. 2026)"))
		return
	}

	zone := r.URL.Query().Get("zone")
	if zone == "" {
		zone = service.DefaultZone
	}

	holidays, err := h.svc.ListByYear(ctx, year, zone)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	out := make([]holidayResponse, 0, len(holidays))
	for _, hol := range holidays {
		out = append(out, holidayResponse{
			Date: hol.Date.Format("2006-01-02"),
			Name: hol.Name,
			Zone: hol.Zone,
		})
	}

	pkg.WriteJSON(w, http.StatusOK, out)
}
