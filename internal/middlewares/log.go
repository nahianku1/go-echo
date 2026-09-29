package middlewares

import (
	"regexp"
	"strings"
	"time"

	"net-http/internal/utils"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
)

type RequestLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	Method     string    `gorm:"size:10;not null" json:"method"`
	Path       string    `gorm:"size:255;not null;index" json:"path"`
	StatusCode int       `gorm:"index" json:"status_code"`
	DurationMs int64     `json:"duration_ms"`
	IP         string    `gorm:"size:50" json:"ip"`
	OS         string    `gorm:"size:50" json:"os"`
	Browser    string    `gorm:"size:50" json:"browser"`
	UserAgent  string    `gorm:"type:text" json:"user_agent"`
	User       string    `gorm:"size:100;index" json:"user"`
}

var (
	reWindows = regexp.MustCompile(`Windows NT (\d+\.\d+)`)
	reMacOS   = regexp.MustCompile(`Mac OS X ([\d_]+)`)
	reIOS     = regexp.MustCompile(`(?:iPhone OS|CPU OS) ([\d_]+)`)
	reAndroid = regexp.MustCompile(`Android ([\d.]+)`)

	reEdge    = regexp.MustCompile(`(?:Edg|Edge)\/([\d.]+)`)
	reOpera   = regexp.MustCompile(`(?:OPR|Opera)\/([\d.]+)`)
	reSamsung = regexp.MustCompile(`SamsungBrowser\/([\d.]+)`)
	reChrome  = regexp.MustCompile(`(?:Chrome|CriOS)\/([\d.]+)`)
	reFirefox = regexp.MustCompile(`(?:Firefox|FxiOS)\/([\d.]+)`)
	reSafari  = regexp.MustCompile(`Version\/([\d.]+).*Safari`)
	rePostman = regexp.MustCompile(`PostmanRuntime\/([\d.]+)`)
	reCurl    = regexp.MustCompile(`curl\/([\d.]+)`)
)

// RequestLogger persists each HTTP request to the database without console printing.
//
// The store and the token verifier are injected rather than read from the
// database.DB and utils.VerifyToken globals: the middleware states exactly what
// it needs (somewhere to write a log row, and a way to attribute a request to a
// user), so it can run against a throwaway database and a fake verifier.
type RequestLogger struct {
	db     *gorm.DB
	tokens utils.TokenService
}

// NewRequestLogger injects the log store and the token service used to
// attribute a request to a user.
func NewRequestLogger(db *gorm.DB, tokens utils.TokenService) *RequestLogger {
	return &RequestLogger{db: db, tokens: tokens}
}

// Middleware returns the echo middleware.
func (l *RequestLogger) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			r := c.Request()

			// 1. Extract user info optionally (don't block the request if missing)
			user := "anonymous"
			var token string

			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else if cookie, err := c.Cookie("access_token"); err == nil {
				token = cookie.Value
			}

			if token != "" {
				if claims, err := l.tokens.VerifyAccessToken(token); err == nil && claims.Username != "" {
					user = claims.UserEmail
				} else if oauthClaims, err := l.tokens.VerifyOAuthToken(token); err == nil && oauthClaims.Provider_id != "" {
					user = oauthClaims.Provider_id
				}
			}

			// 2. Execute downstream handlers
			err := next(c)
			duration := time.Since(start)

			// 3. Asynchronously persist log directly to database
			if l.db != nil && r.URL.Path != "/favicon.ico" {

				ua := r.UserAgent()
				osName := parseOS(ua)
				browserName := parseBrowser(ua)

				// c.Response() is the http.ResponseWriter; the *echo.Response
				// wrapper carries the status actually written.
				statusCode := 200
				if resp, respErr := echo.UnwrapResponse(c.Response()); respErr == nil {
					statusCode = resp.Status
				}

				go func(logEntry RequestLog) {
					if err := l.db.Create(&logEntry).Error; err != nil {
						utils.NewLogger("LOGGER").Printf("failed to persist request log: %v", err)
					}
				}(RequestLog{
					Method:     r.Method,
					Path:       r.URL.Path,
					StatusCode: statusCode,
					DurationMs: duration.Milliseconds(),
					IP:         c.RealIP(),
					OS:         osName,
					Browser:    browserName,
					UserAgent:  ua,
					User:       user,
				})
			}

			return err
		}
	}
}

func parseBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge/"):
		if m := reEdge.FindStringSubmatch(ua); len(m) > 1 {
			return "Edge " + m[1]
		}
		return "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		if m := reOpera.FindStringSubmatch(ua); len(m) > 1 {
			return "Opera " + m[1]
		}
		return "Opera"
	case strings.Contains(ua, "SamsungBrowser"):
		if m := reSamsung.FindStringSubmatch(ua); len(m) > 1 {
			return "Samsung Internet " + m[1]
		}
		return "Samsung Internet"
	case strings.Contains(ua, "Firefox/") || strings.Contains(ua, "FxiOS/"):
		if m := reFirefox.FindStringSubmatch(ua); len(m) > 1 {
			return "Firefox " + m[1]
		}
		return "Firefox"
	case strings.Contains(ua, "Chrome/") || strings.Contains(ua, "CriOS/"):
		if m := reChrome.FindStringSubmatch(ua); len(m) > 1 {
			return "Chrome " + m[1]
		}
		return "Chrome"
	case strings.Contains(ua, "Safari") && strings.Contains(ua, "Version/"):
		if m := reSafari.FindStringSubmatch(ua); len(m) > 1 {
			return "Safari " + m[1]
		}
		return "Safari"
	case strings.Contains(ua, "PostmanRuntime"):
		if m := rePostman.FindStringSubmatch(ua); len(m) > 1 {
			return "Postman " + m[1]
		}
		return "Postman"
	case strings.Contains(ua, "curl"):
		if m := reCurl.FindStringSubmatch(ua); len(m) > 1 {
			return "curl " + m[1]
		}
		return "curl"
	default:
		return "Unknown"
	}
}

func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows"):
		if matches := reWindows.FindStringSubmatch(ua); len(matches) > 1 {

			switch matches[1] {
			case "10.0":

				return "Windows 10/11"
			case "6.3":
				return "Windows 8.1"
			case "6.2":
				return "Windows 8"
			case "6.1":
				return "Windows 7"
			case "6.0":
				return "Windows Vista"
			case "5.1", "5.2":
				return "Windows XP"
			default:
				return "Windows NT " + matches[1]
			}
		}
		return "Windows"
	case strings.Contains(ua, "iPhone"):
		if matches := reIOS.FindStringSubmatch(ua); len(matches) > 1 {
			return "iOS " + strings.ReplaceAll(matches[1], "_", ".")
		}
		return "iOS"
	case strings.Contains(ua, "iPad"):
		if matches := reIOS.FindStringSubmatch(ua); len(matches) > 1 {
			return "iPadOS " + strings.ReplaceAll(matches[1], "_", ".")
		}
		return "iPadOS"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X"):
		if matches := reMacOS.FindStringSubmatch(ua); len(matches) > 1 {
			return "macOS " + strings.ReplaceAll(matches[1], "_", ".")
		}
		return "macOS"
	case strings.Contains(ua, "Android"):
		if matches := reAndroid.FindStringSubmatch(ua); len(matches) > 1 {
			return "Android " + matches[1]
		}
		return "Android"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	case strings.Contains(ua, "Linux") || strings.Contains(ua, "X11"):
		return "Linux"
	case strings.Contains(ua, "PostmanRuntime"):
		return "Postman"
	case strings.Contains(ua, "curl"):
		return "curl"
	default:
		return "Unknown"
	}
}
