# Todo Before Initial Commit
- [ ] More documentation
- [ ] Audio file support

## filedb
### file_db.go
- [ ] Remove AddFile
- [ ] Test AddFiles
  - [ ] Hash
  - [ ] Insert existing file
- [ ] Decide what to do with AddFilesWithInfo
  - [ ] Remove
  - [ ] Test
- [ ] UpdateFile
  - [ ] Test Update file with hash that already is in database
  - [ ] Tes tRemove hash
- [ ] RemoveFile
  - [ ] Remove non existant file
- [ ] SearchFile
  - [ ] By hash
  - [ ] SortMethodId
  - [ ] SortMethodLastViewed
  - [ ] SortMethodRandom
- [X] Remove SearchRegex
- [X] Remove GetAllFiles
- [X] Remove GetRandomFile
- [ ] Figure out what to do with AddInfoToFiles
  - [ ] Finish stuff
  - [ ] Remove
- [X] Remove GetVersion
- [ ] Test GetMetadata
- [ ] Remove safe mode? Make the user call `OpenForUpdate` or something similar and it just force updates
- [X] Remove VerifyUser & the `user` table
- [X] Remove AddAccount
- [ ] Test setupMetadata
- [X] Remove NewFileDbExperimental