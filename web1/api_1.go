package web1

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mediamanager/filedb"
	"net/http"
	"os"
	"strconv"
	"time"
)

type DbApi1 struct {
	db *filedb.FileDb
	lm *LoginManager
}

type apiBase struct {
	Code int
	Data any
}

type apiFile struct {
	Id         int
	Path       string
	Tags       []string
	LastViewed time.Time
	Stars      uint8
	Size       int64
}

func (a *DbApi1) writeApiError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	_ = r
	errMsg := apiBase{
		Code: code,
		Data: msg,
	}
	data, err := json.Marshal(errMsg)
	if err != nil {
		slog.Error("DbApi1.writeApiError json.Marshal failed", "Error", err.Error(), "code", code, "msg", msg)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to encode error: %v", err)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (a *DbApi1) writeApiData(w http.ResponseWriter, r *http.Request, data any) {
	jData, err := json.Marshal(apiBase{
		Code: 200,
		Data: data,
	})
	if err != nil {
		slog.Error("DbApi1.writeApiData json.Marshal failed", "Error", err.Error(), "data", fmt.Sprintf("%+v", data))
		a.writeApiData(w, r, fmt.Sprintf("Failed to encode Api data: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jData)
}

func (a *DbApi1) filesToApiFile(files []*filedb.File) []*apiFile {
	apiFiles := make([]*apiFile, len(files))
	for i, v := range files {
		apiFiles[i] = &apiFile{
			Id:         v.GetId(),
			Path:       v.GetPath(),
			Tags:       v.GetTags(),
			LastViewed: v.GetLastPlayTime(),
			Stars:      v.GetStars(),
			Size:       v.GetSize(),
		}
	}
	return apiFiles
}

func (a *DbApi1) serveFileOrApiError(w http.ResponseWriter, r *http.Request, path string) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			a.writeApiError(w, r, http.StatusNotFound, "file exists in database, but not on disk.")
			return
		}
		a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("error accessing file: %v", err))
		return
	}
	http.ServeFile(w, r, path)
}

// Serve file content
//
// Method: GET
//
// URL: /api/1/content
//
// Headers: None
//
// Query Params:
//   - id: File id
//   - update: Should the file last viewed date be updated (true/false), default: false
//
// Auth: Required
//
// Returns: File content, this is not in the API format
//
// Error on: Path/id not found, path & id used together, path / id dont exist
func (a *DbApi1) ServeFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' or 'HEAD' request")
		return
	}
	qr := r.URL.Query()
	idStr := qr.Get("id")
	if idStr == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "'id' must be set")
		return
	}
	id, err := strconv.ParseUint(idStr, 0, 64)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, "invalid id")
		return
	}
	file, err := a.db.GetFileById(int(id))
	if err != nil {
		a.writeApiError(w, r, http.StatusNotFound, "file not found")
		return
	}
	// If this is a HEAD request we never update (We don't get any content.)
	if r.Method == http.MethodGet && qr.Get("update") == "true" {
		file.MarkFileRead()
		a.db.UpdateFile(file)
	}
	// Now it depends, if this is a HEAD we just send content type, otherwise we send the file
	switch r.Method {
	case http.MethodGet:
		a.serveFileOrApiError(w, r, file.GetPath())
	case http.MethodHead:
		f, err := os.Open(file.GetPath())
		if err != nil {
			// The body is ignored, this is 'HEAD' request, but I don't really know how else to go about this.
			a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to open file: %v", err))
			return
		}
		st, err := f.Stat()
		if err != nil {
			// The body is ignored, this is 'HEAD' request, but I don't really know how else to go about this.
			a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to stat file: %v", err))
			return
		}
		w.Header().Add("accept-ranges", "bytes")
		w.Header().Add("content-ranges", fmt.Sprintf("bytes 0-%d/%d", st.Size()-1, st.Size()))
		w.Header().Add("content-length", fmt.Sprintf("%d", st.Size()))
		// DetectContentType only reads 512 bytes anyway.
		buffer := make([]byte, 512)
		_, err = f.Read(buffer)
		if err != nil {
			// The body is ignored, this is 'HEAD' request, but I don't really know how else to go about this.
			a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to read file: %v", err))
			return
		}
		contentType := http.DetectContentType(buffer)
		w.Header().Add("content-type", contentType)
	default:
		panic(fmt.Sprintf("Web1: ServeFile: Expected method to be either 'GET' or 'HEAD' (Should have been checked), Was '%s'", r.Method))
	}
}

