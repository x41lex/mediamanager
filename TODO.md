# TODO
## Most Important
- [ ] Change some import stuff
  - [ ] Only hash if the path has already been confirmed to be unique.
- [ ] Database improvments.
  - [ ] Change to `STRICT` on all tables
  - [X] file
    - [X] CHECK constraint to ensure `hash` is either 32 bytes long of hex characters, or NULL
    - [X] CHECK constraint to ensure `stars` is 0-5
    - [X] Move lastViewed to a integer.
  - [X] tag
    - [X] Ensure entire row is unique
    - [X] Forgien key for fileId=>file.id, tagNameId=>tag_name.id
  - [X] Ensure `PRAGMA foreign_keys = true`	is ran on connection to database.
  - [ ] Checks
    - [ ] `PRAGMA schema.foreign_key_check` 
    - [ ] `PRAGMA schema.integrity_check`
- [ ] Figure out of the database locks are needed.
- [ ] Research
  - [ ] `PRAGMA optimize`
## PeopleWatching - 4.0 
- [ ] Create table `user`, `Id INTEGER`, `Username TEXT`, `PasswordHash TEXT` (Scrypt)
- [ ] Remove `stars` and `lastViewed` from `file`
- [ ] Create new table `file_info`, `UserId` (user.Id), `fileId` (file.Id), stars, lastViewed. These values are only added for stuff that exists
## Features
- [X] Split `main.go` into a bunch of smaller files.
- [ ] Ensure files are only displayed if they exist, but don't remove them from the database if they don't unless Prune is called
- [X] Web Add/Remove Tag
  - [X] Existing tags are shown, can be clicked to enable/disabled them
  - [X] Or you can input a new tag
- [X] Fix redirect not copying query params
- [X] Merge Search & File list (Empty search replaces FileList)
- [ ] Add missing method checks of API
- [ ] Display names of files, through 'name:*' tag
- [ ] Account Permissions
  - [ ] Restrict adding/removing tags
  - [ ] Restrict settings starts
  - [ ] Restric files shown via some meta tag (Maybe restricted:true)


- [ ] Logging
  - [X] filedb
  - [ ] web1/api
  - [ ] web1/auth
- [ ] Testing
  - [ ] filedb
  - [ ] web1

- [X] Try and cut down on requests for files, for instance instead of having to search for each file being able to bundle 'tag_name' searches for files would be good (I think)
- [X] Rework search
- [ ] Censor Sensitive logging info (File names, ID, Update values)

Figure out when files should count as 'last seen', when a file is imported should it be counted as 'last seen' on add?


# Testing

## file_db.go
### removeFileTag
File not found, should remove 0 tags and return nil
```go
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
    // !! HERE !!
		// Doesn't exist - do nothing
		return nil
	}
```
Test with
```go
removeFileTag(testTx, 0, "who_cares")
```
Tag deleted already race condition
```go
slog.Info("Executing DELETE", "Query", "DELETE FROM tag WHERE fileId=? AND tagNameId=?", "QueryArgs", []any{fileId, tagId})
	_, err = tx.Exec("DELETE FROM tag WHERE fileId=? AND tagNameId=?", fileId, tagId)
	if err != nil {
		// This *could* fail if we just deleted from the table
		slog.Error("Failed to delete from 'tag' table", "Query", "DELETE FROM tag WHERE fileId=? AND tagNameId=?", "QueryArgs", []any{fileId, tagId}, "Error", err.Error())
		tx.Rollback()
		return fmt.Errorf("failed to insert into tag table: %v", err)
	}
```
I don't actually think this is possible because its in a transaction, read into how transactions work.


# Possible ways to mess with the database
## RemoveTag 
Begin a `AddFile` call, the `AddFile` has a tag `tag_1`, at the same time call `RemoveTag` on `tag_1`, right before the commit (See below) we could call `RemoveTag`, removing the tag from the database, but because the transaction doesn't see the tag as deleted it commits with a now invalid tag ID, messing with the database and requiring manual fixes, this could also lead to weird tags being applied to things.
```go
// filedb/file_db.go:AddFile
// Add each file tag, adding to tag_name as needed
for _, v := range f.tags {
	err = d.addFileTag(tx, int(fileId), v)
	if err != nil {
		// This is actually concerning as this shouldn't happen.
		slog.Warn("Aborting AddFile after failing to add tag", "Error", err.Error(), "Tag", v)
		tx.Rollback()
		return err
	}
}
// Call 'RemoveTag' here
// Now we can commit
err = tx.Commit()
if err != nil {
	// This is concerning because we have the database locked & no other inset/delete/update operations should be happening.
	slog.Warn("Failed to commit AddFile", "Error", err.Error(), "File", *f)
	return fmt.Errorf("transaction failed to commit: %v", err)
}
```
This requires a lock on RemoveTag, but that hampers performance. I don't know of any other solution though.

The same is true for UpdateFile
```go
// Figure out what tags we need to remove & what tags we need to add
for _, v := range f.tags {
	if slices.Index(oldFile.tags, v) == -1 {
		slog.Debug("Adding new file tag", "FileId", f.id, "Tag", v)
		// Does't exist in old file, add it
		err = d.addFileTag(tx, f.id, v)
		if err != nil {
			slog.Warn("Aborting UpdateFile after failing to add tag", "Error", err.Error(), "Tag", v, "File.Id", fid)
			tx.Rollback()
			return err
		}
	}
}
// ... Trimmed removing tags ...
// Call RemoveTag here
err = tx.Commit()
if err != nil {
	// This could happen because
	slog.Warn("Failed to commit UpdateFile", "Error", err.Error(), "File", *f)
	return fmt.Errorf("transaction failed to commit: %v", err)
}
```

- [X] Metadata 

Like tags, but with special values & default values, these are typically hidden

- [ ] Raspberry Pi / Remote support / 'Remote' mode
  - [ ] Upload files
    - [ ] Change metadata to work with us better
  - [ ] FileDb directory format
  - [ ] Thumbnails
  - [ ] Partitions, for instance account 'TestAccount' has special access to files with special tags / metadata


## To Test
- [ ] For sure rewrite a bunch of testing.
### /.go
- [ ] main.go:Verify
- [ ] main.go:IsDatabaseOperation
- [ ] main.go:dbSelect
- [ ] main.go:dbOpDry
- [ ] main.go:dbOp
- [ ] main.go:main
- [ ] util.go:GetRandomAccount
- [ ] util.go:copyFile
### filedb
- [ ] file_db.go:SearchFile
  - [ ] Multiple whitelist tags
  - [ ] Multiple blacklist tags
  - [X] Path Regex
  - [X] Sort by 
    - [X] none
    - [X] size
    - [X] stars
  - [X] Sort Direction
    - [X] ASC
    - [X] DESC
- [X] file_db.go:AddTag
- [X] sql_regex_exit.go:sqlRegex
- [X] util.go:isValidFile
### web1
- [ ] api.go
- [ ] app.go
- [ ] auth.go
- [ ] drop.go
- [ ] util.go


## Previously Completed
- [X] Hashing
- [X] Faster importing algo. (Bypassed, you can just not including hashing / Getting size)
- [X] Rewrite tests for file_db_test.go
- [X] Allow hashes to be NULL for speed, maybe only hash if requested? (Maybe `AddFiles` should have a option, or maybe you just need to hash it first and theres like a `AddFilesWithInfo`)
- [X] Arguments for not hashing files / not adding file size during import
- [X] Arguments for adding hash / size to files that don't currently have it