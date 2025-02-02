/**
 * *Really* shitty code, super messy cause I was experimenting (Also javascript & typescript let you get away with some nasty shit, idk its a language that makes you want to work like shit.)
 * 
 * @todo Error handling popups.
 */
import { MMFile, apiGetFile, apiGetContentUri, apiGetFileContentType, apiUpdateFile, apiGetTags, apiUpdateFileViewed, apiAddTag, apiSearch, apiGetCollection, apiGetRandomFile } from "./api.js"

/**
 * Data of a tag
 */
type tagData = {
    Element: HTMLAnchorElement,
    Value: string,
    IsFileTag: boolean,
    IsSelected: boolean
}

/**
 * Id of the current file, or -1 if we're using collection.
 * @todo Replace with file, or just remove.
 */
var fileId = -1

/**
 * A list of all tags
 */
var tagList: tagData[] = []

/**
 * Current collection this file is part of (If it is, otherwise its empty)
 */
var collection: MMFile[] = []

/**
 * Index in the collection this file is
 */
var colIndex = 0

/**
 * Loads a collection by name
 * @param name Collection name (Without the collection: part)
 * @param showFull Should the collection be showed now
 * @param author Replace using 'collection:' with using 'author:'
 * @todo Remove globals, add whatever error handling were going to use
 */
async function loadCollection(name: string, showFull = false, author = false) {
    if (collection.length > 0) {
        // If the collection isn't empty its already been loaded
        console.error("Attempted to reload loaded collection")
        return
    }
    // Search the collection.s
    if (author) {
        try {
            collection = await apiGetCollection(name, "author")
        } catch(e) {
            alert(`Failed to get collection: ${e}`)
            return
        }
    } else {
        try {
            collection = await apiGetCollection(name)
        } catch(e) {
            alert(`Failed to get collection: ${e}`)
            return
        }
    }
    // If we're showing the collection just do that now
    if (showFull) {
        await showFullCollection()
        return
    }
    const colIndexA = document.getElementById("col_index") as HTMLAnchorElement
    // Find the file in the collection
    for (let i = 0; i < collection.length; i++) {
        if (collection[i].getId() == fileId) {
            colIndex = i
            colIndexA.text = `${i}`
            break
        }
    }
    // Not found?
    if (colIndex == -1) {
        alert(`File doesn't exist in collection?`)
        collection = []
        return
    }
    // Set the button actions.
    const colPrev = document.getElementById("col_back") as HTMLButtonElement
    const colNext = document.getElementById("col_next") as HTMLButtonElement
    const showAll = document.getElementById("col_show_all") as HTMLButtonElement
    colPrev.onclick = () => {
        if (colIndex > 0) {
            colIndex -= 1
            // Change our file
            changeFile(collection[colIndex])
        }
    }
    colNext.onclick = () => {
        if (colIndex < collection.length) {
            colIndex += 1
            // Change our file
            changeFile(collection[colIndex])
        }
    }
    showAll.onclick = showFullCollection
    return
}

/**
 * Show the full collection in the 'collection' global
 * @returns 
 */
async function showFullCollection() {
    // No collection = bad
    if (collection.length == 0) {
        alert("No collection loaded")
        return
    }
    // SHow the collection controls
    (document.getElementById("col_controls") as HTMLDivElement).hidden = false;
    // Setup the show names buttons
    (document.getElementById("col_show_names") as HTMLInputElement).onchange = (s) => {
        const state = (s.target as HTMLInputElement).checked
        const elements = document.getElementsByClassName("file-name")
        for (let index = 0; index < elements.length; index++) {
            const e = elements[index] as HTMLHeadingElement;
            e.hidden = !state
        }
    };
    // Get collection name
    let cName = ""
    for (const tag of collection[0].getTags()) {
        console.log(tag)
        if (tag.startsWith("collection:")) {
            cName = tag.split(":")[1]
            break
        }
    }
    // No name = bad.
    if (cName == "") {
        alert("Failed to get collection name")
        return
    }
    // Set it
    const path = document.getElementById("path") as HTMLHeadElement
    path.innerText = `Collection: ${cName}`
    // Hide controls
    const controls = document.getElementById("controls") as HTMLDivElement
    controls.hidden = true
    // Remove current media
    const container = document.getElementById("mediaContainer") as HTMLDivElement
    container.innerHTML = ""
    // Change URL
    window.history.pushState("", "")
    window.history.replaceState("", "", `/file?collection=${cName}`)
    // Add each one, making sure they are in order.
    collection.forEach(element => {
        let d = createFileElement(element, true, true)
        container.appendChild(d)
    });
}

