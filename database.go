package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mediamanager/filedb"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

// Database operation
type DatabaseArgs struct {
	DatabasePath string `arg:"positional,required" help:"Path to the database."`
	Backup       bool   `arg:"-B,--backup" help:"Action. Backup the database to <Datbasepath>.bak"`
	NoBackup     bool   `arg:"--nobackup" help:"disables database backup during operations"`
	Dry          bool   `arg:"--dry" help:"A select & Action must be provided. don't modify the database during this operation and log results based on --dryout"`
	DryOutput    string `arg:"--dryoutput" help:"Output during dry operation. Can be stdout, log, (PATH).txt or (PATH).json" default:"stdout"`
	// * SELECTION METHODS *
	SelectId        []int    `arg:"-i,--selectid,separate" help:"select file ids, cannot coexist with any other selects"`
	SelectTags      []string `arg:"-t,--selecttag,separate" help:"select by tags"`
	SelectPath      string   `arg:"-p,--selectpath" help:"file paths to select"`
	SelectStars     []uint8  `arg:"-s,--selectstars,separate" help:"select stars, each value must be 0 to 5"`
	SelectDateUnset bool     `arg:"--selectdateunset" help:"select date values that aren't set"`
	SelectMissing   bool     `arg:"--selectmissing" help:"select files missing from disk"`
	SelectNoHash    bool     `arg:"--selectnohash" help:"select files with no hash"`
	SelectNoSize    bool     `arg:"--selectnosize" help:"select files with no size"`
	// ** ACTIONS **
	DisplayFiles bool   `arg:"-d,--display" help:"Action. Display files selected"`
	WriteJson    string `arg:"-j,--json" help:"Output files as a JSON array"`
	// * MODIFY *
	AddTag    []string `arg:"--tag,separate" help:"Action. tags to add to selected entries"`
	SetStars  int      `arg:"--stars" help:"Action. stars to set on selected file" default:"-1"`
	RemoveTag []string `arg:"--removetag,separate" help:"Action. remove a tag from selected files"`
	// * TAGS *
	RemoveTagFromDb []string `arg:"--deletetag,separate" help:"Cannot be used with a select or other action. remove tag from database and all files"`
	// * REMOVAL *
	Remove         bool `arg:"--remove" help:"Action. remove selected files from database. Overrides --addtag, --stars, --rmtag, --updatehash and --updatesize."`
	RemoveFromDisk bool `arg:"--removefromdisk" help:"Action. Remove selected files from disk. Overrides --addtag, --stars, --rmtag, --updatehash and --updatesize."`
	// * UPDATE *
	UpdateFileHash bool `arg:"-H,--updatehash" help:"Action. Update hashes of selected files"`
	UpdateFileSize bool `arg:"-S,--updatesize" help:"Action. Update sizes of selected files"`
	// * INFO *
	Metadata bool `arg:"--metadata" help:"Show all metadata"`
	// ** VERSION **
	Version bool `arg:"-v,--version" help:"Get database info and exit"`
	Update  bool `arg:"-U,--update" help:"Update to the latest version, if possible"`
}

// Verify database argument as valid
func (d *DatabaseArgs) Verify(p *arg.Parser) bool {
	// Things we can't combine
	if d.Backup && d.NoBackup {
		p.FailSubcommand("--backup and --nobackup cannot be combined", "database")
		return false
	}
	// Ensure select id is the only select used.
	if len(d.SelectId) > 0 && (len(d.SelectTags) > 0 || d.SelectPath != "" || len(d.SelectStars) > 0 || d.SelectDateUnset || d.SelectMissing || d.SelectNoHash || d.SelectNoSize) {
		p.FailSubcommand("--selectid cannot be combined with other select arguments", "database")
		return false
	}
	// Ensure we aren't selecting & removing tags from database.
	if d.HasSelect() && len(d.RemoveTagFromDb) > 0 {
		p.FailSubcommand("select argument and --deletetag cannot be used together", "database")
		return false
	}
	return true
}

// Has a Select argument
func (d *DatabaseArgs) HasSelect() bool {
	return len(d.SelectId) > 0 || len(d.SelectTags) > 0 || d.SelectPath != "" || len(d.SelectStars) > 0 || d.SelectDateUnset || d.SelectMissing || d.SelectNoHash || d.SelectNoSize
}

// Has a action argument
func (d *DatabaseArgs) HasAction() bool {
	return d.DisplayFiles || len(d.AddTag) > 0 || d.SetStars != -1 || len(d.RemoveTag) > 0 || d.Remove || d.RemoveFromDisk || d.UpdateFileHash || d.UpdateFileSize || len(d.RemoveTagFromDb) > 0 || d.WriteJson != ""
}

