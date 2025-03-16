package web1

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type account struct {
	Username string
	Password string
}

type cookie struct {
	AccountName string
	Value       string
	At          time.Time
}

type LoginManager struct {
	accountLock   sync.Mutex
	accounts      []*account
	cookieLock    sync.Mutex
	cookies       []*cookie
	loginPageData []byte
}

func (l *LoginManager) CookieToAccount(cookie string) (string, error) {
	for _, c := range l.cookies {
		if c.Value == cookie {
			return c.AccountName, nil
		}
	}
	return "", errors.New("account not found")
}

func (l *LoginManager) authenticate(w http.ResponseWriter, r *http.Request) bool {
	slog.Debug("LoginManger.authenticate called", "Path", r.URL.Path)
	if r.URL.Path == "/authenticate" {
		slog.Debug("LoginManger.authenticate sending authPage", "Path", r.URL.Path)
		http.Redirect(w, r, fmt.Sprintf("/authenticate?redirect=%s", r.URL.Path), http.StatusTemporaryRedirect)
		return false
	}
	redirectTo := r.PostFormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/"
	}
	slog.Debug("Checking cookies")
	for _, c := range r.Cookies() {
		if c.Name == "filedb_account" {
			l.cookieLock.Lock()
			defer l.cookieLock.Unlock()
			for _, v := range l.cookies {
				if v.Value == c.Value {
					slog.Debug("Found cookie")
					return true
				}
			}
		}
	}
	slog.Debug("Sending to /login page")
	// Redirect to login
	http.Redirect(w, r, "/login?redirect="+r.URL.Path+"?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
	return false
}

func (l *LoginManager) addAccountCookie(w http.ResponseWriter, r *http.Request, account *account) bool {
	_ = r
	buffer := make([]byte, 128)
	_, err := rand.Read(buffer)
	if err != nil {
		slog.Error("Failed to read account cookie", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("random reader failed for cookie"))
		return false
	}
	cook := &cookie{
		AccountName: account.Username,
		At:          time.Now(),
	}
	cook.Value = base64.StdEncoding.EncodeToString(buffer)
	l.cookieLock.Lock()
	defer l.cookieLock.Unlock()
	l.cookies = append(l.cookies, cook)
	// Probably set more cookie values
	http.SetCookie(w, &http.Cookie{
		Name:   "filedb_account",
		Value:  cook.Value,
		Secure: true,
	})
	return true
}

func (l *LoginManager) authPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	user := r.PostFormValue("username")
	if user == "" {
		w.WriteHeader(401)
		w.Write([]byte("missing 'username' value"))
		return
	}
	password := r.PostFormValue("password")
	if password == "" {
		w.WriteHeader(401)
		w.Write([]byte("missing 'password' value"))
		return
	}
	redirect := r.PostFormValue("redirect_to")
	if redirect == "" {
		redirect = "/"
	}
	l.accountLock.Lock()
	defer l.accountLock.Unlock()
	for _, v := range l.accounts {
		if v.Username == user {
			if v.Password == password {
				// Create cookie
				if l.addAccountCookie(w, r, v) {
					r.Method = "GET"
					w.Write([]byte(fmt.Sprintf("<!DOCTYPE html><style>body {background-color: #282a36;color: #f8f8f2;}</style><html><body><a href=\"%s\">Redirecting in 1 second if not click here.</a>\n<meta http-equiv=\"Refresh\" content=\"1; url='%s'\" /></body></html>", redirect, redirect)))
					return
				}
				return
			}
			// Invalid password
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("invalid account"))
			return
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("invalid account"))
}

func (l *LoginManager) HttpHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This seems... fishy.
		if r.URL.EscapedPath() == "/login" || r.URL.EscapedPath() == "/authenticate" {
			next.ServeHTTP(w, r)
			return
		}
		if !l.authenticate(w, r) {
			// Don't forward
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *LoginManager) AddAccount(user string, password string) error {
	l.accountLock.Lock()
	defer l.accountLock.Unlock()
	for _, v := range l.accounts {
		if v.Username == user {
			return errors.New("account exists")
		}
	}
	l.accounts = append(l.accounts, &account{
		Username: user,
		Password: password,
	})
	return nil
}

func (l *LoginManager) loginPage(w http.ResponseWriter, r *http.Request) {
	for _, c := range r.Cookies() {
		if c.Name == "filedb_account" {
			l.cookieLock.Lock()
			defer l.cookieLock.Unlock()
			for _, v := range l.cookies {
				if v.Value == c.Value {
					slog.Debug("Found cookie")
					// Why are we still on the login page...?
					http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
					return
				}
			}
		}
	}
	w.Write(l.loginPageData)
}