/**
 * Change the current displayed file
 * @param file New file to display
 */
function changeFile(file: MMFile) {
    fileId = file.getId()
    // Add a new thing
    setupFileSource(file )
    // Remove the old one
    const container = document.getElementById("mediaContainer") as HTMLDivElement
    // Remove old nodes
    while (container.childNodes.length != 1) {
        container.removeChild(container.childNodes[0])
    }
    setupFile(file, false)
    // Change URL
    window.history.pushState("", "")
    window.history.replaceState("", "", `/file?id=${fileId}`)
}

/**
 * Setup the tags of this file
 * @param file File to set the tags up for
 */
async function setupTags(file: MMFile) {
    const tags = await apiGetTags()
    const fileTags = file.getTags()
    tagList = []
    tags.forEach(t => {
        // Create element for it
        let element = document.createElement("a")
        element.classList.add("tag")
        let sp = t.split(":")
        console.log(sp)
        if (sp.length > 1) {
            if (sp[0] == "author") {
                element.classList.add("author")
            } else if (sp[0] == "collection") {
                element.classList.add("collection")
            } else {
                element.classList.add('metadata')
            }
            element.text = sp[1]
        } else {
            element.text = t
        }
        tagList.push({
            Element: element,
            IsFileTag: fileTags.includes(t),
            IsSelected: false,
            Value: t
        })
    });
}

/**
 * Re-render the tags on the file, call on any updates
 */
async function renderTags() {
    const fileTagE = document.getElementById("tags") as HTMLSpanElement
    const allTagE = document.getElementById("all_tags") as HTMLSpanElement
    const authorTagE = document.getElementById("author_tags") as HTMLSpanElement
    const authorDiv = document.getElementById("author_div") as HTMLDivElement
    const collectionTagE = document.getElementById("col_tags") as HTMLSpanElement
    const colDiv = document.getElementById("collection_div") as HTMLDivElement
    const showAuthors = (document.getElementById("show_authors") as HTMLInputElement).checked
    const showCollections = (document.getElementById("show_collections") as HTMLInputElement).checked
    // Clear tag
    fileTagE.innerHTML = ""
    allTagE.innerHTML = ""
    authorTagE.innerHTML = ""
    collectionTagE.innerHTML = ""
    tagList.forEach(td => {
        if (td.Value.includes(":")) {
            // Metadata
            if (td.Value.startsWith("author:")) {
                // Author data
                td.Element.innerText = td.Value.split(":")[1]
                if (td.IsFileTag) {
                    authorTagE.appendChild(td.Element)
                    authorDiv.hidden = false
                    return
                }
                if(!showAuthors) {
                    return
                }
            } else if (td.Value.startsWith("collection:")) {
                // Collection data
                td.Element.innerText = td.Value.split(":")[1]
                if (td.IsFileTag) {
                    loadCollection(td.Value.split(":")[1])
                    collectionTagE.appendChild(td.Element)
                    colDiv.hidden = false
                }
                if(!showCollections) {
                    return
                }
            } else if(td.Value.startsWith("colindex:")) {
                const colIndexA = document.getElementById("col_index") as HTMLAnchorElement
                colIndexA.classList.add("tag")
                colIndexA.classList.add("metadata")
                colIndexA.text = `${td.Value.split(":")[1]}`
                return
            } else {
                // Other metadata
                console.log(`Unknown tag namespace: ${td.Value}`)
                return
            }
        }
        td.Element.classList.remove("add_tag", "rem_tag")
        if (td.IsFileTag) {
            td.Element.onclick = () => {
                if (td.Element.classList.contains("rem_tag")) {
                    td.Element.classList.remove("rem_tag")
                    //td.Element.classList.add("tag")
                } else {
                    //td.Element.classList.remove("tag")
                    td.Element.classList.add("rem_tag")
                }
            }
            fileTagE.appendChild(td.Element)
        } else {
            td.Element.onclick = () => {
                if (td.Element.classList.contains("add_tag")) {
                    td.Element.classList.remove("add_tag")
                    //td.Element.classList.add("tag")
                } else {
                    //td.Element.classList.remove("tag")
                    td.Element.classList.add("add_tag")
                }
            }
            allTagE.appendChild(td.Element)
        }
    });
}