// Execute a database operation live
func DbLiveExecute(d *ArgList, db *filedb.FileDb, files []*filedb.File) {
	if len(files) == 0 {
		return
	}
	if !d.Database.Backup && !d.Database.NoBackup {
		// If d.Database.Backup is set we already backed up
		err := copyFile(d.Database.DatabasePath, fmt.Sprintf("%s.bak", d.Database.DatabasePath))
		if err != nil {
			fmt.Printf("Failed to backup database: %v\n", err)
			return
		}
	}
	for _, f := range files {
		if d.Database.Remove || d.Database.RemoveFromDisk {
			// Ignore other arguments.
			if d.Database.Remove {
				err := db.RemoveFile(f)
				if err != nil {
					fmt.Printf("Failed to remove file from database '%s' (%d): %v\n", f.GetPath(), f.GetId(), err)
				}
			}
			if d.Database.RemoveFromDisk {
				err := os.Remove(f.GetPath())
				if err != nil {
					fmt.Printf("Failed to remove file from disk '%s' (%d): %v\n", f.GetPath(), f.GetId(), err)
				}
			}
			continue
		}
		for _, t := range d.Database.AddTag {
			err := f.AddTag(t)
			if err != nil {
				fmt.Printf("Failed to add tag '%s' to file '%s': %v\n", t, f.GetPath(), err)
				return
			}
		}
		for _, r := range d.Database.RemoveTag {
			// If it doesn't have it who cares.
			f.RemoveTag(r)
		}
		if d.Database.SetStars != -1 {
			err := f.SetStars(uint8(d.Database.SetStars))
			if err != nil {
				fmt.Printf("Failed to set '%d' stars to file '%s': %v\n", d.Database.SetStars, f.GetPath(), err)
				return
			}
		}
	}
	if d.Database.Remove || d.Database.RemoveFromDisk {
		// Nothing else to do.
		return
	}
	// Add file hash & sizes, this might take a bit.
	if d.Database.UpdateFileHash || d.Database.UpdateFileSize {
		err := filedb.AddInfoToFiles(&filedb.AddInfoOpts{
			DontAddHash:       !d.Database.UpdateFileHash,
			DontAddSize:       !d.Database.UpdateFileSize,
			ProgressBarWriter: os.Stdout,
		}, files...)
		if err != nil {
			fmt.Printf("Failed to add info to files: %v\n", err)
			return
		}
	}
	// Update all the files
	for _, v := range files {
		err := db.UpdateFile(v)
		if err != nil {
			fmt.Printf("File '%s' failed to update: %v\n", v.GetPath(), err)
			fmt.Printf("  | Hash: %s\n", v.GetHash())
			// Update the remaning files anyway.
		}
	}
}

// Dry operation stuff
type DryOperation struct {
	Operation     string   // Operation being executed
	OperationArgs []string // Arguments to the operation
	TargetId      int      // ID of the target file, or -1 if none
	TargetPath    string   // Path of the target file, or "" if none
	Error         error    // Error that occoured or nil
}