// Serve a list of files info
//
// Method: GET
//
// URL: /api/1/files
//
// Auth: Required
//
// Headers: None
//
// Query params:
//   - file: A file path, can have multiple
//   - id: File ids, can have multiple
//
// Returns: JSON filedb.File array
//
// Error: File not found
func (a *DbApi1) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	qr := r.URL.Query()
	files := make([]*filedb.File, 0)
	for _, p := range qr["file"] {
		f, err := a.db.GetFileByPath(p)
		if err != nil {
			a.writeApiError(w, r, http.StatusNotFound, fmt.Sprintf("Failed to find file by path '%s'", p))
			return
		}
		files = append(files, f)
	}
	for _, i := range qr["id"] {
		id, err := strconv.ParseUint(i, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid 'id' value '%s'", i))
			return
		}
		f, err := a.db.GetFileById(int(id))
		if err != nil {
			a.writeApiError(w, r, http.StatusNotFound, fmt.Sprintf("Failed to find file by id '%d'", id))
			return
		}
		files = append(files, f)
	}
	a.writeApiData(w, r, a.filesToApiFile(files))
}

// Update a file
//
// Method: POST
//
// Auth: Required
//
// Headers: None
//
// Post Data:
//   - Path   : File path, cannot exist with 'Id'
//   - Id     : File Id, cannot exist with 'Path'
//   - Stars  : New star count, if not set no change occurs
//   - AddTags: Tag to add, can have multiple
//   - RemTags: Tag to remove, can have multiple
//
// Returns: Empty API response
//
// Error: 'Stars' field invalid, 'Path', or 'Id' references invalid file
func (a *DbApi1) UpdateFile(w http.ResponseWriter, r *http.Request) {
	var file *filedb.File
	path := r.PostFormValue("Path")
	idStr := r.PostFormValue("Id")
	if path != "" && idStr != "" {
		a.writeApiError(w, r, http.StatusBadRequest, "'path' and 'id' values cannot exist together")
		return
	} else if path == "" && idStr == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "missing 'path' and 'id' values")
		return
	} else if path != "" {
		var err error
		file, err = a.db.GetFileByPath(path)
		if err != nil {
			a.writeApiError(w, r, http.StatusNotFound, "file not found")
			return
		}
	} else if idStr != "" {
		id, err := strconv.ParseUint(idStr, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "invalid 'id' field")
			return
		}
		file, err = a.db.GetFileById(int(id))
		if err != nil {
			a.writeApiError(w, r, http.StatusNotFound, "file not found")
			return
		}
	}
	starsStr := r.PostForm.Get("Stars")
	if starsStr != "" {
		stars, err := strconv.ParseUint(starsStr, 0, 8)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "failed to parse 'stars' value")
			return
		}
		oldStars := file.GetStars()
		err = file.SetStars(uint8(stars))
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("failed to set 'stars' value: %v", err))
			return
		}
		slog.Info("SetStars for file", "File.Path", file.GetPath(), "File.Id", file.GetId(), "Stars", stars, "OldStars", oldStars)
	}
	add_tags := r.PostForm["AddTags"]
	for _, v := range add_tags {
		err := file.AddTag(v)
		if err != nil {
			// Not always a issue, theres a lot of reasons this could fail (I.E Another user just added the tag.)
			slog.Info("Failed to add tag to file", "File.Path", file.GetPath(), "File.Id", file.GetId(), "AddTag", v, "Tags", file.GetTags())
			a.writeApiError(w, r, http.StatusBadRequest, "Failed to add tag")
			return
		}
	}
	rem_tags := r.PostForm["RemTags"]
	for _, v := range rem_tags {
		file.RemoveTag(v)
	}
	err := a.db.UpdateFile(file)
	if err != nil {
		slog.Warn("Failed to update file", "File.Path", file.GetPath(), "File.Id", file.GetId(), "Error", err.Error())
		a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("failed to update file: %+v", err))
		return
	}
	a.writeApiData(w, r, nil)
}

