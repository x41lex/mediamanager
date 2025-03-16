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
 * *Really* shitty code, super messy cause I was experimenting (Also javascript & typescript let you get away with some nasty shit, idk its a language that makes you want to work like shit.)
 *
 * @todo Error handling popups.
 */
import { apiGetFile, apiGetContentUri, apiGetFileContentType, apiUpdateFile, apiGetTags, apiUpdateFileViewed, apiGetCollection, apiGetRandomFile, getCookie } from "./api.js";
/**
 * Id of the current file, or -1 if we're using collection.
 * @todo Replace with file, or just remove.
 */
var fileId = -1;
/**
 * A list of all tags
 */
var tagList = [];
/**
 * Current collection this file is part of (If it is, otherwise its empty)
 */
var collection = [];
/**
 * Index in the collection this file is
 */
var colIndex = 0;
/**
 * Loads a collection by name
 * @param name Collection name (Without the collection: part)
 * @param showFull Should the collection be showed now
 * @param author Replace using 'collection:' with using 'author:'
 * @todo Remove globals, add whatever error handling were going to use
 */
function loadCollection(name, showFull = false, author = false) {
    return __awaiter(this, void 0, void 0, function* () {
        if (collection.length > 0) {
            // If the collection isn't empty its already been loaded
            console.error("Attempted to reload loaded collection");
            return;
        }
        // Search the collection.s
        if (author) {
            try {
                collection = yield apiGetCollection(name, "author");
            }
            catch (e) {
                alert(`Failed to get collection: ${e}`);
                return;
            }
        }
        else {
            try {
                collection = yield apiGetCollection(name);
            }
            catch (e) {
                alert(`Failed to get collection: ${e}`);
                return;
            }
        }
        // If we're showing the collection just do that now
        if (showFull) {
            yield showFullCollection();
            return;
        }
        const colIndexA = document.getElementById("col_index");
        // Find the file in the collection
        for (let i = 0; i < collection.length; i++) {
            if (collection[i].getId() == fileId) {
                colIndex = i;
                colIndexA.text = `${i}`;
                break;
            }
        }
        // Not found?
        if (colIndex == -1) {
            alert(`File doesn't exist in collection?`);
            collection = [];
            return;
        }
        // Set the button actions.
        const colPrev = document.getElementById("col_back");
        const colNext = document.getElementById("col_next");
        const showAll = document.getElementById("col_show_all");
        colPrev.onclick = () => {
            if (colIndex > 0) {
                colIndex -= 1;
                // Change our file
                changeFile(collection[colIndex]);
            }
        };
        colNext.onclick = () => {
            if (colIndex < collection.length) {
                colIndex += 1;
                // Change our file
                changeFile(collection[colIndex]);
            }
        };
        showAll.onclick = showFullCollection;
        return;
    });
}
/**
 * Show the full collection in the 'collection' global
 * @returns
 */
function showFullCollection() {
    return __awaiter(this, void 0, void 0, function* () {
        // No collection = bad
        if (collection.length == 0) {
            alert("No collection loaded");
            return;
        }
        // SHow the collection controls
        document.getElementById("col_controls").hidden = false;
        // Setup the show names buttons
        document.getElementById("col_show_names").onchange = (s) => {
            const state = s.target.checked;
            const elements = document.getElementsByClassName("file-name");
            for (let index = 0; index < elements.length; index++) {
                const e = elements[index];
                e.hidden = !state;
            }
        };
        // Get collection name
        let cName = "";
        for (const tag of collection[0].getTags()) {
            if (tag.startsWith("collection:")) {
                cName = tag.split(":")[1];
                break;
            }
        }
        // No name = bad.
        if (cName == "") {
            alert("Failed to get collection name");
            return;
        }
        // Set it
        const path = document.getElementById("path");
        path.innerText = `Collection: ${cName}`;
        // Hide controls
        const controls = document.getElementById("controls");
        controls.hidden = true;
        // Remove current media
        const container = document.getElementById("mediaContainer");
        container.innerHTML = "";
        // Change URL
        window.history.pushState("", "");
        window.history.replaceState("", "", `/file?collection=${cName}`);
        // Add each one, making sure they are in order.
        collection.forEach(element => {
            let d = createFileElement(element, true, true);
            container.appendChild(d);
        });
    });
}
/**
 * Change the current displayed file
 * @param file New file to display
 */
function changeFile(file) {
    fileId = file.getId();
    // Add a new thing
    setupFileSource(file);
    // Remove the old one
    const container = document.getElementById("mediaContainer");
    // Remove old nodes
    while (container.childNodes.length != 1) {
        container.removeChild(container.childNodes[0]);
    }
    setupFile(file, false);
    // Change URL
    window.history.pushState("", "");
    window.history.replaceState("", "", `/file?id=${fileId}`);
}
/**
 * Setup the tags of this file
 * @param file File to set the tags up for
 */