// Get what options would do
func DbDryExecute(d *ArgList, files []*filedb.File) {
	ops := make([]*DryOperation, 0)
	for _, f := range files {
		if d.Database.Remove || d.Database.RemoveFromDisk {
			// Ignore other arguments.
			if d.Database.Remove {
				ops = append(ops, &DryOperation{
					Operation:     "db.RemoveFile",
					OperationArgs: []string{},
					TargetId:      f.GetId(),
					TargetPath:    f.GetPath(),
					Error:         nil,
				})
			}
			if d.Database.RemoveFromDisk {
				ops = append(ops, &DryOperation{
					Operation:     "os.Remove",
					OperationArgs: []string{},
					TargetId:      f.GetId(),
					TargetPath:    f.GetPath(),
					Error:         nil,
				})
			}
			continue
		}
		for _, t := range d.Database.AddTag {
			// We can still run this cause we never update it in the database.
			err := f.AddTag(t)
			ops = append(ops, &DryOperation{
				Operation:     "f.AddTag",
				OperationArgs: []string{t},
				TargetId:      f.GetId(),
				TargetPath:    f.GetPath(),
				Error:         err,
			})
		}
		for _, t := range d.Database.RemoveTag {
			// If it doesn't have it who cares.
			ops = append(ops, &DryOperation{
				Operation:     "f.RemoveTag",
				OperationArgs: []string{t},
				TargetId:      f.GetId(),
				TargetPath:    f.GetPath(),
				Error:         nil,
			})
		}
		if d.Database.SetStars != -1 {
			err := f.SetStars(uint8(d.Database.SetStars))
			ops = append(ops, &DryOperation{
				Operation:     "f.SetStars",
				OperationArgs: []string{fmt.Sprintf("%d", uint8(d.Database.SetStars))},
				TargetId:      f.GetId(),
				TargetPath:    f.GetPath(),
				Error:         err,
			})
		}
	}
	// We don't actuall run this cause it would take a long as time for *no* reason.
	if d.Database.UpdateFileHash || d.Database.UpdateFileSize {
		ops = append(ops, &DryOperation{
			Operation: "filedb.AddInfoToFiles",
			OperationArgs: []string{
				fmt.Sprintf("%+v", &filedb.AddInfoOpts{
					DontAddHash:       !d.Database.UpdateFileHash,
					DontAddSize:       !d.Database.UpdateFileSize,
					ProgressBarWriter: os.Stdout,
				}),
				fmt.Sprintf("{%d files}", len(files)),
			},
			TargetId:   -1,
			TargetPath: "",
			Error:      nil,
		})
	}
	// Updatea all the files
	for _, v := range files {
		ops = append(ops, &DryOperation{
			Operation:     "db.UpdateFile",
			OperationArgs: []string{},
			TargetId:      v.GetId(),
			TargetPath:    v.GetPath(),
			Error:         nil,
		})
	}
	// Write output
	var outputWriter io.Writer
	var outputCloser io.Closer
	if d.Database.DryOutput == "stdout" {
		outputWriter = os.Stdout
	} else if strings.HasSuffix(d.Database.DryOutput, ".txt") {
		file, err := os.OpenFile(d.Database.DryOutput, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			fmt.Printf("Failed to open output file: %v\n", err)
			return
		}
		outputWriter = file
		outputCloser = file
	} else {
		fmt.Printf("Invalid --dryoutput value, must either be 'stdout' or '<PATH>.txt'")
		return
	}
	for _, o := range ops {
		if o.TargetId == -1 {
			fmt.Fprintf(outputWriter, "Operation=%s, Args=%v, Err=%v\n", o.Operation, o.OperationArgs, o.Error)
		} else {
			fmt.Fprintf(outputWriter, "Id=%d, Path=%s, Operation=%s, Args=%v, Err=%v\n", o.TargetId, o.TargetPath, o.Operation, o.OperationArgs, o.Error)
		}
	}
	if outputCloser != nil {
		outputCloser.Close()
	}
}

