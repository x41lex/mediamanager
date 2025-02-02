/**
 * Methods for interacting with the FileDb API (v1)
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
/**
 * A MediaManager File
 */
export class MMFile {
    /**
     * Create a new MediaManagerFile from a API file
     * @param f
     */
    constructor(f) {
        this.file = f;
    }
    getId() {
        return this.file.Id;
    }
    getPath() {
        return this.file.Path;
    }
    getTags() {
        return this.file.Tags;
    }
    addTag(tag) {
        if (this.file.Tags.includes(tag)) {
            throw new TypeError(`Tag ${tag} already exists`);
        }
        this.file.Tags.push(tag);
    }
    removeTag(tag) {
        let index = this.file.Tags.indexOf(tag);
        if (index <= -1) {
            return;
        }
        this.file.Tags.splice(index, 1);
    }
    getLastViewed() {
        return this.file.LastViewed;
    }
    getStars() {
        return this.file.Stars;
    }
    setStars(v) {
        if (0 > v || v > 5) {
            throw new TypeError("stars cannot be greater then 5 or less then 0");
        }
        this.file.Stars = v;
    }
    getSize() {
        return this.file.Size;
    }
    getContentUri() {
        return `/api/1/content?id=${this.file.Id}`;
    }
    copy() {
        // Copy the api file
        let newFile = {
            Id: this.file.Id,
            LastViewed: this.file.LastViewed,
            Path: this.file.Path,
            Size: this.file.Size,
            Stars: this.file.Stars,
            Tags: []
        };
        this.file.Tags.forEach(v => {
            newFile.Tags.push(v);
        });
        return new MMFile(newFile);
    }
}
/**
 * Make a Async API request, on rejection the apiBaseResponse format is used, and Data is **ALWAYS** a string.
 * @param uri URI to make this request with
 * @param method Method to make this request
 * @returns string
 */
function apiRequest(uri, method = "GET", body, content_type) {
    return __awaiter(this, void 0, void 0, function* () {
        let opts = {
            "method": method,
        };
        if (body != undefined) {
            opts.body = body;
        }
        if (content_type != undefined) {
            opts.headers = {
                "Content-Type": content_type
            };
        }
        let r = yield fetch(uri, opts);
        let data = yield r.json();
        return new Promise((resolve, reject) => {
            if (data.Code != 200) {
                console.error(`API Request failed: '${uri}', ${data.Code}`);
                reject(data);
                return;
            }
            resolve(data);
        });
    });
}
function indexedObjectToArray(obj) {
    let keys = Object.keys(obj);
    let result = [];
    keys.forEach(k => {
        result.push(obj[k]);
    });
    return result;
}
export function apiGetContentUri(id, update = false) {
    return `/api/1/content?id=${id}&update=${update}`;
}
export function bytesToHumanReadableSize(size) {
    if (size >= 1e12) {
        return `${(size / 1e12).toFixed(2)} TB`;
    }
    else if (size >= 1e9) {
        return `${(size / 1e9).toFixed(2)} GB`;
    }
    else if (size >= 1e6) {
        return `${(size / 1e6).toFixed(2)} MB`;
    }
    else if (size >= 1e3) {
        return `${(size / 1e3).toFixed(2)} KB`;
    }
    return `${size} B`;
}
export function apiGetFileContentType(id) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            const path = apiGetContentUri(id, false);
            let data = yield fetch(path, {
                method: "HEAD"
            });
            let type = data.headers.get("content-type");
            if (type == null) {
                throw `Expected HEAD request to ${path} to yield 'content-type', but it didn't?`;
            }
            resolve(type);
        }));
    });
}
/**
 * Get a file by ID
 * @param id File ID
 * @param update Should the last viewed time be updated
 * @returns File or string on rejection
 */
