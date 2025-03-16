/**
 * Methods for interacting with the FileDb API (v1)
 */

/**
 * Base of a API request
 */
type apiBaseResponse = {
    Code: number,
    /**
     * If 200 this value could be anything or null, if non 200 this is a string describing the error
     */
    Data: object
}

/** 
 * Represents a file via the API
*/
type apiFile = {
    Id: number
    Path: string
    Tags: string[]
    /**
     * RFC3339 format UTC time string
     */
    LastViewed: string
    /**
     * Stars from 0 (not set) to 5
     */
    Stars: number
    /**
     * File size in bytes
     */
    Size: number
}

type apiVersion = {
    String: string
    CodeName: string
    Major: number
    Minor: number
    Revision: number
}

type versionData = {
    FileDb: apiVersion
    Database: apiVersion
}

type statusData = {
    VersionInfo: versionData
    InSafeMode: boolean
}

/**
 * A MediaManager File
 */
export class MMFile {
    private file: apiFile

    /**
     * Create a new MediaManagerFile from a API file
     * @param f 
     */
    constructor(f: apiFile) {
        this.file = f
    }

    public getId(): number {
        return this.file.Id
    }

    public getPath(): string {
        return this.file.Path
    }

    public getTags(): string[] {
        return this.file.Tags
    }

    public addTag(tag: string) {
        if (this.file.Tags.includes(tag)) {
            throw new TypeError(`Tag ${tag} already exists`)
        }
        this.file.Tags.push(tag)
    }

    public removeTag(tag: string) {
        let index = this.file.Tags.indexOf(tag)
        if(index <= -1) {
            return
        }
        this.file.Tags.splice(index, 1)
    }

    public getLastViewed(): string {
        return this.file.LastViewed
    }

    public getStars(): number {
        return this.file.Stars
    }

    public setStars(v: number) {
        if (0 > v || v > 5) {
            throw new TypeError("stars cannot be greater then 5 or less then 0")
        }
        this.file.Stars = v
    }

    public getSize(): number {
        return this.file.Size
    }

    public getContentUri(): string {
        return `/api/1/content?id=${this.file.Id}`
    }

    public async getContent(): Promise<string> {
        return new Promise(async (resolve, reject) => {
            let data = await fetch(this.getContentUri())
            if(data.status != 200) {
                reject(`bad status code ${data.statusText}`)
            }
            let txt = await data.text();
            resolve(txt);
        })
    }

    public copy(): MMFile {
        // Copy the api file
        let newFile: apiFile = {
            Id: this.file.Id,
            LastViewed: this.file.LastViewed,
            Path: this.file.Path,
            Size: this.file.Size,
            Stars: this.file.Stars,
            Tags: []
        }
        this.file.Tags.forEach(v => {
            newFile.Tags.push(v)
        })
        return new MMFile(newFile)
    }
}

/**
 * Make a Async API request, on rejection the apiBaseResponse format is used, and Data is **ALWAYS** a string.
 * @param uri URI to make this request with
 * @param method Method to make this request
 * @returns string
 */
async function apiRequest(uri: string, method = "GET", body?: any, content_type?: string): Promise<apiBaseResponse> {
    let opts: RequestInit = {
        "method": method,
    }
    if (body != undefined) {
        opts.body = body
    }
    if (content_type != undefined) {
        opts.headers = {
            "Content-Type": content_type
        }
    }
    let r = await fetch(uri, opts)
    let data = await r.json() as apiBaseResponse
    return new Promise((resolve, reject) => {
        if(data.Code != 200) {
            console.error(`API Request failed: '${uri}', ${data.Code}`)
            reject(data)
            return
        }
        resolve(data)
    })
}

function indexedObjectToArray(obj: any): any[] {
    let keys = Object.keys(obj)
    let result: any[] = []
    keys.forEach(k => {
        result.push(obj[k])
    })
    return result
}

export function apiGetContentUri(id: number, update = false) {
    return `/api/1/content?id=${id}&update=${update}`
}

export function bytesToHumanReadableSize(size: number): string {
   if (size >= 1e12) {
        return `${(size/1e12).toFixed(2)} TB`
   } else if (size >= 1e9) {
        return `${(size/1e9).toFixed(2)} GB`
   } else if (size >= 1e6) {
        return `${(size/1e6).toFixed(2)} MB`
   } else if (size >= 1e3) {
        return `${(size/1e3).toFixed(2)} KB`
    }
    return `${size} B`
}

export async function apiGetFileContentType(id: number): Promise<string> {
    return new Promise(async (resolve, reject) => {
        const path = apiGetContentUri(id, false)
        let data = await fetch(path, {
            method: "HEAD"
        })
        let type = data.headers.get("content-type")
        if (type == null) {
            throw `Expected HEAD request to ${path} to yield 'content-type', but it didn't?`
        }
        resolve(type)
    })
}

