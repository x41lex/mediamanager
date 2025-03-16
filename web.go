package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mediamanager/filedb"
	"mediamanager/web1"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
)

// Web arguments
type WebArgs struct {
	DatabasePath string `arg:"positional,required" help:"Path to the database."`
	ApiVersion   int    `arg:"--apiversion" help:"Api version to use. Version 1 is stable, 2 is experimental" default:"1"`
	Address      string `arg:"-a,--address" help:"Address to host on as IP:PORT, default is <Local IP>:5555"`
	LiveUpdate   bool   `arg:"--live" help:"Update files live for development purpose"`
	DisableAuth  bool   `arg:"--noauth" help:"Disable HTTP authentication"`
	AuthConfig   string `arg:"-c,--authconfig" help:"Authentication config file, a JSON dictonary of <Username>:{Password: <Password>}. Must be provided unelss --noauth is used"`
	TlsCert      string `arg:"--cert" help:"Certificate file path, to use TLS this and --key must be used."`
	TlsKey       string `arg:"--key" help:"Key file path, to use TLS this and --cert must be used."`
}

func (w *WebArgs) Verify() error {
	if w.TlsCert == "" && w.TlsKey != "" {
		return errors.New("--key must be used with --cert")
	}
	if w.TlsCert != "" && w.TlsKey == "" {
		return errors.New("--cert must be used with --key")
	}
	if w.ApiVersion <= 0 || w.ApiVersion > 2 {
		return errors.New("unsupported --apversion value")
	}
	if !w.DisableAuth && w.AuthConfig == "" {
		return errors.New("--authconfig must be providede unless --noauth is used")
	}
	return nil
}

// Config for the account
type AccountConfigValue struct {
	Password string
}

// Parse and execute web
func ParseWeb(a *ArgList, p *arg.Parser) {
	// Verify arguments are correct
	if isPortable && a.Web.LiveUpdate {
		p.FailSubcommand("--live cannot be used on a portable install", "web")
		return
	}
	if a.Web.TlsCert != "" && a.Web.TlsKey == "" {
		p.FailSubcommand("--cert must be used with --key", "web")
		return
	}
	if a.Web.TlsCert == "" && a.Web.TlsKey != "" {
		p.FailSubcommand("--key must be used with --cert", "web")
		return
	}
	// If auth is enabled & config is not provided.
	if !a.Web.DisableAuth && a.Web.AuthConfig == "" {
		p.FailSubcommand("--authconfig must provided unless --noauth is provided", "web")
		return
	}
	if a.Web.ApiVersion != 1 && a.Web.ApiVersion != 2 {
		p.FailSubcommand("--apiversion must be either 1 or 2", "web")
		return
	}
	// Get address if needed
	if a.Web.Address == "" {
		// Get local address
		s, err := net.Dial("udp", "1.1.1.1:1")
		if err != nil {
			p.FailSubcommand(fmt.Sprintf("Failed to get local IP, set one using -a/--address (Error: %s)", err))
			return
		}
		a.Web.Address = strings.Split(s.LocalAddr().String(), ":")[0] + ":5555"
		s.Close()
	}
	accounts := make(map[string]*AccountConfigValue)
	// Load accounts (If we are doing that)
	if !a.Web.DisableAuth {
		data, err := os.ReadFile(a.Web.AuthConfig)
		if err != nil {
			fmt.Printf("Failed to open -c/--authconfig: %v\n", err)
			return
		}
		err = json.Unmarshal(data, &accounts)
		if err != nil {
			fmt.Printf("Failed to parse JSON data of --authconfig: %v\n", err)
			return
		}
		if len(accounts) == 0 {
			fmt.Printf("-c/--authconfig requires at least one account provided\n")
			return
		}
	}
	// Load database
	db, err := filedb.NewFileDb(a.Web.DatabasePath)
	if err != nil {
		fmt.Printf("Failed to create file database: %v\n", err)
		return
	}
	defer db.Close()
	if db.IsSafeMode() {
		meta, err := db.GetMetadata()
		if err != nil {
			fmt.Printf("Can't get database version: %v\n", err)
			return
		}
		fmt.Printf("Legacy databases cannot be used, this database is version %s, the current version is %s, the minimum supported version is %d.XrX\n  Use '%s database %s --migrate to update'\n",
			meta.VersionString(), filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision), filedb.MajorVersion, os.Args[0], a.Web.DatabasePath)
		return
	}
	switch a.Web.ApiVersion {
	case 1:
		var lm *web1.LoginManager
		// Get API
		mux := http.NewServeMux()
		var handler http.Handler = mux
		if !a.Web.DisableAuth {
			// Setup authentication
			lm = web1.NewLoginManager(mux, a.Web.LiveUpdate)
			for k, v := range accounts {
				err = lm.AddAccount(k, v.Password)
				if err != nil {
					fmt.Printf("Failed to add account '%s': %v\n", k, err)
					return
				}
			}
			handler = lm.HttpHandler(handler)
		}
		// Load API
		api := web1.NewFileDbApi(db, mux, lm)
		api.InitApp(mux, a.Web.LiveUpdate)
		// Add the listen logger
		handler = web1.ListenerLogger(handler)
		if a.Web.TlsCert != "" || a.Web.TlsKey != "" {
			fmt.Printf("Hosting on https://%s\n", a.Web.Address)
			err := http.ListenAndServeTLS(a.Web.Address, a.Web.TlsCert, a.Web.TlsKey, handler)
			if err != nil {
				slog.Error("Failed to server TLS", "Error", err.Error(), "Address", a.Web.Address, "CertFilePath", a.Web.TlsCert, "KeyFilePath", a.Web.TlsKey)
				fmt.Printf("Failed: %v\n", err)
			}
		} else {
			fmt.Printf("Hosting on http://%s\n", a.Web.Address)
			err := http.ListenAndServe(a.Web.Address, handler)
			if err != nil {
				slog.Error("Failed to server TLS", "Error", err.Error(), "Address", a.Web.Address)
				fmt.Printf("Failed: %v\n", err)
			}
		}
	default:
		panic(fmt.Sprintf("MediaManager: Version '%d' was allowed through argument check, but not implemented.", a.Web.ApiVersion))
	}
}