function setupTags(file) {
    return __awaiter(this, void 0, void 0, function* () {
        const tags = yield apiGetTags();
        const fileTags = file.getTags();
        tagList = [];
        tags.forEach(t => {
            // Create element for it
            let element = document.createElement("a");
            element.classList.add("tag");
            let sp = t.split(":");
            if (sp.length > 1) {
                if (sp[0] == "author") {
                    element.classList.add("author");
                }
                else if (sp[0] == "collection") {
                    element.classList.add("collection");
                }
                else {
                    element.classList.add('metadata');
                }
                element.text = sp[1];
            }
            else {
                element.text = t;
            }
            tagList.push({
                Element: element,
                IsFileTag: fileTags.includes(t),
                IsSelected: false,
                Value: t
            });
        });
    });
}
/**
 * Re-render the tags on the file, call on any updates
 */
function renderTags() {
    return __awaiter(this, void 0, void 0, function* () {
        const fileTagE = document.getElementById("tags");
        const allTagE = document.getElementById("all_tags");
        const authorTagE = document.getElementById("author_tags");
        const authorDiv = document.getElementById("author_div");
        const collectionTagE = document.getElementById("col_tags");
        const colDiv = document.getElementById("collection_div");
        const showAuthors = document.getElementById("show_authors").checked;
        const showCollections = document.getElementById("show_collections").checked;
        // Clear tag
        fileTagE.innerHTML = "";
        allTagE.innerHTML = "";
        authorTagE.innerHTML = "";
        collectionTagE.innerHTML = "";
        tagList.forEach(td => {
            if (td.Value.includes(":")) {
                // Metadata
                if (td.Value.startsWith("author:")) {
                    // Author data
                    td.Element.innerText = td.Value.split(":")[1];
                    if (td.IsFileTag) {
                        authorTagE.appendChild(td.Element);
                        authorDiv.hidden = false;
                        return;
                    }
                    if (!showAuthors) {
                        return;
                    }
                }
                else if (td.Value.startsWith("collection:")) {
                    // Collection data
                    td.Element.innerText = td.Value.split(":")[1];
                    if (td.IsFileTag) {
                        loadCollection(td.Value.split(":")[1]);
                        collectionTagE.appendChild(td.Element);
                        colDiv.hidden = false;
                    }
                    if (!showCollections) {
                        return;
                    }
                }
                else if (td.Value.startsWith("colindex:")) {
                    const colIndexA = document.getElementById("col_index");
                    colIndexA.classList.add("tag");
                    colIndexA.classList.add("metadata");
                    colIndexA.text = `${td.Value.split(":")[1]}`;
                    return;
                }
                else {
                    // Other metadata
                    console.log(`Unknown tag namespace: ${td.Value}`);
                    return;
                }
            }
            td.Element.classList.remove("add_tag", "rem_tag");
            if (td.IsFileTag) {
                td.Element.onclick = () => {
                    if (td.Element.classList.contains("rem_tag")) {
                        td.Element.classList.remove("rem_tag");
                        //td.Element.classList.add("tag")
                    }
                    else {
                        //td.Element.classList.remove("tag")
                        td.Element.classList.add("rem_tag");
                    }
                };
                fileTagE.appendChild(td.Element);
            }
            else {
                td.Element.onclick = () => {
                    if (td.Element.classList.contains("add_tag")) {
                        td.Element.classList.remove("add_tag");
                        //td.Element.classList.add("tag")
                    }
                    else {
                        //td.Element.classList.remove("tag")
                        td.Element.classList.add("add_tag");
                    }
                };
                allTagE.appendChild(td.Element);
            }
        });
    });
}
/**
 * Get the file from the query param & return it
 * @deprecated Dude come on, this is gross.
 * @returns File, or on rejection a error string
 */
function getFile() {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => __awaiter(this, void 0, void 0, function* () {
            const urlParams = new URLSearchParams(window.location.search);
            const id = urlParams.get("id");
            if (id == null) {
                alert("Expected actual 'id' query param, got nothing.");
                reject("missing 'id' query");
                return;
            }
            const file_id = Number(urlParams.get('id'));
            if (Number.isNaN(file_id)) {
                alert("Expected 'id' parameter to be a number");
                reject("invalid 'id' query");
                return;
            }
            if (file_id <= 0) {
                alert("Expected positive 'id' parameter.");
                reject("invalid 'id' query");
                return;
            }
            let file;
            try {
                file = yield apiGetFile(file_id, false);
                resolve(file);
                return;
            }
            catch (e) {
                console.error(`Failed to get file: ${e}`);
                alert(`File not found`);
                reject("failed to get file");
                return;
            }
        }));
    });
}
/**
 * Creates a div for the file & loads its content
 * @param file File to load the content of
 * @param addHiddenTitle Should there be a hidden title (class 'file-name') added in the div
 * @param expandedImages Should the image be expanded by default
 * @returns A DIV with the file, this file data may not be loaded yet.
 */