/**
 * Get a file by ID
 * @param id File ID
 * @param update Should the last viewed time be updated
 * @returns File or string on rejection
 */
export async function apiGetFile(id: number, update = true): Promise<MMFile> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/files?id=${id}&update=${update}`).then((data) => {
            let files = data.Data as apiFile[]
            if (files.length == 0) {
                // This should never happen
                throw new RangeError(`Expected '/api/1/files?id=${id}' to return files or 404, returned a empty array`)
            }
            resolve(new MMFile(files[0]))
            return;
        }).catch((data: apiBaseResponse) => {
            reject(`Get file failed with ${data.Code}: ${data.Data}`)
        })
    })
}

/**
 * Update a file
 * @param file File to update to
 * @param oldFile Old file, or undefined and the file will be got using the new files ID
 * @returns 
 */
export async function apiUpdateFile(file: MMFile, oldFile?: MMFile): Promise<void> {
    return new Promise(async (resolve, reject) => {
        // Get the file if needed.
        if (oldFile == undefined) {
            oldFile = await apiGetFile(file.getId())
        }
        if (oldFile == undefined) {
            throw new TypeError("oldFile was undefined")
        }
        let form = new FormData()
        // Id is required.
        form.append("Id", file.getId().toString())
        // Now we just check whats changed.
        if (file.getStars() != oldFile.getStars()) {
            console.log("Adding stars to update")
            form.append("Stars", file.getStars().toString())
        }
        let newTags = file.getTags()
        let oldTags = oldFile.getTags()
        newTags.forEach(element => {
            if (!oldTags.includes(element)) {
                // Add it
                console.log(`Adding tag ${element} to file`)
                form.append("AddTags", element)
            }
        });
        oldTags.forEach(element => {
            if (!newTags.includes(element)) {
                // Add it
                console.log(`Removing tag ${element} to file`)
                form.append("RemTags", element)
            }
        });
        console.log(form)
        apiRequest(`/api/1/update`, "POST", form).then((_) => {
            console.log("OK")
            resolve()
        }).catch((err) => {
            reject(err)
        })
    })
}

export type searchQuery = {
    Path?: string,
    PathRe?: string,
    TagWhitelist?: string[],
    TagBlacklist?: string[]
    Index?: number,
    Count?: number,
    Sort?: "none" | "size" | "stars" | "date" | "id" | "random",
    SortReverse?: boolean
}

export async function apiSearch(qr: searchQuery): Promise<MMFile[]> {
    return new Promise((resolve, reject) => {
        let uri = "/api/1/search?"
        console.log(qr)
        if (qr.Path != undefined) {
            uri += `path=${qr.Path}&`
        }
        if (qr.PathRe != undefined) {
            uri += `path_re=${qr.PathRe}&`
        }
        if (qr.TagWhitelist != undefined && qr.TagWhitelist.length > 0) {
            console.log(`qr.TagWhitelist.length: ${qr.TagWhitelist.length}`)
            console.log(`qr.TagWhitelist: ${qr.TagWhitelist}`)
            qr.TagWhitelist.forEach(element => {
                uri += `tag_whitelist=${element}&`
            });
        }
        if (qr.TagBlacklist != undefined && qr.TagBlacklist.length > 0) {
            qr.TagBlacklist.forEach(element => {
                uri += `tag_blacklist=${element}&`
            });
        }
        if(qr.Count != undefined) {
            uri += `count=${qr.Count}&`
        }
        if(qr.Index != undefined) {
            uri += `index=${qr.Index}&`
        }
        if(qr.Sort != undefined) {
            uri += `sort=${qr.Sort}&`
        }
        if(qr.SortReverse == true) {
            uri += `sort_reverse=true&`
        }
        uri = uri.substring(0, uri.length - 1)
        apiRequest(uri).then((data) => {
            let files: MMFile[] = [];
            (data.Data as apiFile[]).forEach(element => {
                files.push(new MMFile(element))
            });
            resolve(files)
        }).catch((err) => {
            reject(err)
        })
    })
}

export async function apiDeleteTag(tag: string): Promise<void> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/deletetag?tag=${tag}`, "DELETE").then(_ => {
            resolve()
        }).catch(err => {
            reject(err)
        })
    })
}

/**
 * 
 * @deprecated Use apiSearch
 * @param index 
 * @param count 
 * @returns 
 */