export function apiGetFile(id, update = true) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/files?id=${id}&update=${update}`).then((data) => {
                let files = data.Data;
                if (files.length == 0) {
                    // This should never happen
                    throw new RangeError(`Expected '/api/1/files?id=${id}' to return files or 404, returned a empty array`);
                }
                resolve(new MMFile(files[0]));
                return;
            }).catch((data) => {
                reject(`Get file failed with ${data.Code}: ${data.Data}`);
            });
        });
    });
}
/**
 * Update a file
 * @param file File to update to
 * @param oldFile Old file, or undefined and the file will be got using the new files ID
 * @returns
 */
export function apiUpdateFile(file, oldFile) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            // Get the file if needed.
            if (oldFile == undefined) {
                oldFile = yield apiGetFile(file.getId());
            }
            if (oldFile == undefined) {
                throw new TypeError("oldFile was undefined");
            }
            let form = new FormData();
            // Id is required.
            form.append("Id", file.getId().toString());
            // Now we just check whats changed.
            if (file.getStars() != oldFile.getStars()) {
                console.log("Adding stars to update");
                form.append("Stars", file.getStars().toString());
            }
            let newTags = file.getTags();
            let oldTags = oldFile.getTags();
            newTags.forEach(element => {
                if (!oldTags.includes(element)) {
                    // Add it
                    console.log(`Adding tag ${element} to file`);
                    form.append("AddTags", element);
                }
            });
            oldTags.forEach(element => {
                if (!newTags.includes(element)) {
                    // Add it
                    console.log(`Removing tag ${element} to file`);
                    form.append("RemTags", element);
                }
            });
            console.log("Sending update");
            apiRequest(`/api/1/update`, "POST", form).then((_) => {
                console.log("OK");
                resolve();
            }).catch((err) => {
                reject(err);
            });
        }));
    });
}
export function apiSearch(qr) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            let uri = "/api/1/search?";
            console.log(qr);
            if (qr.Path != undefined) {
                uri += `path=${qr.Path}&`;
            }
            if (qr.PathRe != undefined) {
                uri += `path_re=${qr.PathRe}&`;
            }
            if (qr.TagWhitelist != undefined && qr.TagWhitelist.length > 0) {
                console.log(`qr.TagWhitelist.length: ${qr.TagWhitelist.length}`);
                console.log(`qr.TagWhitelist: ${qr.TagWhitelist}`);
                qr.TagWhitelist.forEach(element => {
                    uri += `tag_whitelist=${element}&`;
                });
            }
            if (qr.TagBlacklist != undefined && qr.TagBlacklist.length > 0) {
                qr.TagBlacklist.forEach(element => {
                    uri += `tag_blacklist=${element}&`;
                });
            }
            if (qr.Count != undefined) {
                uri += `count=${qr.Count}&`;
            }
            if (qr.Index != undefined) {
                uri += `index=${qr.Index}&`;
            }
            if (qr.Sort != undefined) {
                uri += `sort=${qr.Sort}&`;
            }
            if (qr.SortReverse == true) {
                uri += `sort_reverse=true&`;
            }
            uri = uri.substring(0, uri.length - 1);
            apiRequest(uri).then((data) => {
                let files = [];
                data.Data.forEach(element => {
                    files.push(new MMFile(element));
                });
                resolve(files);
            }).catch((err) => {
                reject(err);
            });
        });
    });
}
export function apiDeleteTag(tag) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/deletetag?tag=${tag}`, "DELETE").then(_ => {
                resolve();
            }).catch(err => {
                reject(err);
            });
        });
    });
}
/**
 *
 * @deprecated Use apiSearch
 * @param index
 * @param count
 * @returns
 */
export function apiGetFileList(index, count = 50) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/list?index=${index}&count=${count}`).then((data) => {
                let files = [];
                data.Data.forEach(element => {
                    files.push(new MMFile(element));
                });
                resolve(files);
            }).catch((err) => {
                reject(err);
            });
        });
    });
}
export function apiGetTags() {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/tags`).then((data) => {
                resolve(indexedObjectToArray(data.Data));
            }).catch((err) => {
                reject(err);
            });
        });
    });
}
export function apiAddTag(tag) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/addtag?tag=${tag}`).then((data) => {
                resolve();
            }).catch((err) => {
                reject(err);
            });
        });
    });
}
export function apiUpdateFileViewed(id) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            apiRequest(`/api/1/viewed?id=${id}`).then((data) => {
                resolve();
            }).catch((err) => {
                reject(err);
            });
        });
    });
}
function pushToUndefined(array, value) {
    for (const i in array) {
        if (array[i] === undefined) {
            array[i] = value;
            return array;
        }
    }
    // No undefined values, push
    console.log(`Push`, value);
    array.push(value);
    return array;
}
export function apiGetCollection(name, namespace = "collection") {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            const files = yield apiSearch({
                "TagWhitelist": [`${namespace}:${name}`]
            });
            let orderedFiles = [];
            let noOrder = [];
            // If we have colindex we move the value to that index, given its a valid index, otherwise we reject with a ordering error
            for (const element of files) {
                let addedToArray = false;
                for (const t of element.getTags()) {
                    if (t.startsWith("colindex:")) {
                        // Get the index value
                        let n = Number(t.split(":")[1]);
                        if (isNaN(n)) {
                            reject(`Expected colindex value to be a number, but it was parsed as NaN`);
                            return;
                        }
                        // Make sure its not already filled
                        if (orderedFiles[n] != undefined) {
                            reject(`Duplicate colindex: ${n}`);
                            return;
                        }
                        orderedFiles[n] = element;
                        addedToArray = true;
                    }
                }
                if (!addedToArray) {
                    noOrder.push(element);
                }
            }
            for (const element of noOrder) {
                orderedFiles = pushToUndefined(orderedFiles, element);
            }
            console.log(orderedFiles);
            resolve(orderedFiles);
        }));
    });
}
/**
 * @deprecated
 * @returns
 */
export function apiGetRandomFile() {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            const file = yield apiRequest("/api/1/random");
            if (file.Code == 200) {
                resolve(new MMFile(file.Data[0]));
                return;
            }
            else {
                reject(file);
            }
        }));
    });
}
export function apiGetVersion() {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            const file = yield apiRequest("/api/1/version");
            if (file.Code == 200) {
                resolve(file.Data);
                return;
            }
            else {
                reject();
            }
        }));
    });
}