/**
 * Get the file from the query param & return it
 * @deprecated Dude come on, this is gross.
 * @returns File, or on rejection a error string
 */
async function getFile(): Promise<MMFile> {
    return new Promise(async (resolve, reject) => {
        const urlParams = new URLSearchParams(window.location.search)
        const id = urlParams.get("id")
        if (id == null) {
            alert("Expected actual 'id' query param, got nothing.")
            reject("missing 'id' query")
            return
        }
        const file_id = Number(urlParams.get('id'))
        if (Number.isNaN(file_id)) {
            alert("Expected 'id' parameter to be a number")
            reject("invalid 'id' query")
            return
        }
        if (file_id <= 0) {
            alert("Expected positive 'id' parameter.")
            reject("invalid 'id' query")
            return
        }
        let file: MMFile
        try {
            file = await apiGetFile(file_id, false)
            resolve(file)
            return
        } catch (e) {
            console.error(`Failed to get file: ${e}`)
            alert(`File not found`)
            reject("failed to get file")
            return
        }
    })
}

/**
 * Creates a div for the file & loads its content
 * @param file File to load the content of
 * @param addHiddenTitle Should there be a hidden title (class 'file-name') added in the div
 * @param expandedImages Should the image be expanded by default
 * @returns A DIV with the file, this file data may not be loaded yet.
 */
function createFileElement(file: MMFile, addHiddenTitle = false, expandedImages = false) {
    // Create the div
    const ourDiv = document.createElement("div")
    ourDiv.classList.add("file-div")
    // Add the title if desired.
    if (addHiddenTitle) {
        const name = document.createElement("a")
        name.href = `file?id=${file.getId()}`
        name.classList.add("file-name")
        name.hidden = true
        name.textContent = file.getPath()
        ourDiv.appendChild(name)
    }
    // Create a source
    let source = document.createElement("source") as HTMLSourceElement
    source.id = "media_src"
    source.src = apiGetContentUri(file.getId(), false)
    // Get our expected content type
    apiGetFileContentType(file.getId()).then((v) => {
        if (v.startsWith("image")) {
            // Create a image 
            let image = document.createElement("img") as HTMLImageElement
            // This should be classes.
            if (expandedImages) {
                image.id = "expanded-media"
            } else {
                image.id = "media"
                // On click expand it
                image.onclick = () => {
                    if (image.id == "media") {
                        image.id = "expanded-media"
                    } else {
                        image.id = "media"
                    }
                }
            }
            // Set source & add to div
            image.src = source.src
            ourDiv.appendChild(image)
        } else if (v.startsWith("video")) {
            // Create video tag
            let video = document.createElement("video") as HTMLVideoElement
            video.id = "media"
            video.autoplay = false
            // We want this incase we got short videos.
            video.loop = true
            video.controls = true
            // Preload the metadata stuff
            video.preload = "metadata"
            video.appendChild(source)
            ourDiv.appendChild(video)
        } else {
            // TODO: Some sorta fallback
            alert(`Failed to get content-type: ${v}`)
            console.error("Failed to get content type", v)
            throw `Got unusable content type ${v}`
        }
    }).catch((e) => {
        alert(`Failed to get content-type: ${e}`)
        console.error("Failed to get content type", e)
        throw "Failed to get content type"
    })
    return ourDiv
}

/**
 * Add a file to the mediaContainer
 * @param file File to add
 */
function setupFileSource(file: MMFile) {
    const container = document.getElementById("mediaContainer") as HTMLDivElement
    container.appendChild(createFileElement(file))
}

/**
 * Update a file
 * @param file 
 * @todo Document this better
 * @returns 
 */