export async function apiGetFileList(index: number, count: number = 50): Promise<MMFile[]> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/list?index=${index}&count=${count}`).then((data) => {
            let files: MMFile[] = [];
            (data.Data as apiFile[]).forEach(element => {
                files.push(new MMFile(element))
            });
            resolve(files)
        }).catch((err) => {
            reject(err)
        })
    })
}

export async function apiGetTags(): Promise<string[]> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/tags`).then((data) => {
            resolve(indexedObjectToArray(data.Data) as string[])
        }).catch((err) => {
            reject(err)
        })
    })
}

export async function apiAddTag(tag: string): Promise<void> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/addtag?tag=${tag}`).then((data) => {
            resolve()
        }).catch((err) => {
            reject(err)
        })
    })
}

export async function apiUpdateFileViewed(id: number): Promise<void> {
    return new Promise((resolve, reject) => {
        apiRequest(`/api/1/viewed?id=${id}`).then((data) => {
            resolve()
        }).catch((err) => {
            reject(err)
        })
    })
}

function pushToUndefined<T>(array: T[], value: T): T[] {
    for (const i in array) {
        if(array[i] === undefined) {
            array[i] = value
            return array
        }
    }
    // No undefined values, push
    console.log(`Push`, value)
    array.push(value)
    return array
}

export async function apiGetCollection(name: string, namespace="collection"): Promise<MMFile[]> {
    return new Promise(async (resolve, reject) => {
        const files = await apiSearch({
            "TagWhitelist": [`${namespace}:${name}`]
        })
        let orderedFiles: MMFile[] = []
        let noOrder: MMFile[] = []
        // If we have colindex we move the value to that index, given its a valid index, otherwise we reject with a ordering error
        for (const element of files) {
            let addedToArray = false
            for (const t of element.getTags()) {
                if(t.startsWith("colindex:")) {
                    // Get the index value
                    let n = Number(t.split(":")[1])
                    if (isNaN(n)) {
                        reject(`Expected colindex value to be a number, but it was parsed as NaN`)
                        return
                    }
                    // Make sure its not already filled
                    if (orderedFiles[n] != undefined) {
                        reject(`Duplicate colindex: ${n}`)
                        return
                    }
                    orderedFiles[n] = element
                    addedToArray = true
                }
            } 
            if(!addedToArray) {
                noOrder.push(element)
            }
        }
        for(const element of noOrder) {
            orderedFiles = pushToUndefined(orderedFiles, element)
        }
        console.log(orderedFiles)
        resolve(orderedFiles)
    })
}

/**
 * @deprecated
 * @returns 
 */
export async function apiGetRandomFile(): Promise<MMFile> {
    return new Promise(async (resolve, reject) => {
        const file = await apiRequest("/api/1/random")
        if(file.Code == 200) {
            resolve(new MMFile((file.Data as apiFile[])[0]))
            return
        } else {
            reject(file)
        }
    })
}

/**
 * @deprecated
 * @returns 
 */
export async function apiGetVersion(): Promise<versionData> {
    return new Promise(async (resolve, reject) => {
        let status = await apiGetStatus()
        console.log(status)
        console.log("apiGetVersion, ", status.VersionInfo)
        resolve(status.VersionInfo);
        return;
    })
}

export async function apiGetStatus(): Promise<statusData> {
    return new Promise(async (resolve, reject) => {
        const file = await apiRequest("/api/1/status")
        if(file.Code == 200) {
            console.log("apiGetStatus, ", file.Data)
            resolve(file.Data as statusData)
        } else {
            reject()
        }
    })
}

export function getCookie(name: string): string|null {
    let cooks = document.cookie.split(" ")
    for(const c of cooks) {
        const sp1 = c.split(";");
        const sp2 = sp1[0].split("=");
        if(sp2[0] == name) {
            return sp2[1]
        }
    }
    return null
}

type cookieOpts = {
    MaxAge?: number
    Partitioned?: boolean
    Path?: string
    Expires?: Date
    Secure?: boolean
    SameSite?: boolean
}

export function setCookie(name: string, value: string, opts?: cookieOpts) {
    let cook = `${name}=${value};`
    if(opts) {
        if(opts.MaxAge) {
            cook += `max-age=${opts.MaxAge}`
        }
        if(opts.Partitioned) {
            cook += "partitioned;"
        }
        if(opts.Expires) {
            cook += `expires=${opts.Expires.toUTCString()};`
        }
        if(opts.Secure) {
            cook += "secure;"
        }
        if(opts.SameSite) {
            cook += "samesite;"
        }
        if (opts.Path) {
            if(opts.Path == "") {
                cook += "path=/;"
            } else {
                cook += `path=${opts.Path};`
            }
        }
    } else {
        cook += "path=/;"
    }
    document.cookie = cook
}

export function deleteCookie(name: string) {
    document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 GMT`
}