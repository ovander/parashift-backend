package handler

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/bff"
	"github.com/ovander/backendkit/socrate"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
	"github.com/sirupsen/logrus"
)

// bffScopes are the scopes the BFF asks Socrate for.
const bffScopes = "openid email profile api"

// maxPendingLogins caps the sign-ins started but not finished, so /bff/login
// cannot be used to fill memory. Each lives bff.DefaultLoginBindingTTL.
const maxPendingLogins = 10000

// BFFHandler serves the Backend-for-Frontend routes under /bff. Parashift is the
// confidential OAuth client: it runs the authorization-code flow with PKCE on
// the server, keeps the tokens in a server-side session (bff.Gateway's store)
// and gives the browser only an opaque, HttpOnly session cookie and a CSRF
// token. No OAuth token ever reaches the browser.
type BFFHandler struct {
	gw           *bff.Gateway
	binding      bff.LoginBinding
	pending      *pendingLogins
	auth         *service.SessionAuthService
	authorizeURL string // Socrate's /oauth/authorize, a browser navigation
	clientID     string
	redirectURI  string
	appOrigin    string // scheme://host of redirectURI: the SPA's origin
	logger       *logrus.Entry
	now          func() time.Time
}

// BFFOptions configures NewBFFHandler.
type BFFOptions struct {
	Gateway     *bff.Gateway
	Auth        *service.SessionAuthService
	Issuer      string // Socrate's public URL, no trailing slash
	ClientID    string
	RedirectURI string // the registered /bff/callback URL; empty disables sign-in
	Logger      *logrus.Entry
	Now         func() time.Time // overrides time.Now (tests)
}

// NewBFFHandler creates a BFFHandler. The login-binding cookie follows the
// session cookie's Secure setting and is named after it ("<name>_login").
func NewBFFHandler(o BFFOptions) *BFFHandler {
	origin := ""
	if u, err := url.Parse(o.RedirectURI); err == nil && u.Scheme != "" && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &BFFHandler{
		gw: o.Gateway,
		binding: bff.LoginBinding{
			Cookie: bff.CookieConfig{Name: o.Gateway.Cookie.Name + "_login", Secure: o.Gateway.Cookie.Secure},
			TTL:    bff.DefaultLoginBindingTTL,
		},
		pending:      newPendingLogins(maxPendingLogins),
		auth:         o.Auth,
		authorizeURL: strings.TrimRight(o.Issuer, "/") + "/oauth/authorize",
		clientID:     o.ClientID,
		redirectURI:  o.RedirectURI,
		appOrigin:    origin,
		logger:       o.Logger,
		now:          now,
	}
}

func (h *BFFHandler) enabled() bool { return h.redirectURI != "" && h.appOrigin != "" }

// noStore marks a BFF response as never cacheable: it sets or reads the
// session, or carries the CSRF token.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

// Login handles GET /bff/login?return_to=. It starts the authorization-code
// flow: a PKCE S256 pair and a single-use state kept on the server, a
// login-binding cookie tying the flow to this browser, then a redirect to
// Socrate's /oauth/authorize.
func (h *BFFHandler) Login(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.enabled() {
		apierror.ServiceUnavailable("sign-in is not configured").WithKey("errors.unknown").WriteJSON(w)
		return
	}
	pkce := bff.NewPKCE()
	state := bff.RandomToken(32)
	nonce := h.binding.Begin(w)
	if !h.pending.put(state, pendingLogin{
		verifier: pkce.Verifier,
		nonce:    nonce,
		returnTo: sanitizeReturnTo(r.URL.Query().Get("return_to")),
		expires:  h.now().Add(bff.DefaultLoginBindingTTL),
	}, h.now()) {
		h.logger.Warn("bff: too many sign-ins in progress; refusing a new one")
		apierror.ServiceUnavailable("too many sign-ins in progress, try again shortly").WithKey("errors.unknown").WriteJSON(w)
		return
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {h.clientID},
		"redirect_uri":          {h.redirectURI},
		"scope":                 {bffScopes},
		"state":                 {state},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, h.authorizeURL+"?"+q.Encode(), http.StatusFound)
}