async function updateFile(file: MMFile) {
    // Modify this file, not the original.
    let mFile = file.copy()
    // Update tags & the file
    tagList.forEach(t => {
        if (t.Element.classList.contains("add_tag")) {
            // Add it the file
            t.IsFileTag = true
            try {
                mFile.addTag(t.Value)
            } catch (e) {
                // Probably some bullshit
                console.log(`addTag failed: ${e}`)
            }
        } else if (t.Element.classList.contains("rem_tag")) {
            // Remove it
            t.IsFileTag = false
            mFile.removeTag(t.Value)
        }
    });
    // Stars
    let stars = document.getElementById("stars") as HTMLSelectElement
    let set_to = 0
    switch (stars.value) {
        case "5":
            set_to = 5
            break
        case "4":
            set_to = 4
            break
        case "3":
            set_to = 3
            break
        case "2":
            set_to = 2
            break
        case "1":
            set_to = 1
            break
        case "0":
            set_to = 0
            break
        default:
            alert(`Attempted to set stars to ${stars.value}, a invalid value`)
            return
    }
    mFile.setStars(set_to)
    // Re render the tags
    renderTags()
    // Send update & update the fil
    await apiUpdateFile(mFile, file)
}

/**
 * Setup the file info on page.
 * @param file File to set
 * @param render Should the media be rendered
 * @returns 
 */
async function setupFile(file: MMFile, render = true) {
    // Setup file data
    const lastView = document.getElementById("last_view") as HTMLAnchorElement
    const size = document.getElementById("size") as HTMLAnchorElement
    const path = document.getElementById("path") as HTMLHeadElement
    const container = document.getElementById("mediaContainer") as HTMLDivElement
    path.textContent = file.getPath()
    // Setup tags
    await setupTags(file)
    renderTags()
    lastView.text = `Last Viewed: ${file.getLastViewed()}`
    size.text = `Size: ${file.getSize()} bytes`
    // Setup actual file display
    if (render) {
        await setupFileSource(file)
    }
    // Handle interactions
    // Set the 'stars' value
    switch (file.getStars()) {
        case 0:
            (document.getElementById("stars_0") as HTMLOptionElement).selected = true
            break
        case 1:
            (document.getElementById("stars_1") as HTMLOptionElement).selected = true
            break
        case 2:
            (document.getElementById("stars_2") as HTMLOptionElement).selected = true
            break
        case 3:
            (document.getElementById("stars_3") as HTMLOptionElement).selected = true
            break
        case 4:
            (document.getElementById("stars_4") as HTMLOptionElement).selected = true
            break
        case 5:
            (document.getElementById("stars_5") as HTMLOptionElement).selected = true
            break
        default:
            alert(`Did stars change? this file has ${file.getStars()}`)
            return
    }
    // Mark file as viewed
    const viewedButton = document.getElementById("viewed") as HTMLButtonElement
    viewedButton.onclick = () => {
        // Maybe need some sort API for this.
        apiUpdateFileViewed(file.getId())
    }
    // Setup submit button
    const submitStars = document.getElementById("stars_submit") as HTMLButtonElement
    submitStars.onclick = () => {
        updateFile(file)
    }
    // Setup new tag button
    const submitNewTag = document.getElementById("new_tag_sub") as HTMLButtonElement
    submitNewTag.onclick = () => {
        const tag = (document.getElementById("add_tag") as HTMLInputElement).value
        if (tag == "") {
            alert("Cant add empty tag")
            return
        }
        let copy = file.copy()
        copy.addTag(tag)
        apiUpdateFile(copy, file).then(() => {
            const fileTagE = document.getElementById("tags") as HTMLSpanElement
            let element = document.createElement("a")
            element.classList.add("tag")
            element.text = tag
            fileTagE.appendChild(element)
        }).catch((e) => {
            console.error("Failed to update file", e)
            alert(`Failed to update file: ${e.Data}`)
        })
    }
}

/**
 * Initial setup function
 * @returns 
 */
async function setup() {
    let button = document.getElementById("random_button") as HTMLButtonElement
        button.onclick = () => {
            apiGetRandomFile().then(file => {
                console.log(file)
                window.location.href = `/file?id=${file.getId()}`
            })
        }
    (document.getElementById("show_authors") as HTMLInputElement).onclick = () => {
        renderTags()
    }
    (document.getElementById("show_collections") as HTMLInputElement).onclick = () => {
        renderTags()
    }
    // Figure out if were viewing a file or collection (I know, putting it under /file is dumb, but idc)
    const urlParams = new URLSearchParams(window.location.search)
    // Figure out of it a collection, id or neither
    const id = urlParams.get("id")
    const col = urlParams.get("collection")
    if (id == null && col == null) {
        alert("Expected a 'id' or 'collection' query parameter")
        return
    }
    if (col != null) {
        await loadCollection(col, true)
        return
    }
    let file = await getFile()
    fileId = file.getId()
    setupFile(file)
}

window.onload = setup