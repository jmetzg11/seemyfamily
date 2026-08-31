package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

const (
	sessionCookie   = "session"
	sessionLifetime = 30 * 24 * time.Hour
)

var errInvalidSession = errors.New("invalid session cookie")

type contextKey string

const userContextKey = contextKey("user")

func (app *application) sign(payload string) string {
	mac := hmac.New(sha256.New, app.sessionSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (app *application) setSessionCookie(w http.ResponseWriter, userID int) {
	expiry := time.Now().Add(sessionLifetime)
	payload := strconv.Itoa(userID) + "." + strconv.FormatInt(expiry.Unix(), 10)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    payload + "." + app.sign(payload),
		Path:     "/",
		Expires:  expiry,
		MaxAge:   int(sessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   app.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (app *application) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   app.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (app *application) readSessionCookie(r *http.Request) (int, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return 0, errInvalidSession
	}

	i := strings.LastIndex(c.Value, ".")
	if i < 0 {
		return 0, errInvalidSession
	}

	payload, signature := c.Value[:i], c.Value[i+1:]

	if !hmac.Equal([]byte(signature), []byte(app.sign(payload))) {
		return 0, errInvalidSession
	}

	id, expiry, ok := strings.Cut(payload, ".")
	if !ok {
		return 0, errInvalidSession
	}

	seconds, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || time.Now().After(time.Unix(seconds, 0)) {
		return 0, errInvalidSession
	}

	userID, err := strconv.Atoi(id)
	if err != nil {
		return 0, errInvalidSession
	}

	return userID, nil
}

func userFromContext(r *http.Request) (models.User, bool) {
	user, ok := r.Context().Value(userContextKey).(models.User)
	return user, ok
}
