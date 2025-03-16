package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"mediamanager/filedb"
	"os"
	"path/filepath"

	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"
)

type ImportArgs struct {
	DatabasePath string   `arg:"positional,required" help:"Path to the database."`
	ImportFiles  []string `arg:"-f,--importfiles,separate" help:"Import files"`
	ImportDirs   []string `arg:"-d,--importdirs,separate" help:"Import directories"`
	AddTags      []string `arg:"-t,--tag,separate" help:"Add tags to every import"`
	AddHashes    bool     `arg:"-H,--addhash" help:"Add hashes to file"`
	AddSizes     bool     `arg:"-S,--addsizes" help:"Add sizes to files"`
	SetStars     int      `arg:"-s,--stars" help:"Number of stars to set on imports"`
	SetDate      bool     `arg:"-l,--setlastviewed" help:"Set last view date to right now"`
	Silent       bool     `arg:"--silent" help:"Don't log errors on import"`

	ImportJson string `arg:"--importjson" help:"Deprecated: Import from a JSON config, see README.md for format. Cannot co exist with ImportDirs or ImportFiles. Ignore all other values."`
}

type JsonEntry struct {
	Tags []string
}

type JsonImport struct {
	AddFileInfo         bool     // Add hash & sizes to files
	Extensions          []string // By default extensions added are .jpg, .png, .jpeg, .gif, .mp3, .flac, .wav, .webm, .mp4, .mov, .m4v
	NoDefaultExtensions bool
	Dirs                map[string]JsonEntry
	Files               map[string]JsonEntry
}

func ptermProgressBar(total int, ch <-chan int64, ctx context.Context) {
	pBar, err := pterm.DefaultProgressbar.WithCurrent(0).WithShowElapsedTime(true).WithShowCount(true).WithShowPercentage(true).WithTotal(total).Start("Adding file info")
	if err != nil {
		panic(fmt.Sprintf("MediaManager: pterm.DefaultProgressbar: Failed to start progress bar: %v", err))
	}
	defer pBar.Stop()
	for {
		select {
		case c := <-ch:
			if c == -1 {
				return
			}
			// ... This probably will never be a issue, unless your importing 2,147,483,647 files, which is fucking wild.
			pBar.Current = int(c)
		case <-ctx.Done():
			return
		}
	}
}

func importJson(db *filedb.FileDb, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read JSON data\n")
		return
	}
	im := &JsonImport{}
	err = json.Unmarshal(data, im)
	if err != nil {
		fmt.Printf("Failed to unmarshal JSON data\n")
		return
	}
	importList := make([]*filedb.File, 0)
	for path, meta := range im.Files {
		f := filedb.NewFile(path)
		for _, t := range meta.Tags {
			err = f.AddTag(t)
			if err != nil {
				fmt.Printf("! Failed to add tag '%s' to file '%s': %v\n", t, path, err)
				return
			}
		}
		importList = append(importList, f)
	}
	exitNow := false
	for path, meta := range im.Dirs {
		filepath.WalkDir(path, func(path string, d fs.DirEntry, _ error) error {
			if d == nil {
				slog.Warn("DirEntry was nil?", "Path", path)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			// Check if we want this file imported
			if !isImportableFile(path, im.Extensions...) {
				slog.Debug("Not importing non media", "Path", path)
				return nil
			}
			f := filedb.NewFile(path)
			for _, t := range meta.Tags {
				err = f.AddTag(t)
				if err != nil {
					fmt.Printf("! Failed to add tag '%s' to file '%s': %v\n", t, path, err)
					exitNow = true
					return filepath.SkipAll
				}
			}
			importList = append(importList, f)
			return nil
		})
	}
	if exitNow {
		return
	}
	fmt.Printf("Importing %d files\n", len(importList))
	if im.AddFileInfo {
		fmt.Printf("Adding file info\n")
		ctx, can := context.WithCancel(context.Background())
		defer can()
		channel := make(chan int64, 10)
		go ptermProgressBar(len(importList), channel, ctx)
		err = filedb.AddInfoToFiles(&filedb.AddInfoOpts{
			DontAddHash:  false,
			DontAddSize:  false,
			ProgressChan: channel,
			Context:      ctx,
		}, importList...)
		if err != nil {
			fmt.Printf("Failed to add file info: %v\n", err)
			return
		}
	}
	failed, err := db.AddFiles(importList...)
	if err != nil {
		fmt.Printf("Failed to add files: %v\n", err)
		return
	}
	imported := len(importList)
	for _, f := range failed {
		fmt.Printf("Failed to add file '%s': %v\n", f.File.GetPath(), f.Error)
		fmt.Printf("  | Hash: %s\n", f.File.GetHash())
		imported--
	}
	fmt.Printf("Imported %d files\n", imported)
}

// Parse import arguments
func ParseImport(a *ArgList, p *arg.Parser) {
	db, err := filedb.NewFileDb(a.Import.DatabasePath)
	if err != nil {
		fmt.Printf("Failed to create file database: %v\n", err)
		return
	}
	defer db.Close()
	toImport := make([]*filedb.File, 0)
	if a.Import.ImportJson != "" {
		importJson(db, a.Import.ImportJson)
		return
	}
	for _, v := range a.Import.ImportFiles {
		f := filedb.NewFile(v)
		toImport = append(toImport, f)
	}
	for _, d := range a.Import.ImportDirs {
		filepath.WalkDir(d, func(path string, d fs.DirEntry, _ error) error {
			if d == nil {
				slog.Warn("DirEntry was nil?", "Path", path)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			// Check if we want this file imported
			if !isImportableFile(path) {
				slog.Debug("Not importing non media", "  bPath", path)
				return nil
			}
			f := filedb.NewFile(path)
			toImport = append(toImport, f)
			return nil
		})
	}
	for _, v := range toImport {
		for _, t := range a.Import.AddTags {
			err = v.AddTag(t)
			if err != nil {
				fmt.Printf("Failed to add tag '%s': %v\n", t, err)
				return
			}
		}
		if a.Import.SetStars != 0 {
			err = v.SetStars(uint8(a.Import.SetStars))
			if err != nil {
				fmt.Printf("Failed to set stars '%d': %v\n", a.Import.SetStars, err)
				return
			}
		}
		if a.Import.SetDate {
			v.MarkFileRead()
		}
	}
	ctx, can := context.WithCancel(context.Background())
	if a.Import.AddHashes || a.Import.AddSizes {
		channel := make(chan int64, 10)
		go ptermProgressBar(len(toImport), channel, ctx)
		err = filedb.AddInfoToFiles(&filedb.AddInfoOpts{
			DontAddHash:  !a.Import.AddHashes,
			DontAddSize:  !a.Import.AddSizes,
			ProgressChan: channel,
			Context:      ctx,
		}, toImport...)
		if err != nil {
			can()
			fmt.Printf("Failed to add file info: %v\n", err)
			return
		}
	}
	can()
	failed, err := db.AddFiles(toImport...)
	if err != nil {
		fmt.Printf("Failed to add files: %v\n", err)
		return
	}
	imported := len(toImport)
	for _, f := range failed {
		if !a.Import.Silent {
			fmt.Printf("Failed to add file '%s': %v\n", f.File.GetPath(), f.Error)
			fmt.Printf("  | Hash: %s\n", f.File.GetHash())
		}
		imported--
	}
	fmt.Printf("Imported %d files\n", imported)
}
