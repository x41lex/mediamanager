package filedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-sqlite3"
)

/*
File Table:
CREATE TABLE file (
  id INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE NOT NULL,
  path TEXT NOT NULL UNIQUE,
  lastViewed TEXT NOT NULL,
  stars INTEGER,
  size INTEGER
 )

Tag Table (File -> Tag ID):
CREATE TABLE tag (
  fileId INTEGER NOT NULL,
  tagNameId INTEGER NOT NULL
)

Tab Lookup Table (Tag ID -> Text):
CREATE TABLE tag_name (
  id INTEGER PRIMARY KEY UNIQUE NOT NULL,
  value TEXT NOT NULL UNIQUE
)
*/

var ErrOutdatedDatabase error = errors.New("outdated databases must be migrated to be accessed")

// Init the database drivers
func init() {
	// Setup regex extension
	slog.Debug("Creating 'sqlite3-re' driver")
	sql.Register("sqlite3-re", &sqlite3.SQLiteDriver{
		ConnectHook: func(sc *sqlite3.SQLiteConn) error {
			return sc.RegisterFunc("regexp", regexp.MatchString, true)
		},
	})
}

// Sorting methods
type SortMethod int

const (
	SortMethodNone       SortMethod = iota // Don't sort
	SortMethodStars                        // Sort by stars
	SortMethodSize                         // Sort by size
	SortMethodLastViewed                   // Sort by last viewed
	SortMethodId                           // Sort by ID (Similar to SortMethodNone, but explicit and can be used with ASC and DESC)
	SortMethodRandom                       // Get random files
)

// A new search query, leaves values unfilled and they wont be used
type SearchQuery struct {
	Path          string     // File path, search with a sql LIKE
	PathRe        string     // File path Regex search.
	WhitelistTags []string   // Tags that must exist, tags must be exact.
	BlacklistTags []string   // Tags that cannot exist, tags must be exact.
	Count         int64      // Max number of results to get. Default: 50
	Index         int64      // Index to start getting files at.
	SortBy        SortMethod // Sorting method
	SortReverse   bool       // Sort by DESC instead of ASC
	Hash          string     // Search by hash, or "NULL" to search for values with no hashes, if empty ignore this.
}

// File Database
type FileDb struct {
	db       *sql.DB
	lock     sync.Mutex
	safeMode bool // Enabled if the database is outdated.
}