function createFileElement(file, addHiddenTitle = false, expandedImages = false) {
    // Create the div
    const ourDiv = document.createElement("div");
    ourDiv.classList.add("file-div");
    // Add the title if desired.
    if (addHiddenTitle) {
        const name = document.createElement("a");
        name.href = `file?id=${file.getId()}`;
        name.classList.add("file-name");
        name.hidden = true;
        name.textContent = file.getPath();
        ourDiv.appendChild(name);
    }
    // Get our expected content type
    apiGetFileContentType(file.getId()).then((v) => {
        if (v.startsWith("application/octet-stream") || v.startsWith("text/plain")) {
            let p = document.createElement("p");
            p.id = "text";
            file.getContent().then((v) => {
                p.textContent = v;
            });
            ourDiv.appendChild(p);
            return;
        }
        // Create a source
        let source = document.createElement("source");
        source.id = "media_src";
        source.src = apiGetContentUri(file.getId(), false);
        if (v.startsWith("image")) {
            // Create a image 
            let image = document.createElement("img");
            // This should be classes.
            if (expandedImages) {
                image.id = "expanded-media";
            }
            else {
                image.id = "media";
                // On click expand it
                image.onclick = () => {
                    if (image.id == "media") {
                        image.id = "expanded-media";
                    }
                    else {
                        image.id = "media";
                    }
                };
            }
            // Set source & add to div
            image.src = source.src;
            ourDiv.appendChild(image);
        }
        else if (v.startsWith("video")) {
            // Create video tag
            let video = document.createElement("video");
            video.id = "media";
            video.autoplay = false;
            // We want this incase we got short videos.
            video.loop = true;
            video.controls = true;
            // Preload the metadata stuff
            video.preload = "metadata";
            video.appendChild(source);
            ourDiv.appendChild(video);
        }
        else {
            // TODO: Some sorta fallback
            alert(`Failed to get content-type: ${v}`);
            console.error("Failed to get content type", v);
            throw `Got unusable content type ${v}`;
        }
    }).catch((e) => {
        alert(`Failed to get content-type: ${e}`);
        console.error("Failed to get content type", e);
        throw "Failed to get content type";
    });
    return ourDiv;
}
/**
 * Add a file to the mediaContainer
 * @param file File to add
 */
function setupFileSource(file) {
    const container = document.getElementById("mediaContainer");
    container.appendChild(createFileElement(file));
}
/**
 * Update a file
 * @param file
 * @todo Document this better
 * @returns
 */
function updateFile(file) {
    return __awaiter(this, void 0, void 0, function* () {
        // Modify this file, not the original.
        let mFile = file.copy();
        // Update tags & the file
        tagList.forEach(t => {
            if (t.Element.classList.contains("add_tag")) {
                // Add it the file
                t.IsFileTag = true;
                try {
                    mFile.addTag(t.Value);
                }
                catch (e) {
                    // Probably some bullshit
                    console.log(`addTag failed: ${e}`);
                }
            }
            else if (t.Element.classList.contains("rem_tag")) {
                // Remove it
                t.IsFileTag = false;
                mFile.removeTag(t.Value);
            }
        });
        // Stars
        let stars = document.getElementById("stars");
        let set_to = 0;
        switch (stars.value) {
            case "5":
                set_to = 5;
                break;
            case "4":
                set_to = 4;
                break;
            case "3":
                set_to = 3;
                break;
            case "2":
                set_to = 2;
                break;
            case "1":
                set_to = 1;
                break;
            case "0":
                set_to = 0;
                break;
            default:
                alert(`Attempted to set stars to ${stars.value}, a invalid value`);
                return;
        }
        mFile.setStars(set_to);
        // Re render the tags
        renderTags();
        console.log(`Updating file`, mFile, file);
        // Send update & update the fil
        yield apiUpdateFile(mFile, file);
    });
}
/**
 * Setup the file info on page.
 * @param file File to set
 * @param render Should the media be rendered
 * @returns
 */