// Select a database and get files
func DbSelect(d *ArgList, db *filedb.FileDb) ([]*filedb.File, error) {
	files := make([]*filedb.File, 0)
	if len(d.Database.SelectId) > 0 {
		for _, v := range d.Database.SelectId {
			f, err := db.GetFileById(v)
			if err != nil {
				return nil, fmt.Errorf("failed to get file with id '%d': %v", v, err)
			}
			files = append(files, f)
		}
	} else {
		// Search
		sFiles, err := db.SearchFile(&filedb.SearchQuery{
			Path:          d.Database.SelectPath,
			WhitelistTags: d.Database.SelectTags,
			Count:         -1,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to search: %v", err)
		}
		for _, v := range sFiles {
			// Stars
			if len(d.Database.SelectStars) > 0 {
				if !slices.Contains(d.Database.SelectStars, v.GetStars()) {
					continue
				}
			}
			// Unset date
			if d.Database.SelectDateUnset && !v.GetLastPlayTime().Equal(time.Time{}) {
				continue
			}
			// Missing from disk (This might take a second)
			if d.Database.SelectMissing {
				_, err := os.Stat(v.GetPath())
				if err != nil {
					if os.IsNotExist(err) {
						// File doesn't exist - select it.
					} else {
						return nil, fmt.Errorf("failed to stat file '%s' for unexpected reason: %v", v.GetPath(), err)
					}
				} else {
					continue
				}
			}
			// No hash
			if d.Database.SelectNoHash && v.GetHash() != "" {
				continue
			}
			// No size
			if d.Database.SelectNoSize && v.GetSize() != 0 {
				continue
			}
			files = append(files, v)
		}
	}
	return files, nil
}

type jsonFile struct {
	Id         int
	Tags       []string
	Path       string
	LastViewed string
	Stars      int
	Size       int64
	Hash       string
}

// Parse the database arguments
func ParseDatabase(d *ArgList, p *arg.Parser) {
	if !d.Database.Verify(p) {
		return
	}
	if d.Database.Backup {
		err := copyFile(d.Database.DatabasePath, fmt.Sprintf("%s.bak", d.Database.DatabasePath))
		if err != nil {
			fmt.Printf("Failed to backup database: %v\n", err)
			return
		}
	}
	if d.Database.HasSelect() {
		if !d.Database.HasAction() {
			p.FailSubcommand("a action argument is required with a select.", "database")
			return
		}
	} else if !d.Database.Version && !d.Database.Update && !d.Database.Metadata {
		if d.Database.Backup {
			return
		}
		p.FailSubcommand("--version, --metadata, --update, --backup or a select & action must be provided", "database")
		return
	}
	// Load database.
	db, err := filedb.NewFileDb(d.Database.DatabasePath)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		return
	}
	defer db.Close()
	if d.Database.Version {
		meta, err := db.GetMetadata()
		if err != nil {
			fmt.Printf("Failed to get version: %v\n", err)
			return
		}
		fmt.Printf("FileDb Version  : %s (%s)\n", filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision), filedb.VersionCodeName)
		fmt.Printf("Database Version: %s (%s)\n", filedb.FormatVersion(meta.MajorVersion, meta.MinorVersion, meta.RevisionVersion), meta.VersionCodeName)
		if meta.Experimental {
			fmt.Printf("  | Database is set to experimental. This is a early developer database\n")
			return
		}
		if meta.MajorVersion > filedb.MajorVersion {
			fmt.Printf("!! FileDb out of date for this database\n")
			return
		} else if meta.MajorVersion < filedb.MajorVersion {
			fmt.Printf("!! Legacy database, this database no longer works with filedb. try --update\n")
			return
		}
		if meta.MinorVersion != filedb.MinorVersion {
			fmt.Printf("! Database out of date, but is still supported. Use --update to apply updates\n")
		}
		if meta.RevisionVersion != filedb.Revision {
			fmt.Printf("? Database out of date, but is still supported. Use --update to apply bugfixes\n")
		}
		return
	}
	if d.Database.Metadata {
		meta, err := db.GetMetadata()
		if err != nil {
			fmt.Printf("Failed to get version: %v\n", err)
			return
		}
		pad := 0
		keys := make([]string, 0, len(meta.Map))
		for k := range meta.Map {
			if len(k) > pad {
				pad = len(k)
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%-*s: %v\n", pad, k, meta.Map[k])
		}
		return
	}
	if d.Database.Update {
		meta, err := db.GetMetadata()
		if err != nil {
			fmt.Printf("! Failed to get database version: %v\n", err)
			return
		}
		if meta.MajorVersion == filedb.MajorVersion && meta.MinorVersion == filedb.MinorVersion && meta.RevisionVersion == filedb.Revision {
			fmt.Printf("- No migration needed\n")
			return
		}
		fmt.Printf("* Attempting to update from %s (%s) to %s (%s)\n", meta.VersionString(), meta.VersionCodeName, filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision), filedb.VersionCodeName)
		err = filedb.DoMigration(db)
		if err != nil {
			fmt.Printf("! Failed to update: %v\n", err)
			return
		}
		fmt.Printf("+ OK\n")
		return
	}
	if len(d.Database.RemoveTagFromDb) > 0 {
		for _, t := range d.Database.RemoveTagFromDb {
			err = db.RemoveTag(t)
			if err != nil {
				fmt.Printf("Failed to remove tag '%s': %v", t, err)
				return
			}
			fmt.Printf("- Removed tag '%s'\n", t)
		}
		return
	}
	// Do the select
	files, err := DbSelect(d, db)
	if err != nil {
		fmt.Printf("Select failed: %v\n", err)
		return
	}
	fmt.Printf("Got %d files\n", len(files))
	if d.Database.DisplayFiles {
		totalSize := 0
		fmt.Printf("ID, Path, Stars, LastPlayDate, Tags, Size, Hash\n")
		for _, v := range files {
			fmt.Printf("%d, %s, %d, %s, %v, %d, %s\n", v.GetId(), v.GetPath(), v.GetStars(), v.GetLastPlayTime().Format(time.RFC3339), v.GetTags(), v.GetSize(), v.GetHash())
			totalSize += int(v.GetSize())
		}
		fmt.Printf("Total size is %s\n", bytesToString(float64(totalSize)))
		return
	} else if d.Database.WriteJson != "" {
		jFiles := make([]*jsonFile, len(files))
		for i, v := range files {
			jFiles[i] = &jsonFile{
				Id:         v.GetId(),
				Tags:       v.GetTags(),
				Path:       v.GetPath(),
				LastViewed: v.GetLastPlayTime().UTC().Format(time.RFC3339),
				Stars:      int(v.GetStars()),
				Size:       v.GetSize(),
				Hash:       v.GetHash(),
			}
		}
		jData, err := json.MarshalIndent(jFiles, "", "  ")
		if err != nil {
			fmt.Printf("! Failed to convert files to JSON? Report this issue: %v\n", err)
			return
		}
		err = os.WriteFile(d.Database.WriteJson, jData, 0666)
		if err != nil {
			fmt.Printf("! Failed to write to file '%s': %v\n", d.Database.WriteJson, err)
			return
		}
		fmt.Printf("+ Wrote %d files to '%s'\n", len(jFiles), d.Database.WriteJson)
	} else if d.Database.Dry {
		DbDryExecute(d, files)
	} else {
		DbLiveExecute(d, db, files)
	}
}