// Callback handles GET /bff/callback, where Socrate sends the browser back.
// The state must be one this server issued, unexpired and unused, and the
// browser must present the login-binding cookie issued with it; then the code
// is exchanged with the PKCE verifier, the profile read, and a session created.
// Any failure sends the browser to /login?error=… without a session.
func (h *BFFHandler) Callback(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.enabled() {
		apierror.ServiceUnavailable("sign-in is not configured").WithKey("errors.unknown").WriteJSON(w)
		return
	}
	q := r.URL.Query()
	// Take the state first: it is spent whatever happens next.
	pl, ok := h.pending.take(q.Get("state"), h.now())
	if !ok {
		h.binding.Verify(w, r, "") // clears the binding cookie
		h.failSignIn(w, r, "unknown, used or expired state", "sign_in_failed")
		return
	}
	if !h.binding.Verify(w, r, pl.nonce) {
		h.failSignIn(w, r, "login-binding cookie missing or foreign", "sign_in_failed")
		return
	}
	if e := q.Get("error"); e != "" {
		reason := "sign_in_failed"
		if e == "access_denied" {
			reason = "access_denied"
		}
		h.failSignIn(w, r, "Socrate returned error "+e, reason)
		return
	}
	code := q.Get("code")
	if code == "" {
		h.failSignIn(w, r, "no code", "sign_in_failed")
		return
	}
	ts, id, err := h.auth.SignIn(r.Context(), code, h.redirectURI, pl.verifier)
	if err != nil {
		h.failSignIn(w, r, "code exchange failed: "+err.Error(), "sign_in_failed")
		return
	}
	h.startSession(w, r, ts, id)
	http.Redirect(w, r, pl.returnTo, http.StatusFound)
}

// failSignIn logs why a sign-in was refused and sends the browser to the
// sign-in page with a generic reason, never the cause.
func (h *BFFHandler) failSignIn(w http.ResponseWriter, r *http.Request, cause, reason string) {
	h.logger.WithField("cause", cause).Warn("bff: sign-in refused")
	http.Redirect(w, r, "/login?error="+url.QueryEscape(reason), http.StatusFound)
}

// startSession stores a new session for the token set and sets its cookie.
// A session the browser already had is dropped: signing in always starts a
// fresh session id (no session fixation).
func (h *BFFHandler) startSession(w http.ResponseWriter, r *http.Request, ts *socrate.TokenSet, id service.SessionIdentity) *bff.Session {
	if old, ok := h.gw.Cookie.SessionID(r); ok {
		h.gw.Store.Delete(old)
	}
	s := bff.NewSession(bff.RandomToken(32), bff.RandomToken(32), ts,
		bff.UserInfo{Sub: id.Sub, Email: id.Email, Name: id.Name, Roles: []string{}}, h.now())
	h.gw.Store.Put(s)
	h.gw.Cookie.SetSession(w, s.ID())
	return s
}