function setupFile(file, render = true) {
    return __awaiter(this, void 0, void 0, function* () {
        // Setup file data
        const lastView = document.getElementById("last_view");
        const size = document.getElementById("size");
        const path = document.getElementById("path");
        const container = document.getElementById("mediaContainer");
        path.textContent = file.getPath();
        // Setup tags
        yield setupTags(file);
        renderTags();
        lastView.text = `Last Viewed: ${file.getLastViewed()}`;
        size.text = `Size: ${file.getSize()} bytes`;
        // Setup actual file display
        if (render) {
            yield setupFileSource(file);
        }
        // Handle interactions
        // Set the 'stars' value
        switch (file.getStars()) {
            case 0:
                document.getElementById("stars_0").selected = true;
                break;
            case 1:
                document.getElementById("stars_1").selected = true;
                break;
            case 2:
                document.getElementById("stars_2").selected = true;
                break;
            case 3:
                document.getElementById("stars_3").selected = true;
                break;
            case 4:
                document.getElementById("stars_4").selected = true;
                break;
            case 5:
                document.getElementById("stars_5").selected = true;
                break;
            default:
                alert(`Did stars change? this file has ${file.getStars()}`);
                return;
        }
        // Mark file as viewed
        const viewedButton = document.getElementById("viewed");
        viewedButton.onclick = () => {
            // Maybe need some sort API for this.
            apiUpdateFileViewed(file.getId());
        };
        // Setup submit button
        const submitStars = document.getElementById("stars_submit");
        submitStars.onclick = () => {
            updateFile(file);
        };
        // Setup new tag button
        const submitNewTag = document.getElementById("new_tag_sub");
        submitNewTag.onclick = () => {
            const tag = document.getElementById("add_tag").value;
            if (tag == "") {
                alert("Cant add empty tag");
                return;
            }
            let copy = file.copy();
            copy.addTag(tag);
            apiUpdateFile(copy, file).then(() => {
                const fileTagE = document.getElementById("tags");
                let element = document.createElement("a");
                element.classList.add("tag");
                element.text = tag;
                fileTagE.appendChild(element);
            }).catch((e) => {
                console.error("Failed to update file", e);
                alert(`Failed to update file: ${e.Data}`);
            });
        };
    });
}
function setNextAndPrev(id) {
    const indexs = getCookie("file_idx");
    if (!indexs) {
        return;
    }
    const jIdx = JSON.parse(indexs);
    const idx = jIdx.indexOf(Number(id));
    if (idx == -1) {
        console.error(`file_idx cookie failed to find file id`);
        return;
    }
    if (idx > 0) {
        const backButton = document.getElementById("go_back");
        backButton.disabled = false;
        backButton.onclick = () => {
            window.location.href = `/file?id=${jIdx[idx - 1]}`;
        };
        // TODO: Check if the search query provided in cookie "search" .Index is not 0, if its not zero we can do the reverse DB query and put it in front of this array
        // (Maybe remove all but the first one from old array?)
    }
    if ((idx + 1) < jIdx.length) {
        const nextButton = document.getElementById("go_next");
        nextButton.disabled = false;
        nextButton.onclick = () => {
            window.location.href = `/file?id=${jIdx[idx + 1]}`;
        };
        // TODO: Make the next DB query based on "search" if we're out of data we stop, otherwise append it (Maybe remove all but the last one from the old array)
    }
}
/**
 * Initial setup function
 * @returns
 */
function setup() {
    return __awaiter(this, void 0, void 0, function* () {
        let button = document.getElementById("random_button");
        button.onclick = () => {
            apiGetRandomFile().then(file => {
                window.location.href = `/file?id=${file.getId()}`;
            });
        };
        document.getElementById("show_authors").onclick = () => {
            renderTags();
        };
        document.getElementById("show_collections").onclick = () => {
            renderTags();
        };
        // Figure out if were viewing a file or collection (I know, putting it under /file is dumb, but idc)
        const urlParams = new URLSearchParams(window.location.search);
        // Figure out of it a collection, id or neither
        const id = urlParams.get("id");
        setNextAndPrev(Number(id));
        const col = urlParams.get("collection");
        if (id == null && col == null) {
            alert("Expected a 'id' or 'collection' query parameter");
            return;
        }
        if (col != null) {
            yield loadCollection(col, true);
            return;
        }
        let file = yield getFile();
        fileId = file.getId();
        setupFile(file);
        document.getElementById("pop_out").onclick = () => {
            var _a;
            (_a = window.open(file.getContentUri() + "&update=false", "_blank")) === null || _a === void 0 ? void 0 : _a.focus();
        };
        document.getElementById("path").onclick = () => {
            var _a;
            (_a = window.open(file.getContentUri() + "&update=false", "_blank")) === null || _a === void 0 ? void 0 : _a.focus();
        };
    });
}
window.onload = setup;