// Search for files
//
// Method: GET
//
// Auth: Required
//
// Headers:
//
// Query Params:
//   - path: Search Path for any path that contains this value
//   - path_re: Regex match
//   - tag_whitelist: Whitelisted tags, multiple max exist
//   - tag_blacklist: Blacklisted tags, multiple max exist
//   - count        : Number of results to return
//   - index        : Index to start at
//   - sort         : Sort method, values are 'none', 'size' and 'stars' (In the future 'date' will be supported). Default: none
//   - sort_reverse : boolean, reverse search order (From ascending to descending) default: false
//
// Returns: filedb.File array for each found value
//
// Error: Search fails
func (a *DbApi1) SearchFile(w http.ResponseWriter, r *http.Request) {
	search := &filedb.SearchQuery{
		Path:          "",
		PathRe:        "",
		WhitelistTags: make([]string, 0),
		BlacklistTags: make([]string, 0),
		Index:         0,
		Count:         50,
	}
	qr := r.URL.Query()
	// Setup values
	search.Path = qr.Get("path")
	search.PathRe = qr.Get("path_re")
	search.WhitelistTags = qr["tag_whitelist"]
	search.BlacklistTags = qr["tag_blacklist"]
	search.SortReverse = qr.Get("sort_reverse") == "true"
	// Index, Count, sort need parsing
	idxStr := qr.Get("index")
	cntStr := qr.Get("count")
	searchStr := qr.Get("sort")
	if idxStr != "" {
		var err error
		index, err := strconv.ParseUint(idxStr, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "invalid 'index' value")
			return
		}
		search.Index = int64(index)
	}
	if cntStr != "" {
		var err error
		count, err := strconv.ParseUint(cntStr, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "invalid 'index' value")
			return
		}
		// Max count value (For performance reasons) is 200
		if count > 200 {
			a.writeApiError(w, r, http.StatusBadRequest, "'count' cannot exceed 200")
			return
		}
		search.Count = int64(count)
	}
	switch searchStr {
	case "none", "":
		search.SortBy = filedb.SortMethodNone
	case "size":
		search.SortBy = filedb.SortMethodSize
	case "stars":
		search.SortBy = filedb.SortMethodStars
	case "date":
		search.SortBy = filedb.SortMethodLastViewed
	case "id":
		search.SortBy = filedb.SortMethodId
	case "random":
		search.SortBy = filedb.SortMethodRandom
	default:
		a.writeApiError(w, r, http.StatusBadRequest, "invalid 'sort' value, must be one of 'none, 'size', 'stars', 'date' or 'id'")
		return
	}
	files, err := a.db.SearchFile(search)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("Search failed: %v", err))
		return
	}
	a.writeApiData(w, r, a.filesToApiFile(files))
}

// Delete a tag
//
// Method: DELETE
//
// Auth: Required
//
// Headers:
//
// Query Param:
//   - tag: Tag to delete
//
// Returns: Empty API Response
//
// Error: None
func (a *DbApi1) DeleteTag(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "missing 'tag' query parameter")
		return
	}
	err := a.db.RemoveTag(tag)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to remove tag '%s'", tag))
		return
	}
	a.writeApiData(w, r, nil)
}

// Delete a file
//
// Method: DELETE
//
// Auth: Required
//
// Headers:
//
// Query Param:
//   - id: File ID to delete
//
// Returns: Empty API Response
//
// Error: None
func (a *DbApi1) DeleteFile(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "missing 'id' query parameter")
		return
	}
	id, err := strconv.ParseUint(idStr, 0, 64)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, "'id' is invalid")
		return
	}
	f, err := a.db.GetFileById(int(id))
	if err != nil {
		a.writeApiError(w, r, http.StatusNotFound, "File not found")
		return
	}
	err = a.db.RemoveFile(f)
	if err != nil {
		a.writeApiError(w, r, http.StatusInternalServerError, "Failed to remove file")
		return
	}
	a.writeApiData(w, r, nil)
}

// Deprecated: Use 'SearchFile' with a empty query and 'Count' and 'Index' values set
//
// Get file list from a start to start+count index
//
// Method: GET
//
// Auth: Required
//
// Headers: None
//
// Query Params:
//   - count: Number of files to get, Default: 50
//   - index: Index to start at
//
// Returns: filedb.File array
//
// Error: count or index invalid
func (a *DbApi1) GetFileList(w http.ResponseWriter, r *http.Request) {
	index := uint64(0)
	count := uint64(50)
	qr := r.URL.Query()
	idxStr := qr.Get("index")
	cntStr := qr.Get("count")
	if idxStr != "" {
		var err error
		index, err = strconv.ParseUint(idxStr, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "invalid 'index' value")
			return
		}
	}
	if cntStr != "" {
		var err error
		count, err = strconv.ParseUint(cntStr, 0, 64)
		if err != nil {
			a.writeApiError(w, r, http.StatusBadRequest, "invalid 'index' value")
			return
		}
	}
	if count > 200 {
		a.writeApiError(w, r, http.StatusBadRequest, "'count' cannot exceed 200")
		return
	}
	files, err := a.db.SearchFile(&filedb.SearchQuery{
		Count: int64(count),
		Index: int64(index),
	})
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("Query failed '%v'", err))
		return
	}
	a.writeApiData(w, r, a.filesToApiFile(files))
}

