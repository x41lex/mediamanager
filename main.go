package main

import (
	"fmt"
	"log/slog"
	"mediamanager/filedb"
	"os"

	"github.com/alexflint/go-arg"
)

const (
	VersionString string = "1.4"
)

var (
	isPortable bool = false
)

type ArgList struct {
	// * GENERAL *
	Version bool `arg:"-v,--version" help:"Show version and exit"`
	// * LOGGING *
	LogLevel string `arg:"--loglevel" help:"log level to run as, must be 'error', 'warn', 'info' or 'debug'" default:"warn"`
	LogPath  string `arg:"--logpath" help:"Path to log to" default:"log"`
	LogType  string `arg:"--logtype" help:"log type, must be either 'text' or 'json'" default:"text"`

	Database *DatabaseArgs `arg:"subcommand:database" help:"Database management"`
	Web      *WebArgs      `arg:"subcommand:web" help:"Web API"`
	Import   *ImportArgs   `arg:"subcommand:import" help:"Import into database"`
}

func main() {
	args := &ArgList{}
	p := arg.MustParse(args)
	// Setup logging either way.
	logFile, err := os.OpenFile(args.LogPath, os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	var logger *slog.Logger
	logLevel := slog.LevelWarn
	switch args.LogLevel {
	case "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	case "info":
		logLevel = slog.LevelInfo
	case "debug":
		logLevel = slog.LevelDebug
	default:
		panic(fmt.Sprintf("MediaManager: args.LogLevel should have been verified, got unexpected value '%s'", args.LogLevel))
	}
	switch args.LogType {
	case "text":
		logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		}))
	case "json":
		logger = slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		}))
	default:
		panic(fmt.Sprintf("MediaManager: args.LogType should have been verified, got unexpected value '%s'", args.LogType))
	}
	slog.SetDefault(logger)
	if isPortable {
		slog.Debug("Portable release")
	}
	// Actually do actions
	switch {
	case args.Database != nil:
		// Allow version to show
		args.Database.Version = args.Version
		ParseDatabase(args, p)
	case args.Web != nil:
		ParseWeb(args, p)
	case args.Import != nil:
		ParseImport(args, p)
	case args.Version:
		fmt.Printf("MediaManager & FileDb by Alex Strueby\n")
		fmt.Printf("  MediaManager Version: %s", VersionString)
		if isPortable {
			fmt.Printf(" PORTABLE")
		}
		fmt.Printf("\n  FileDb Version: %s (%s)\n", filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision), filedb.VersionCodeName)
		return
	default:
		p.Fail("A subcommand must be selected")
		return
	}
}
