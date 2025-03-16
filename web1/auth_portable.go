//go:build portable
// +build portable

package web1

import (
	"fmt"
	"net/http"
	"sync"
)

func NewLoginManager(mux *http.ServeMux, live bool) *LoginManager {
	lm := &LoginManager{
		accountLock: sync.Mutex{},
		accounts:    make([]*account, 0),
		cookieLock:  sync.Mutex{},
		cookies:     make([]*cookie, 0),
	}
	var err error
	lm.loginPageData, err = webFs.ReadFile("frontend/login.html")
	if err != nil {
		panic(fmt.Sprintf("web1: NewLoginManager: Failed to read 'web1/frontend/login.html': %v", err))
	}
	mux.HandleFunc("/login", lm.loginPage)
	mux.HandleFunc("/authenticate", lm.authPage)
	return lm
}