// Get all tags
//
// Method: GET
//
// Auth: Required
//
// Headers: None
//
// Query Params: None
//
// Returns: String array
//
// Error: None
func (a *DbApi1) GetAllTags(w http.ResponseWriter, r *http.Request) {
	a.writeApiData(w, r, a.db.GetAllTags())
}

// # Add a tag
//
// Method: POST
//
// Auth: Required
//
// Headers: None
//
// Query Params: tag
//
// Returns: None
//
// Error: Tag already exists
func (a *DbApi1) AddTag(w http.ResponseWriter, r *http.Request) {
	qr := r.URL.Query()
	tag := qr.Get("tag")
	if tag == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "Expected 'tag' param")
		return
	}
	_, err := a.db.AddTag(tag)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, fmt.Sprintf("failed to add tag: %v", err))
		return
	}
	a.writeApiData(w, r, nil)
}

// Update files last viewed time
//
// Method: GET
//
// Auth: Required
//
// Headers: None
//
// Query Params: id
//
// Returns: None
//
// Error: Tag already exists
func (a *DbApi1) UpdateFileDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	qr := r.URL.Query()
	idStr := qr.Get("id")
	if idStr == "" {
		a.writeApiError(w, r, http.StatusBadRequest, "'id' must be set")
		return
	}
	id, err := strconv.ParseUint(idStr, 0, 64)
	if err != nil {
		a.writeApiError(w, r, http.StatusBadRequest, "invalid id")
		return
	}
	file, err := a.db.GetFileById(int(id))
	if err != nil {
		a.writeApiError(w, r, http.StatusNotFound, "file not found")
		return
	}
	file.MarkFileRead()
	err = a.db.UpdateFile(file)
	if err != nil {
		a.writeApiData(w, r, fmt.Sprintf("failed to update file: %v", err))
		return
	}
	a.writeApiData(w, r, nil)
}

// Deprecated: Use SearchFile with Random
//
// # Get random file
//
// Method: GET
//
// Auth: Required
//
// Headers: None
//
// Query Params: None
//
// Returns: File
func (a *DbApi1) GetRandomFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	file, err := a.db.SearchFile(&filedb.SearchQuery{
		Count:  1,
		SortBy: filedb.SortMethodRandom,
	})
	if err != nil {
		a.writeApiError(w, r, http.StatusNotFound, "file not found")
		return
	}
	a.writeApiData(w, r, a.filesToApiFile(file))
}

type versionData struct {
	String   string
	CodeName string
	Major    int
	Minor    int
	Revision int
}

type VersionInfo struct {
	Database versionData
	FileDb   versionData
	UpToDate bool
}

// Deprecated: Use GetStatus
func (a *DbApi1) GetVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	meta, err := a.db.GetMetadata()
	if err != nil {
		a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to get version from database: %v", err))
		return
	}
	a.writeApiData(w, r, VersionInfo{
		FileDb: versionData{
			String:   filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision),
			CodeName: filedb.VersionCodeName,
			Major:    filedb.MajorVersion,
			Minor:    filedb.MinorVersion,
			Revision: filedb.Revision,
		},
		Database: versionData{
			String:   filedb.FormatVersion(meta.MajorVersion, meta.MinorVersion, meta.RevisionVersion),
			Major:    meta.MajorVersion,
			Minor:    meta.MinorVersion,
			Revision: meta.RevisionVersion,
			CodeName: meta.VersionCodeName,
		},
	})
}

type statusData struct {
	String   string
	CodeName string
	Major    int
	Minor    int
	Revision int
	Metadata map[string]any
}

type statusInfoVersion struct {
	Database statusData
	FileDb   statusData
}

type StatusInfo struct {
	VersionInfo statusInfoVersion
	InSafeMode  bool
}