// sessionUser is the identity /bff/session shows the SPA. The SPA reads its
// Parashift position and store from GET /api/v1/me, as before.
type sessionUser struct {
	Sub   string `json:"sub"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// SessionResponse is the body of GET /bff/session.
// It never carries a token; csrf is the value the SPA sends back in
// X-CSRF-Token on every POST, PUT, PATCH and DELETE.
type SessionResponse struct {
	Authenticated bool         `json:"authenticated"`
	User          *sessionUser `json:"user,omitempty"`
	CSRF          string       `json:"csrf,omitempty"`
}

func sessionResponse(s *bff.Session) SessionResponse {
	u := s.User()
	return SessionResponse{
		Authenticated: true,
		User:          &sessionUser{Sub: u.Sub, Email: u.Email, Name: u.Name},
		CSRF:          s.CSRF(),
	}
}

// Session handles GET /bff/session: {"authenticated":false}, or the user and
// the CSRF token. A session whose refresh token Socrate has rejected is ended
// here, so the SPA does not believe in a dead session.
func (h *BFFHandler) Session(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	s, ok := h.gw.SessionFromRequest(r)
	if !ok {
		if _, had := h.gw.Cookie.SessionID(r); had {
			h.gw.Cookie.ClearSession(w)
		}
		pkg.WriteJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}
	if _, err := h.gw.EnsureFresh(r.Context(), s); err != nil && bff.IsFatalRefreshError(err) {
		h.gw.Store.Delete(s.ID())
		h.gw.Cookie.ClearSession(w)
		pkg.WriteJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}
	s.Touch(h.now())
	pkg.WriteJSON(w, http.StatusOK, sessionResponse(s))
}

// Logout handles POST /bff/logout. With a session it needs the CSRF token,
// revokes the refresh token at Socrate (Socrate has no end_session endpoint),
// deletes the session and clears the cookie. The session is dropped even when
// the revocation fails. Without a session it only clears the cookie.
func (h *BFFHandler) Logout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	s, ok := h.gw.SessionFromRequest(r)
	if !ok {
		h.gw.Cookie.ClearSession(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !h.gw.CheckCSRF(r, s) {
		apierror.Forbidden("missing or invalid CSRF token").WithKey("errors.accessDenied").WriteJSON(w)
		return
	}
	if err := h.auth.SignOut(r.Context(), s.RefreshToken()); err != nil {
		h.logger.WithError(err).WithField("sub", s.User().Sub).Warn("bff: refresh-token revocation failed; session dropped anyway")
	}
	h.gw.Store.Delete(s.ID())
	h.gw.Cookie.ClearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeReturnTo keeps a same-origin path (bff.SanitizeReturnTo) and never
// sends the browser back into the BFF routes themselves.
func sanitizeReturnTo(p string) string {
	p = bff.SanitizeReturnTo(p)
	if p == "/bff" || strings.HasPrefix(p, "/bff/") || strings.HasPrefix(p, "/bff?") {
		return "/"
	}
	return p
}

// pendingLogin is a sign-in started at /bff/login and not yet finished.
type pendingLogin struct {
	verifier string
	nonce    string
	returnTo string
	expires  time.Time
}

// pendingLogins keeps the states issued by /bff/login in memory. A state is
// single-use (take deletes it) and expires after bff.DefaultLoginBindingTTL.
type pendingLogins struct {
	mu  sync.Mutex
	m   map[string]pendingLogin
	max int
}

func newPendingLogins(max int) *pendingLogins {
	return &pendingLogins{m: make(map[string]pendingLogin), max: max}
}

// put records a state. When full it drops expired entries first, and refuses
// the new one if that frees nothing.
func (p *pendingLogins) put(state string, pl pendingLogin, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.m) >= p.max {
		for k, v := range p.m {
			if !now.Before(v.expires) {
				delete(p.m, k)
			}
		}
		if len(p.m) >= p.max {
			return false
		}
	}
	p.m[state] = pl
	return true
}

// take returns and deletes the pending sign-in for state, if it is still valid.
func (p *pendingLogins) take(state string, now time.Time) (pendingLogin, bool) {
	if state == "" {
		return pendingLogin{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	pl, ok := p.m[state]
	if !ok {
		return pendingLogin{}, false
	}
	delete(p.m, state)
	if !now.Before(pl.expires) {
		return pendingLogin{}, false
	}
	return pl, true
}

// Sweep drops expired pending sign-ins; the session-store ticker calls it.
func (h *BFFHandler) Sweep() {
	now := h.now()
	h.pending.mu.Lock()
	for k, v := range h.pending.m {
		if !now.Before(v.expires) {
			delete(h.pending.m, k)
		}
	}
	h.pending.mu.Unlock()
}