// Remove a tag from a file
func (d *FileDb) removeFileTag(tx *sql.Tx, fileId int, tag string) error {
	// Format the tag
	tag = strings.ToLower(tag)
	// Get the ID
	slog.Debug("Executing SELECT", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", []any{tag})
	res, err := tx.Query("SELECT id FROM tag_name WHERE value=?", tag)
	if err != nil {
		// The only way this could fail is if the database is fucked up
		tx.Rollback()
		slog.Error("failed to query tag_name table", "Error", err.Error(), "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
		panic(fmt.Sprintf("MediaManager: removeFileTag: .Query failed when it should never have been able to, Error: %v", err))
	}
	defer res.Close()
	// Get the ID
	tagId := -1
	if res.Next() {
		// Get the ID
		err = res.Scan(&tagId)
		if err != nil {
			slog.Error("Scan failed on query, this is a database structure error", "Error", err.Error(), "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
			// This can only happen if the database structure has changed.
			tx.Rollback()
			panic(fmt.Sprintf("MediaManager: removeFileTag: .Scan failed on tag_name.id, is the database correct?: %v", err.Error()))
		}
	} else {
		// Doesn't exist - do nothing
		return nil
	}
	if tagId == -1 {
		// Programmer error
		slog.Error("tagId was -1 even though scan worked", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
		panic("MediaManager: removeFileTag: tagId was -1 when it should have been set")
	}
	// Remove the tag to the file
	slog.Info("Executing DELETE", "Query", "DELETE FROM tag WHERE fileId=? AND tagNameId=?", "QueryArgs", []any{fileId, tagId})
	_, err = tx.Exec("DELETE FROM tag WHERE fileId=? AND tagNameId=?", fileId, tagId)
	if err != nil {
		// This *could* fail if we just deleted from the table
		slog.Error("Failed to delete from 'tag' table", "Query", "DELETE FROM tag WHERE fileId=? AND tagNameId=?", "QueryArgs", []any{fileId, tagId}, "Error", err.Error())
		tx.Rollback()
		return fmt.Errorf("failed to insert into tag table: %v", err)
	}
	// We don't commit, this is typically just part of a larger operation.
	return nil
}

func (d *FileDb) addFileTag(tx *sql.Tx, fileId int, tag string) error {
	tag = strings.ToLower(tag)
	// Check if the tag already exists
	slog.Debug("Executing SELECT", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", []any{tag})
	res, err := tx.Query("SELECT id FROM tag_name WHERE value=?", tag)
	if err != nil {
		slog.Error("failed to query tag_name table", "Error", err.Error(), "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
		panic(fmt.Sprintf("MediaManager: addFileTag: .Query failed when it should never have been able to, Error: %v", err))
	}
	defer res.Close()
	tagId := -1
	if res.Next() {
		// Get the ID
		err = res.Scan(&tagId)
		if err != nil {
			// This can only happen if the database structure has changed.
			slog.Error("Scan failed on query, this is a database structure error", "Error", err.Error(), "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
			tx.Rollback()
			panic(fmt.Sprintf("MediaManager: addFileTag: .Scan failed on tag_name.id, is the database correct?: %v", err))
		}
	} else {
		// Doesn't exist - insert the tag.
		slog.Info("Executing INSERT", "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag})
		r, err := tx.Exec("INSERT INTO tag_name(value) VALUES (?)", tag)
		if err != nil {
			// This *could* fail if we just inserted it (I suppose.)
			slog.Error("Failed to insert into tag_name", "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag}, "Error", err.Error())
			tx.Rollback()
			return fmt.Errorf("failed to insert into tag_name table: %v", err)
		}
		v, err := r.LastInsertId()
		if err != nil {
			// This can only happen if the database structure has changed.
			slog.Error("Failed to get lastInsertId", "Error", err.Error(), "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag})
			tx.Rollback()
			panic(fmt.Sprintf("MediaManager: addFileTag: .LastInsertId failed to get id, is the database correct?: %v", err))
		}
		tagId = int(v)
	}
	if tagId == -1 {
		// Programmer error
		slog.Error("tagId was -1 even though scan worked", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", tag)
		panic("MediaManager: addFileTag: tagId was -1 when it should have been set")
	}
	// Add the tag to the file
	slog.Info("Executing INSERT", "Query", "INSERT INTO tag(fileId, tagNameId) VALUES (?, ?)", "QueryArgs", []any{fileId, tagId})
	_, err = tx.Exec("INSERT INTO tag(fileId, tagNameId) VALUES (?, ?)", fileId, tagId)
	if err != nil {
		slog.Error("Failed to insert into 'tag'", "Error", err.Error(), "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{fileId, tagId})
		tx.Rollback()
		return fmt.Errorf("failed to insert into tag table: %v", err)
	}
	return nil
}

// Deprecated: Use AddFiles
//
// Adds a file to database, sets f.id
func (d *FileDb) AddFile(f *File) error {
	if d.safeMode {
		return ErrOutdatedDatabase
	}
	_, err := d.AddFiles(f)
	return err
}

type ImportError struct {
	File  *File
	Error error
}

func (d *FileDb) putFilesInDb(tx *sql.Tx, files ...*File) (failed []*ImportError, err error) {
	importErrs := make([]*ImportError, 0)
	d.lock.Lock()
	defer d.lock.Unlock()
	for _, f := range files {
		// Remove id
		f.id = 0
		// We're ok if this file already has a ID (Indicating it likely exists in the database) because it would fail the path unique check, and if it doesn't its just a different file.
		// Lock the database so we don't have some weird tag / file race conditions (They wouldn't spoil the database, but they would result in weird errors)
		// For instance, we call 'AddFile' with a file with tag 'a', 'a' doesn't exist in the database, during the for loop the database reports the tag doesn't exist so we add put it into the
		// database with our transaction but, before the transaction is committed 'RemoveTag' is called with 'a;
		// Hash the file before anything else to save time
		// If the hash is already set we can just ignore it.
		// If we don't have a size or hash we just ignore it
		// Insert the file
		lastViewed := f.lastViewed.UTC().Unix()
		queryArgs := []any{
			f.path,
			lastViewed,
			f.stars,
		}
		insertInto := "path, lastViewed, stars"
		argStr := "?, ?, ?"
		if f.size != 0 {
			insertInto += ", size"
			argStr += ", ?"
			queryArgs = append(queryArgs, f.size)
		}
		if f.hash != "" {
			insertInto += ", hash"
			argStr += ", ?"
			queryArgs = append(queryArgs, f.hash)
		}
		query := fmt.Sprintf("INSERT INTO file(%s) VALUES (%s)", insertInto, argStr)
		slog.Info("Executing INSERT", "Query", query, "QueryArgs", queryArgs)
		res, err := tx.Exec(query, queryArgs...)
		if err != nil {
			// This could fail if the file already exists, we don't log it very seriously.
			slog.Debug("Failed to insert file", "Query", query, "QueryArgs", queryArgs, "Error", err.Error())
			importErrs = append(importErrs, &ImportError{
				File:  f,
				Error: fmt.Errorf("failed to insert file hash:% v", err),
			})
			continue
		}
		// Get the inserted files ID
		fileId, err := res.LastInsertId()
		if err != nil {
			// I don't know how this could happen
			slog.Error("Failed to get lastInsertId", "Query", "INSERT INTO file(path, lastViewed, stars, size, hash) VALUES (?, ?, ?, ?, ?)", "QueryArgs", []any{f.path, lastViewed, f.stars, f.size, f.hash}, "Error", err.Error())
			tx.Rollback()
			panic(fmt.Sprintf("MediaManager: AddFile: Failed to get last insert ID for file table: %v", err))
		}
		// Add each file tag, adding to tag_name as needed
		for _, v := range f.tags {
			err = d.addFileTag(tx, int(fileId), v)
			if err != nil {
				// This is actually concerning as this shouldn't happen.
				slog.Warn("Aborting AddFile after failing to add tag", "Error", err.Error(), "Tag", v)
				// Half of me thinks this should be a panic (Why would this *ever* happen? its 2 numbers and no unique constraints) but
				// there might be some edge case - either way for now we need to rollback.
				tx.Rollback()
				// Database rolled back, ignore importErrs
				return nil, fmt.Errorf("failed to add tag '%s', transaction must be rolled back: %v", v, err)
			}
		}
		// Now set id
		f.id = int(fileId)
	}
	return importErrs, nil
}

// Add files as they are, if size or hash are not provided they will be set to NULL
//
// This may corrupt file stats & allow for weird things because there id may be set to a ghost value if the transactions was rolled back.
func (d *FileDb) AddFiles(files ...*File) (failed []*ImportError, err error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	// First we'll hash the file as needed (We do this in one goroutine just incase we have some sorta speed hangup)
	// Start the transaction, this will include pushing any tags needed.
	tx, err := d.db.Begin()
	if err != nil {
		slog.Error("Failed to create new transaction for AddFile", "Error", err.Error())
		return nil, err
	}
	importErrs, err := d.putFilesInDb(tx, files...)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// Now we can commit
	err = tx.Commit()
	if err != nil {
		// This is concerning because we have the database locked & no other inset/delete/update operations should be happening.
		slog.Warn("Failed to commit AddFile", "Error", err.Error())
		// Everything has failed at this point - the transaction will not be committed so everything is a error.
		return nil, fmt.Errorf("transaction failed to commit: %v", err)
	}
	return importErrs, nil
}

// Add files while also adding hash & size info, this operation may take a while, if you'd like a progres bar
// you can use AddFileInfoWithProgressBar to add the info first.
func (d *FileDb) AddFilesWithInfo(goroutines int, files ...*File) (failed []*ImportError, err error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	tx, err := d.db.Begin()
	if err != nil {
		slog.Error("Failed to create new transaction for AddFile", "Error", err.Error())
		return nil, err
	}
	// First we manually add files.
	importErrs, err := d.putFilesInDb(tx, files...)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	toHash := make([]*File, 0)
	// Now we go through each new file and add hashes
	for _, v := range files {
		if v.id == 0 {
			// File wasn't added
			continue
		}
		// Mark we should hash it
		toHash = append(toHash, v)
	}
	// Hash the files
	err = AddInfoToFiles(&AddInfoOpts{
		DontAddHash:       false,
		DontAddSize:       false,
		Goroutines:        12,
		Context:           context.Background(),
		ProgressChan:      nil,
		ProgressBarWriter: nil,
	}, toHash...)
	if err != nil {
		return nil, err
	}
	// Ad hashes
	for _, v := range toHash {
		slog.Info("Executing UPDATE", "Query", "UPDATE file SET hash=?, size=? WHERE id=?", "QueryArgs", []any{v.hash, v.size, v.id})
		_, err := tx.Exec("UPDATE file SET hash=?, size=? WHERE id=?", v.hash, v.size, v.id)
		if err != nil {
			importErrs = append(importErrs, &ImportError{
				File:  v,
				Error: fmt.Errorf("failed to add hashed file: %v", err),
			})
		}
	}
	// Now we can commit
	err = tx.Commit()
	if err != nil {
		// This is concerning because we have the database locked & no other inset/delete/update operations should be happening.
		slog.Warn("Failed to commit AddFile", "Error", err.Error())
		// Everything has failed at this point - the transaction will not be committed so everything is a error.
		return nil, fmt.Errorf("transaction failed to commit: %v", err)
	}
	err = AddFileInfo(goroutines, files...)
	if err != nil {
		return nil, fmt.Errorf("failed to add file info: %v", err)
	}
	return importErrs, nil
}

// Updates a files tags, stars and path
func (d *FileDb) UpdateFile(f *File) error {
	if d.safeMode {
		return ErrOutdatedDatabase
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	// File must have a id
	if err := isValidFile(f); err != nil {
		return err
	}
	oldFile, err := d.GetFileById(f.id)
	if err != nil {
		// This could just be the user passing a null file or something. Probably not that bad but still worth logging.
		slog.Info("Failed to get old file for UpdateFile", "Error", err.Error(), "Id", f.id)
		return fmt.Errorf("failed to get old file: %v", err)
	}
	tx, err := d.db.Begin()
	if err != nil {
		slog.Error("Failed to create new transaction for UpdateFile", "Error", err.Error())
		return err
	}
	lastViewed := f.lastViewed.UTC().Unix()
	if f.hash == "" {
		slog.Info("Executing UPDATE", "Query", "UPDATE file SET path=?, lastViewed=?, stars=?, size=? WHERE id=?", "QueryArgs", []any{f.path, lastViewed, f.stars, f.size, f.id})
		_, err = tx.Exec("UPDATE file SET path=?, lastViewed=?, stars=?, size=? WHERE id=?", f.path, lastViewed, f.stars, f.size, f.id)
		if err != nil {
			// This could fail if the path is no longer unique, but its still a issue.
			slog.Warn("Failed to update file", "Query", "UPDATE file SET path=?, lastViewed=?, stars=?, size=? WHERE id=?", "QueryArgs", []any{f.path, lastViewed, f.stars, f.size, f.id}, "Error", err.Error())
			tx.Rollback()
			return fmt.Errorf("failed to update file: %v", err)
		}
	} else {
		slog.Info("Executing UPDATE", "Query", "UPDATE file SET path=?, lastViewed=?, stars=?, size=?, hash=? WHERE id=?", "QueryArgs", []any{f.path, lastViewed, f.stars, f.size, f.hash, f.id})
		_, err = tx.Exec("UPDATE file SET path=?, lastViewed=?, stars=?, size=?, hash=? WHERE id=?", f.path, lastViewed, f.stars, f.size, f.hash, f.id)
		if err != nil {
			// This could fail if the path is no longer unique, but its still a issue.
			slog.Warn("Failed to update file", "Query", "UPDATE file SET path=?, lastViewed=?, stars=?, size=?, hash=? WHERE id=?", "QueryArgs", []any{f.path, lastViewed, f.stars, f.size, f.hash, f.id}, "Error", err.Error())
			tx.Rollback()
			return fmt.Errorf("failed to update file: %v", err)
		}
	}
	// Figure out what tags we need to remove & what tags we need to add
	for _, v := range f.tags {
		if slices.Index(oldFile.tags, v) == -1 {
			slog.Debug("Adding new file tag", "FileId", f.id, "Tag", v)
			// Does't exist in old file, add it
			err = d.addFileTag(tx, f.id, v)
			if err != nil {
				slog.Warn("Aborting UpdateFile after failing to add tag", "Error", err.Error(), "Tag", v, "File.Id", f.id)
				tx.Rollback()
				return err
			}
		}
	}
	for _, v := range oldFile.tags {
		if slices.Index(f.tags, v) == -1 {
			slog.Debug("Removing tag", "Id", f.id, "File", f.path, "Tag", v)
			err = d.removeFileTag(tx, f.id, v)
			if err != nil {
				slog.Warn("Failed to remove tag", "Error", err.Error(), "Tag", v, "File.Id", f.id)
				tx.Rollback()
				return err
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		// This could happen because
		slog.Warn("Failed to commit UpdateFile", "Error", err.Error(), "File", *f)
		return fmt.Errorf("transaction failed to commit: %v", err)
	}
	return nil
}

// Remove a file by File
func (d *FileDb) RemoveFile(f *File) error {
	if d.safeMode {
		return ErrOutdatedDatabase
	}
	if err := isValidFile(f); err != nil {
		return err
	}
	tx, err := d.db.Begin()
	if err != nil {
		slog.Error("Failed to create new transaction for RemoveFile", "Error", err.Error())
		return err
	}
	_, err = tx.Exec("DELETE FROM tag WHERE fileId=?", f.id)
	if err != nil {
		slog.Warn("Failed to delete file tags from database", "Query", "DELETE FROM tag WHERE fileId=?", "QueryArgs", []any{f.id}, "Error", err.Error())
		tx.Rollback()
		return fmt.Errorf("failed to remove from tag table: %v", err)
	}
	slog.Info("Executing DELETE", "Query", "DELETE FROM file WHERE id=?", "QueryArgs", []any{f.id})
	_, err = tx.Exec("DELETE FROM file WHERE id=?", f.id)
	if err != nil {
		// This could happen if the File has already been deleted & was then re ran
		slog.Warn("Failed to delete from database", "Query", "DELETE FROM file WHERE id=?", "QueryArgs", []any{f.id}, "Error", err.Error())
		tx.Rollback()
		return fmt.Errorf("failed to remove from file table: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		slog.Warn("Failed to commit RemoveFile", "Error", err.Error(), "File", *f)
		return fmt.Errorf("transaction failed to commit: %v", err)
	}
	// Remove its ID, this makes the file invalid & will fail a earlier check.
	f.id = 0
	return nil
}

// Don't call .Next before calling this function or you will lose a file.
func (d *FileDb) sqlRowsToFiles(r *sql.Rows) []*File {
	files := make([]*File, 0)
	// We don't get tag yet.
	for r.Next() {
		f := &File{
			tags: make([]string, 0),
		}
		// Expected type: nil or string
		var hashStr interface{}
		// Expected type: nil or int
		var sizeInt interface{}
		timeval := int64(0)
		// I think this one is slow as fuck.
		err := r.Scan(&f.id, &f.path, &timeval, &f.stars, &sizeInt, &hashStr)
		if err != nil {
			// The only way this fails if we don't pass a correct Rows value or the database is wrong, this is a programmer error.
			// There really isn't much for us to pass here.
			slog.Error("Failed to scan from File rows", "Error", err.Error())
			// This can only happen if the database structure has changed.
			panic(fmt.Sprintf("MediaManager: sqlRowsToFiles: Scanning from file rows failed: %v", err))
		}
		if sizeInt != nil {
			var ok bool
			f.size, ok = sizeInt.(int64)
			if !ok {
				panic(fmt.Sprintf("MediaManager: sqlRowsToFiles: Failed to size to integer, was %v", reflect.TypeOf(sizeInt)))
			}
		}
		if hashStr != nil {
			var ok bool
			f.hash, ok = hashStr.(string)
			if !ok {
				panic(fmt.Sprintf("MediaManager: sqlRowsToFiles: Failed to hash to string, was %v", reflect.TypeOf(hashStr)))
			}
		}
		// Now we need to parse the time
		f.lastViewed = time.Unix(timeval, 0)
		files = append(files, f)
	}
	// Now we get all tags
	d.addFileTags(files)
	return files
}

// TODO: Further optimize (Less queries.)
//
// Add tags to files in a bundled manner (Improving speed)
func (d *FileDb) addFileTags(files []*File) {
	/*profileOut, err := os.Create("cpoprof")
	if err != nil {
		panic("failed to open cpu profiling file")
	}
	pprof.StartCPUProfile(profileOut)
	defer pprof.StopCPUProfile()*/
	if len(files) == 0 {
		// Don't do anything
		return
	}
	// Create file list, we need this because SELECT max queries is 1000 (Which means about 998 files)
	// We split into 500 because it has no performance hit & its a nicer number
	fileList := make([][]*File, 0)
	for i := 0; i < len(files); i += 500 {
		topIndex := i + 500
		if topIndex > len(files) {
			topIndex = len(files)
		}
		fileList = append(fileList, files[i:topIndex])
	}
	// Maybe only get what we need.
	tagCache := d.GetAllTags()
	tags := make(map[int][]string)
	for _, v := range fileList {
		// Get file.Id -> Tag.Id
		query := "SELECT T.fileId, T.tagNameId FROM tag T WHERE "
		queryArgs := make([]any, 0, len(v))
		needsOr := false
		for _, v := range v {
			if needsOr {
				query += " or "
			} else {
				needsOr = true
			}
			query += "T.fileId=?"
			queryArgs = append(queryArgs, v.id)
		}
		slog.Debug("Executing SELECT", "Query", query, "QueryArgs", queryArgs)
		rows, err := d.db.Query(query, queryArgs...)
		if err != nil {
			slog.Error("Failed to get file tags", "Error", err.Error(), "Query", query, "QueryArgs", queryArgs)
			panic(fmt.Sprintf("MediaManager: addFileTags: Failed to get file tags: %v", err))
		}
		for rows.Next() {
			fileId := 0
			tagId := 0
			err = rows.Scan(&fileId, &tagId)
			if err != nil {
				slog.Error("Failed to scan fileId & tag", "Error", err.Error(), "Query", query, "QueryArgs", queryArgs)
				panic(fmt.Sprintf("MediaManager: addFileTags: Failed to get file id & tag: %v", err))
			}
			if _, found := tags[fileId]; !found {
				tags[fileId] = make([]string, 0)
			}
			if t, found := tagCache[tagId]; found {
				tags[fileId] = append(tags[fileId], t)
			} else {
				panic(fmt.Sprintf("MediaManager: addFileTags: Corrupted database, tag id '%d' on file '%d' doesn't exist", tagId, fileId))
			}
		}
	}
	// Add everything
	for k, v := range tags {
		for _, f := range files {
			if f.id == k {
				f.tags = v
			}
		}
	}
}

// Get a file by path
func (d *FileDb) GetFileByPath(path string) (f *File, err error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM file WHERE path=?", "QueryArgs", []any{path})
	rows, err := d.db.Query("SELECT * FROM file WHERE path=?", path)
	if err != nil {
		// Fatal.
		slog.Error("Failed to execute select query", "Query", "SELECT * FROM file WHERE path=?", "QueryArgs", []any{path}, "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: GetFileByPath query failed: %v", err))
	}
	defer rows.Close()
	sFile := d.sqlRowsToFiles(rows)
	if len(sFile) == 0 {
		return nil, errors.New("file not found")
	}
	if len(sFile) > 1 {
		// UNIQUE constraint failed?
		slog.Error("Got multiple files on query that should have got one", "Query", "SELECT * FROM file WHERE path=?", "QueryArgs", []any{path}, "File", len(sFile))
		panic(fmt.Sprintf("MediaManager: GetFileByPath got multiple files count: %v", len(sFile)))
	}
	return sFile[0], nil
}

// Get a file by ID
func (d *FileDb) GetFileById(id int) (*File, error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM file WHERE id=?", "QueryArgs", []any{id})
	rows, err := d.db.Query("SELECT * FROM file WHERE id=?", id)
	if err != nil {
		slog.Error("Failed to execute select query", "Query", "SELECT * FROM file WHERE id=?", "QueryArgs", []any{id}, "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: GetFileById query failed: %v", err))
	}
	defer rows.Close()
	sFile := d.sqlRowsToFiles(rows)
	if len(sFile) == 0 {
		// File not found
		return nil, errors.New("file not found")
	}
	if len(sFile) > 1 {
		// Dereference them for logging
		derefFiles := make([]File, 0, len(sFile))
		for _, v := range sFile {
			derefFiles = append(derefFiles, *v)
		}
		slog.Error("Expected to get 1 file from GetFileById, got multiple, this should have failed the 'UNIQUE' constraint", "Id", id, "Count", len(sFile), "Files", derefFiles)
		panic(fmt.Sprintf("MediaManager: GetFileById: Expected to get exactly 1 file by Id, got %d, this should have failed the 'UNIQUE' constraint. Id: %d", len(sFile), id))
	}
	return sFile[0], nil
}

// Search for a file, this can be used with a empty search justing using .Count and .Index to list files
// .Count defaults to 50 if set to 0, the next index value should be the last files id+1, if .Count is negative all files are got
//
// This creates a MONSTER query, thats probably awfully optimized but whatever.
func (d *FileDb) SearchFile(q *SearchQuery) ([]*File, error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	// When we build our query we need to respect the order
	// SELECT
	// FROM
	// (WHERE)
	// (AND)
	// ORDER BY
	// LIMIT
	// OFFSET
	if q == nil {
		q = &SearchQuery{}
	}
	if q.Count == 0 {
		q.Count = 50
	}
	// We'll just log the final query, it's enough to look through and see where things went wrong anyway.
	needsAnd := false
	queries := "SELECT DISTINCT f.* FROM file f"
	// I don't know how to do the blacklist & I don't care.
	/*if len(q.BlacklistTags) != 0 {
		queries += "JOIN tag bt ON f.id = bt.fileId JOIN tag_name btn ON bt.tagNameId = bnt.id"
	}*/
	qrArgs := make([]any, 0)
	// This must go first so we can add our joins
	for i, v := range q.WhitelistTags {
		queries += fmt.Sprintf(" JOIN tag wt%d ON f.id = wt%d.fileId JOIN tag_name wtn%d ON wt%d.tagNameId = wtn%d.id AND wtn%d.value = ?", i, i, i, i, i, i)
		qrArgs = append(qrArgs, v)
	}
	// Now add the blacklist (I think this is going to tank performance
	for i, v := range q.BlacklistTags {
		if needsAnd {
			queries += " AND"
		} else {
			queries += " WHERE"
		}
		queries += fmt.Sprintf(" f.id NOT IN (SELECT fileId FROM tag bt%d JOIN tag_name btn%d ON bt%d.tagNameId = btn%d.id WHERE btn%d.value = ?)", i, i, i, i, i)
		qrArgs = append(qrArgs, v)
		needsAnd = true
	}
	if q.Path != "" {
		if needsAnd {
			queries += " AND"
		} else {
			queries += " WHERE"
		}
		queries += " f.path LIKE ?"
		qrArgs = append(qrArgs, "%"+q.Path+"%")
		needsAnd = true
	}
	if q.PathRe != "" {
		if needsAnd {
			queries += " AND"
		} else {
			queries += " WHERE"
		}
		queries += " f.path regexp ?"
		qrArgs = append(qrArgs, q.PathRe)
		needsAnd = true
	}
	if q.Hash != "" {
		if needsAnd {
			queries += " AND"
		} else {
			queries += " WHERE"
		}
		if q.Hash == "NULL" {
			queries += " f.hash IS NULL"
		} else {
			queries += " f.hash = ?"
		}
		qrArgs = append(qrArgs, q.Hash)
	}
	wasSorted := false
	switch q.SortBy {
	case SortMethodNone:
		// Do nothing
	case SortMethodSize:
		queries += " ORDER BY size"
		wasSorted = true
	case SortMethodStars:
		queries += " ORDER BY stars"
		wasSorted = true
	case SortMethodId:
		queries += " ORDER BY id"
		wasSorted = true
	case SortMethodLastViewed:
		queries += " ORDER BY lastViewed"
		wasSorted = true
	case SortMethodRandom:
		queries += " ORDER BY RANDOM()"
		wasSorted = true
	default:
		slog.Error("Got invalid q.SortBy value", "Value", q.SortBy, "Query", *q)
		panic(fmt.Sprintf("MediaManager: FileDb.SearchFile: Got unexpected q.SortBy value '%d'", q.SortBy))
	}
	if wasSorted {
		if q.SortReverse {
			queries += " DESC"
		} else {
			queries += " ASC"
		}
	}
	// Limit the number of elements & offset it
	if q.Count >= 0 {
		queries += " LIMIT ? OFFSET ?"
		qrArgs = append(qrArgs, q.Count, q.Index)
	}
	slog.Debug("Executing SELECT", "QueryString", queries, "QueryArgs", qrArgs, "SearchQuery", *q)
	rows, err := d.db.Query(queries, qrArgs...)
	if err != nil {
		// This is probably a sign of something bad happening
		slog.Error("Search query failed", "QueryString", queries, "QueryArgs", qrArgs, "SearchQuery", *q, "Error", err.Error())
		return nil, fmt.Errorf("failed to execute search: %v, Query: %s, Args: %+v", err, queries, qrArgs)
	}
	defer rows.Close()
	files := d.sqlRowsToFiles(rows)
	return files, nil
}

// Check if a tag exists in the database
func (d *FileDb) HasTag(tag string) bool {
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM tag_name WHERE value=?", "QueryArgs", []any{tag})
	rows, err := d.db.Query("SELECT * FROM tag_name WHERE value=?", tag)
	if err != nil {
		// This shouldn't because tag_name.value should exist.
		slog.Error("Failed to select from tag_name", "Query", "SELECT * FROM tag_name WHERE value=?", "QueryArgs", []any{tag}, "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: HasTag panic: %v", err))
	}
	defer rows.Close()
	return rows.Next()
}

// Get all tags and IDs
func (d *FileDb) GetAllTags() map[int]string {
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM tag_name", "QueryArgs", []any{})
	tags, err := d.db.Query("SELECT * FROM tag_name")
	if err != nil {
		// This can panic - we have no user defined values here & the table should exist.
		slog.Error("Failed to query tag_name", "Query", "SELECT * FROM tag_name", "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: GetAllTags: Failed to query tag_name: %v", err))
	}
	defer tags.Close()
	result := make(map[int]string)
	for tags.Next() {
		id := int(0)
		t := ""
		err = tags.Scan(&id, &t)
		if err != nil {
			// Database changed
			slog.Error("Failed to scan tag_name result", "Error", err.Error(), "Query", "SELECT * from tag_name")
			panic(fmt.Sprintf("MediaManager: GetAllTags: Failed to scan tag_name result: %v", err))
		}
		result[id] = t
	}
	return result
}

// Close the database
func (d *FileDb) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	slog.Debug("Closing FileDb")
	return d.db.Close()
}

// Add a tag
func (d *FileDb) AddTag(tag string) (int, error) {
	if d.safeMode {
		return 0, ErrOutdatedDatabase
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	tx, err := d.db.Begin()
	if err != nil {
		// Idk the conditions this could fail enough to make in a panic - I think not panicking when the database is closed (from .Close) is fine
		slog.Error("Failed to begin transaction for RemoveTag", "Error", err.Error())
		return 0, err
	}
	tag = strings.ToLower(tag)
	// Doesn't exist - insert the tag.
	slog.Info("Executing INSERT", "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag})
	r, err := tx.Exec("INSERT INTO tag_name(value) VALUES (?)", tag)
	if err != nil {
		// This would fail if the unique constraint fails.
		slog.Info("Failed to insert into tag_name", "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag}, "Error", err.Error())
		tx.Rollback()
		return 0, fmt.Errorf("failed to insert into tag_name table: %v", err)
	}
	v, err := r.LastInsertId()
	if err != nil {
		// This can only happen if the database structure has changed.
		slog.Error("Failed to get lastInsertId", "Error", err.Error(), "Query", "INSERT INTO tag_name(value) VALUES (?)", "QueryArgs", []any{tag})
		tx.Rollback()
		panic(fmt.Sprintf("MediaManager: addFileTag: .LastInsertId failed to get id, is the database correct?: %v", err))
	}
	err = tx.Commit()
	if err != nil {
		// This is concerning because we have the database locked & no other inset/delete/update operations should be happening.
		slog.Warn("Failed to commit AddTag", "Error", err.Error(), "Tag", tag)
		return 0, fmt.Errorf("transaction failed to commit: %v", err)
	}
	return int(v), nil
}

// Remove a tag, every file with this tag will have it removed, if the tag isn't found a error is returned
func (d *FileDb) RemoveTag(tag string) error {
	if d.safeMode {
		return ErrOutdatedDatabase
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	tx, err := d.db.Begin()
	if err != nil {
		// Idk the conditions this could fail enough to make in a panic - I think not panicking when the database is closed (from .Close) is fine
		slog.Error("Failed to begin transaction for RemoveTag", "Error", err.Error())
		return err
	}
	// Get its id
	slog.Debug("Executing SElECT", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", []any{tag})
	res, err := tx.Query("SELECT id FROM tag_name WHERE value=?", tag)
	if err != nil {
		// This shouldn't fail
		slog.Error("Failed to query 'tag_name'", "Query", "SELECT id FROM tag_name WHERE value=?", "QueryArgs", []any{tag}, "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: RemoveTag: Failed to query tag_name.value, did the database change?: %v", err))
	}
	defer res.Close()
	if !res.Next() {
		// This would happen if the tag was not found
		slog.Debug("Tag not found")
		tx.Rollback()
		return errors.New("tag not found")
	}
	id := 0
	err = res.Scan(&id)
	if err != nil {
		tx.Rollback()
		slog.Error("Failed to scan tag_name.id value", "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: RemoveTag: Failed to scan tag ID: %v", err))
	}
	// Now remove everything with its id
	slog.Info("Executing DELETE", "Query", "DELETE FROM tag WHERE tagNameId=?", "QueryArgs", []any{id})
	_, err = tx.Exec("DELETE FROM tag WHERE tagNameId=?", id)
	if err != nil {
		// Shouldn't fail
		slog.Error("Failed to delete from tag table", "Error", err.Error(), "Query", "DELETE FROM tag WHERE tagNameId=?", "QueryArgs", []any{id})
		tx.Rollback()
		panic(fmt.Sprintf("MediaManager: RemoveTag: Failed to execute delete on 'tag': %v", err))
	}
	slog.Info("Executing DELETE", "Query", "DELETE FROM tag_name WHERE id=?", "QueryArgs", []any{id})
	_, err = tx.Exec("DELETE FROM tag_name WHERE id=?", id)
	if err != nil {
		// Also should never fail
		slog.Error("Failed to delete from tag_name table", "Error", err.Error(), "Query", "DELETE FROM tag_name WHERE id=?", "QueryArgs", []any{id})
		tx.Rollback()
		panic(fmt.Sprintf("MediaManager: RemoveTag: Failed to execute delete on 'tag_name': %v", err))
	}
	err = tx.Commit()
	if err != nil {
		// This should never fail
		slog.Error("Failed to commit transaction with deletes on 'tag_name' and 'tag'", "Error", err.Error(), "TagId", id)
		panic(fmt.Sprintf("MediaManager: RemoveTag: Failed to commit deletes on 'tag_name' and 'tag': %v", err))
	}
	return nil
}

type UpdateFileInfo struct {
	UpdateGoroutines int  // Default: 100
	ShowProgressBar  bool // Default: false
}

type UpdateFileResult struct {
	File  *File
	Error error
}

// This function may be changed, it is experimental.
//
// Adds info to files, any file that would be removed due to a unique hash info will be returned.
func (d *FileDb) AddInfoToFiles(opts *UpdateFileInfo) (updated []*UpdateFileResult, err error) {
	if d.safeMode {
		return nil, ErrOutdatedDatabase
	}
	if opts == nil {
		opts = &UpdateFileInfo{
			UpdateGoroutines: 100,
			ShowProgressBar:  false,
		}
	}
	if opts.UpdateGoroutines <= 0 {
		return nil, errors.New("at least 1 goroutine must be allocated")
	}
	/*
		slog.Debug("Executing SELECT", "Query", "SELECT * FROM file WHERE hash IS NULL OR size IS NULL")
		rows, err := d.db.Query("SELECT * FROM file WHERE hash IS NULL OR size IS NULL")
		if err != nil {
			slog.Error("Failed to execute select query", "Query", "SELECT * FROM file ORDER BY RANDOM() LIMIT 1", "Error", err.Error())
			panic(fmt.Sprintf("MediaManager: GetRandomFile query failed: %v", err))
		}
		defer rows.Close()
		sFile := d.sqlRowsToFiles(rows)
	*/
	sFile, err := d.SearchFile(&SearchQuery{
		Count: -1,
		Hash:  "NULL",
	})
	if err != nil {
		slog.Error("Failed to execute select query", "Query", "SELECT * FROM file ORDER BY RANDOM() LIMIT 1", "Error", err.Error())
		panic(fmt.Sprintf("MediaManager: GetRandomFile query failed: %v", err))
	}
	updated = make([]*UpdateFileResult, 0, len(sFile))
	if opts.ShowProgressBar {
		err = AddFileInfoWithProgressBar(opts.UpdateGoroutines, sFile...)
	} else {
		err = AddFileInfo(opts.UpdateGoroutines, sFile...)
	}
	if err != nil {
		slog.Warn("Failed to add file info", "Error", err.Error())
		return nil, fmt.Errorf("failed to add file info: %v", err)
	}
	// Now we need to update each file manually so we can tell whats bad.
	tx, err := d.db.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction in AddInfoToFiles", "Error", err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	for _, v := range sFile {
		if v.hash == "" || v.size == 0 {
			slog.Warn("File info was not added to file", "File.Id", v.id, "File.Path", v.path, "File.Hash", v.hash, "File.Size", v.size)
			continue
		}
		slog.Info("Executing UPDATE", "Query", "UPDATE file SET hash=?, size=? WHERE id=?", "QueryArgs", []any{v.hash, v.size, v.id})
		_, err := tx.Exec("UPDATE file SET hash=?, size=? WHERE id=?", v.hash, v.size, v.id)
		if err != nil {
			// This could just be hash thing - Its expected some of those may happen.
			slog.Info("Update failed", "Error", err.Error(), "Query", "UPDATE file SET hash=?, size=? WHERE id=?", "QueryArgs", []any{v.hash, v.size, v.id})
			updated = append(updated, &UpdateFileResult{
				File:  v,
				Error: err,
			})
			continue
		}
		updated = append(updated, &UpdateFileResult{
			File:  v,
			Error: nil,
		})
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("Failed to commit transaction", "Error", err.Error())
		return nil, err
	}
	return updated, nil
}

// Figure out legacy versions
func (f *FileDb) getLegacyMetadata() (*DbMetadata, error) {
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM db_info")
	qr, err := f.db.Query("SELECT * FROM db_info")
	if err != nil {
		// Figure out if its 1 or 2.
		// Some old tables had 'hash' in full caps instead of all lowercase.
		r, err := f.db.Query("SELECT name, type, \"notnull\", pk FROM pragma_table_info(\"file\") WHERE name like 'hash'")
		if err != nil {
			// Not a filedb database.
			slog.Warn("Failed to get table info on 'file'", "Error", err)
			return nil, fmt.Errorf("not a known filedb database version")
		}
		if r.Next() {
			return &DbMetadata{
				MajorVersion:    2,
				MinorVersion:    0,
				RevisionVersion: 0,
				VersionCodeName: MajorVersionToCodeName(2),
			}, nil
		}
		return &DbMetadata{
			MajorVersion:    1,
			MinorVersion:    0,
			RevisionVersion: 0,
			VersionCodeName: MajorVersionToCodeName(1),
		}, nil
	}
	major := 0
	minor := 0
	revision := 0
	for qr.Next() {
		if major != 0 || minor != 0 || revision != 0 {
			slog.Warn("Multiple entries in db_info table")
		}
		err := qr.Scan(&major, &minor, &revision)
		if err != nil {
			// This shouldn't happen.
			slog.Error("Failed to scan db_info table, did the structure change", "Error", err.Error())
			panic(fmt.Sprintf("FileDb: Failed to scan db_info, did the structure change?: %v", err))
		}
	}
	return &DbMetadata{
		MajorVersion:    major,
		MinorVersion:    minor,
		RevisionVersion: revision,
		VersionCodeName: MajorVersionToCodeName(major),
	}, nil
}

type DbMetadata struct {
	MajorVersion    int
	MinorVersion    int
	RevisionVersion int
	VersionCodeName string
	Experimental    bool

	Map map[string]any // All metadata
}

func (d *DbMetadata) VersionString() string {
	return FormatVersion(d.MajorVersion, d.MinorVersion, d.RevisionVersion)
}

func (f *FileDb) GetMetadata() (*DbMetadata, error) {
	slog.Debug("Executing SELECT", "Query", "SELECT * FROM db_info")
	qr, err := f.db.Query("SELECT * FROM db_info")
	if err != nil {
		return f.getLegacyMetadata()
	}
	db := &DbMetadata{
		MajorVersion:    -1,
		MinorVersion:    -1,
		RevisionVersion: -1,
		VersionCodeName: "",
		Experimental:    false,
		Map:             make(map[string]any),
	}
	for qr.Next() {
		key := ""
		var value any
		err := qr.Scan(&key, &value)
		if err != nil {
			// Probably legacy (Used to have 3 fields)
			slog.Warn("Failed to scan entry in db_info, using legacy lookup", "Error", err.Error())
			return f.getLegacyMetadata()
		}
		db.Map[key] = value
		switch key {
		case "majorVersion":
			v, ok := value.(int64)
			if !ok {
				slog.Error("Expected 'majorVersion' to a be integer", "Type", reflect.TypeOf(value))
				panic(fmt.Sprintf("FileDb: 'majorVersion' was not a int, did the structure change?: Was %t", value))
			}
			db.MajorVersion = int(v)
		case "minorVersion":
			v, ok := value.(int64)
			if !ok {
				slog.Error("Expected 'minorVersion' to a be integer", "Type", reflect.TypeOf(value))
				panic(fmt.Sprintf("FileDb: 'minorVersion' was not a int, did the structure change?: %t", value))
			}
			db.MinorVersion = int(v)
		case "revision":
			v, ok := value.(int64)
			if !ok {
				slog.Error("Expected 'revision' to a be integer", "Type", reflect.TypeOf(value))
				panic(fmt.Sprintf("FileDb: 'revision' was not a int, did the structure change?: %t", value))
			}
			db.RevisionVersion = int(v)
		case "versionName":
			v, ok := value.(string)
			if !ok {
				slog.Error("Expected 'versionName' to a be string", "Type", reflect.TypeOf(value))
				panic(fmt.Sprintf("FileDb: 'revision' was not a string, did the structure change?: %t", value))
			}
			db.VersionCodeName = v
		case "experimental":
			v, ok := value.(int64)
			if !ok {
				slog.Error("Expected 'experimental' to a be bool", "Type", reflect.TypeOf(value))
				panic(fmt.Sprintf("FileDb: 'experimental' was not a bool, did the structure change?: %t", value))
			}
			db.Experimental = v > 0
		default:
			slog.Warn("Ignoring unknown metadata", "Key", key, "Value", value, "ValueType", reflect.TypeOf(value))
		}
	}
	if db.MajorVersion == -1 {
		slog.Warn("Didnt get a 'majorVersion' entry")
		return nil, fmt.Errorf("missing required field 'majorVersion'")
	}
	if db.MinorVersion == -1 {
		slog.Warn("Didnt get a 'minorVersion' entry")
		return nil, fmt.Errorf("missing required field 'minorVersion'")
	}
	if db.RevisionVersion == -1 {
		slog.Warn("Didnt get a 'revision' entry")
		return nil, fmt.Errorf("missing required field 'revision'")
	}
	if db.VersionCodeName == "" {
		slog.Warn("Didnt get a 'versionName' entry")
		return nil, fmt.Errorf("missing required field 'versionName'")
	}
	return db, nil
}

func (f *FileDb) IsSafeMode() bool {
	return f.safeMode
}

func (f *FileDb) setupMetadata(meta *DbMetadata) {
	if meta == nil {
		meta = &DbMetadata{
			MajorVersion:    MajorVersion,
			MinorVersion:    MinorVersion,
			RevisionVersion: Revision,
			VersionCodeName: MajorVersionToCodeName(MajorVersion),
			Experimental:    false,
			Map:             nil,
		}
	}
	_, err := f.db.Exec("INSERT INTO db_info VALUES ('majorVersion', ?)", meta.MajorVersion)
	if err != nil {
		panic(fmt.Sprintf("MediaManager: Failed into insert 'majorVersion' into 'db_info': %v", err))
	}
	_, err = f.db.Exec("INSERT INTO db_info VALUES ('minorVersion', ?)", meta.MinorVersion)
	if err != nil {
		panic(fmt.Sprintf("MediaManager: Failed into insert 'minorVersion' into 'db_info': %v", err))
	}
	_, err = f.db.Exec("INSERT INTO db_info VALUES ('revision', ?)", meta.RevisionVersion)
	if err != nil {
		panic(fmt.Sprintf("MediaManager: Failed into insert 'revision' into 'db_info': %v", err))
	}
	_, err = f.db.Exec("INSERT INTO db_info VALUES ('versionName', ?)", meta.VersionCodeName)
	if err != nil {
		panic(fmt.Sprintf("MediaManager: Failed into insert 'versionName' into 'db_info': %v", err))
	}
	if meta.Experimental {
		_, err = f.db.Exec("INSERT INTO db_info VALUES ('experimental', ?)", true)
		if err != nil {
			panic(fmt.Sprintf("MediaManager: Failed into insert 'Experimental' into 'db_info': %v", err))
		}
	}
}

// Open a new file db, or create one if one doesn't exist.
func NewFileDb(dbPath string) (*FileDb, error) {
	createNewDb := false
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("Database file not found, creating new database", "Path", dbPath, "Stat.Error", err)
			createNewDb = true
		}
		// Just let it pass, the error will be continued below if its actually a problem.
	}
	db, err := sql.Open("sqlite3-re", dbPath)
	if err != nil {
		// We don't log this heavily because the user will
		slog.Debug("Failed to open database file", "Error", err.Error(), "path", dbPath)
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	_, err = db.Exec("PRAGMA foreign_keys = true")
	if err != nil {
		db.Close()
		slog.Debug("Failed to execute PRAGMA foreign_keys = true", "Error", err.Error(), "Path", dbPath)
		return nil, fmt.Errorf("failed to enable foreign keys check: %v", err)
	}
	f := &FileDb{
		db: db,
	}
	if createNewDb {
		tx, err := db.Begin()
		if err != nil {
			slog.Error("Failed to create transaction on new database", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to start new tx: %v", err)
		}
		slog.Info("Creating 'db_info' table")
		_, err = tx.Exec(`CREATE TABLE db_info (
			key TEXT UNIQUE NOT NULL ,
			value ANY
		)`)
		if err != nil {
			slog.Error("Failed to create 'db_info' table", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to create 'db_info' table: %v", err)
		}
		slog.Info("Creating 'file' table")
		_, err = tx.Exec(`CREATE TABLE file (
		id INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE NOT NULL,
  		path TEXT NOT NULL UNIQUE,
  		lastViewed INTEGER NOT NULL,
  		stars INTEGER,
		size INTEGER,
		hash TEXT UNIQUE,
		CHECK(HASH is NULL OR length(HASH) == 64),
		CHECK(stars >= 0 AND stars <= 5)
		)`)
		if err != nil {
			slog.Error("Failed to create 'file' table", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to create 'file' table: %v", err)
		}
		slog.Info("Creating 'tag' table")
		_, err = tx.Exec(`CREATE TABLE tag (
  		fileId INTEGER NOT NULL,
  		tagNameId INTEGER NOT NULL,
		FOREIGN KEY (fileId) REFERENCES file(id),
		FOREIGN KEY (tagNameId) REFERENCES tag_name(id),
		UNIQUE(fileId, tagNameId))`)
		if err != nil {
			slog.Error("Failed to create 'tag' table", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to create 'tag' table: %v", err)
		}
		slog.Info("Creating 'tag_name' table")
		_, err = tx.Exec(`CREATE TABLE tag_name (
  		id INTEGER PRIMARY KEY UNIQUE NOT NULL,
  		value TEXT NOT NULL UNIQUE)`)
		if err != nil {
			slog.Error("Failed to create 'tag_name' table", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to create 'tag_name' table: %v", err)
		}
		err = tx.Commit()
		if err != nil {
			slog.Error("Failed to commit setup transaction on database", "Error", err.Error())
			db.Close()
			return nil, fmt.Errorf("failed to commit setup transaction: %v", err)
		}
		f.setupMetadata(nil)
	} else {
		meta, err := f.GetMetadata()
		if err != nil {
			return nil, fmt.Errorf("failed to determine database version: %v", err)
		}
		if meta.MajorVersion != MajorVersion {
			slog.Warn("Database is a different major version, enabling safe mode", "DatabaseVersion", meta.VersionString(), "FileDbVersion", FormatVersion(MajorVersion, MinorVersion, Revision))
			f.safeMode = true
		}
		if v, found := meta.Map["experimental"]; found {
			if isDemo, ok := v.(bool); ok && isDemo {
				slog.Warn("Experimental database, issues may occour")
			}
		}
	}
	return f, nil
}