func (a *DbApi1) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	meta, err := a.db.GetMetadata()
	if err != nil {
		a.writeApiError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to get version from database: %v", err))
		return
	}
	a.writeApiData(w, r, StatusInfo{
		VersionInfo: statusInfoVersion{
			FileDb: statusData{
				String:   filedb.FormatVersion(filedb.MajorVersion, filedb.MinorVersion, filedb.Revision),
				CodeName: filedb.VersionCodeName,
				Major:    filedb.MajorVersion,
				Minor:    filedb.MinorVersion,
				Revision: filedb.Revision,
			},
			Database: statusData{
				String:   filedb.FormatVersion(meta.MajorVersion, meta.MinorVersion, meta.RevisionVersion),
				Major:    meta.MajorVersion,
				Minor:    meta.MinorVersion,
				Revision: meta.RevisionVersion,
				CodeName: meta.VersionCodeName,
				Metadata: meta.Map,
			},
		},
		InSafeMode: a.db.IsSafeMode(),
	})
}

type apiAccount struct {
	Id               int
	Username         string
	LoggedInAt       int64
	LoggedInAtString string
	IpConnectedFrom  string
	UserAgent        string
}

func (a *DbApi1) GetAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	if a.lm == nil {
		a.writeApiError(w, r, 404, "Not using authorization")
		return
	}
	data := make([]apiAccount, 0)
	for k, v := range a.lm.cookies {
		data = append(data, apiAccount{
			Id:               k,
			Username:         v.AccountName,
			LoggedInAt:       v.At.UTC().Unix(),
			LoggedInAtString: v.At.UTC().UTC().Format(time.RFC3339),
			IpConnectedFrom:  v.IpFrom,
			UserAgent:        v.UserAgent,
		})
	}
	a.writeApiData(w, r, data)
}

func (a *DbApi1) RemoveCookie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	if a.lm == nil {
		a.writeApiError(w, r, 404, "Not using authorization")
		return
	}
	qr := r.URL.Query()
	idStr := qr.Get("id")
	if idStr == "" {
		a.writeApiError(w, r, 400, "'id' query parameter must exist")
		return
	}
	id, err := strconv.ParseInt(idStr, 0, 64)
	if err != nil {
		a.writeApiError(w, r, 400, fmt.Sprintf("Bad 'id' query parameter: %v", err))
		return
	}
	a.lm.RemoveCookie(int(id))
	a.writeApiData(w, r, nil)
}

type apiLoginAttempt struct {
	Success          bool
	ErrorMessage     string
	Username         string
	LoggedInAt       int64
	LoggedInAtString string
	IpConnectedFrom  string
	UserAgent        string
}

func (a *DbApi1) LoginAttempts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeApiError(w, r, http.StatusMethodNotAllowed, "Must be a 'GET' request")
		return
	}
	if a.lm == nil {
		a.writeApiError(w, r, 404, "Not using authorization")
		return
	}
	data := make([]apiLoginAttempt, 0)
	for _, v := range a.lm.loginAttempts {
		data = append(data, apiLoginAttempt{
			Username:         v.AccoutName,
			LoggedInAt:       v.At.UTC().Unix(),
			LoggedInAtString: v.At.UTC().UTC().Format(time.RFC3339),
			IpConnectedFrom:  v.Ip,
			Success:          v.Success,
			ErrorMessage:     v.ReasonFailed,
			UserAgent:        v.UserAgent,
		})
	}
	a.writeApiData(w, r, data)
}

func NewFileDbApi(db *filedb.FileDb, mux *http.ServeMux, lm *LoginManager) *DbApi1 {
	api := &DbApi1{
		db: db,
		lm: lm,
	}
	mux.HandleFunc("/api/1/content", api.ServeFile)
	mux.HandleFunc("/api/1/files", api.GetFileInfo)
	mux.HandleFunc("/api/1/update", api.UpdateFile)
	mux.HandleFunc("/api/1/search", api.SearchFile)
	mux.HandleFunc("/api/1/deletetag", api.DeleteTag)
	mux.HandleFunc("/api/1/deletefile", api.DeleteFile)
	mux.HandleFunc("/api/1/tags", api.GetAllTags)
	mux.HandleFunc("/api/1/addtag", api.AddTag)
	mux.HandleFunc("/api/1/viewed", api.UpdateFileDate)
	mux.HandleFunc("/api/1/status", api.GetStatus)
	// Test stuff
	mux.HandleFunc("/api/1/cookies", api.GetAccounts)
	mux.HandleFunc("/api/1/removecookie", api.RemoveCookie)
	mux.HandleFunc("/api/1/loginattempts", api.LoginAttempts)
	// Deprecated stuff below.
	mux.HandleFunc("/api/1/list", api.GetFileList)
	mux.HandleFunc("/api/1/random", api.GetRandomFile)
	//mux.HandleFunc("/api/1/version", api.GetVersion)
	return api
}
