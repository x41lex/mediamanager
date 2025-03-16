var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { apiGetRandomFile, apiSearch, bytesToHumanReadableSize, getCookie, setCookie } from "./api.js";
// TODO: Set cookies based on what we've recently look at & our recent settings os we can actually make this work
class FileManager {
    // TODO: Popup window
    // TODO: Search by tag
    // TODO: Search by name
    // TODO: Select page size (50 to 200)
    // TODO: Update file content
    // TODO: Next/Prev file buttons
    constructor() {
        this.modalMode = true;
        this.file_count = 50;
        this.index = 0;
        this.tag_whitelist = [];
        this.tag_blacklist = [];
        this.query = "";
        this.sortMethod = "stars";
        this.sortReverse = true;
        this.openInNewTab = true;
        this.modal = document.getElementById("filePopup");
        this.iframe = document.getElementById("modalDisplay");
        this.table = document.getElementById("files");
        if (this.table == null) {
            throw "table not found";
        }
        this.files = [];
        this.index = 0;
    }
    addMmFile(index) {
        const file = this.files[index];
        let tr = document.createElement("tr");
        let name = document.createElement("td");
        name.innerText = `${file.getId()}`;
        // TODO: Some javascript to pop this file up
        let path = document.createElement("td");
        let pathA = document.createElement("a");
        // TODO: Remove update=false
        pathA.innerText = file.getPath();
        pathA.href = `/file?id=${file.getId()}`;
        pathA.onclick = () => {
            // TODO: Set a cookie for the next & prev files so we can do something on the file page.
            this.openFile(`/file?id=${file.getId()}`);
            return false;
        };
        pathA.classList.add("path");
        if (this.openInNewTab) {
            pathA.target = "_blank";
        }
        path.appendChild(pathA);
        // TODO: Format better? (Probably just create a whole new display system in the future)
        let tags = document.createElement("td");
        // We display ever tag
        let fileTags = file.getTags();
        fileTags.forEach(tag => {
            // TODO: Prevent the same thing from being added twice.
            let t = document.createElement("a");
            // Just do this 
            t.classList.add("tag");
            let sp = tag.split(":");
            if (sp.length > 1) {
                if (sp[0] == "author") {
                    t.classList.add("author");
                }
                else if (sp[0] == "collection") {
                    t.classList.add("collection");
                }
                else if (sp[0] == "colindex") {
                    // Don't add it.
                    return;
                }
                else {
                    t.classList.add('metadata');
                }
                t.text = sp[1];
            }
            else {
                t.text = tag;
            }
            t.onclick = () => {
                let input = document.getElementById("wl_tags");
                if (input.value.length > 0) {
                    input.value += ",";
                }
                input.value += `${tag}`;
                this.setWhitelistTags(input.value.split(","));
                this.refresh();
            };
            let comma = document.createElement("a");
            comma.innerText = " ";
            tags.appendChild(t);
            tags.appendChild(comma);
        });
        let stars = document.createElement("td");
        let starsNum = file.getStars();
        stars.innerText = `${starsNum}`;
        let last_viewed = document.createElement("td");
        let realLastViewed = file.getLastViewed();
        last_viewed.innerText = realLastViewed == "0001-01-01T00:00:00Z" ? "never" : realLastViewed;
        let size = document.createElement("td");
        size.innerText = `${bytesToHumanReadableSize(file.getSize())}`;
        // Add everything
        tr.appendChild(name);
        tr.appendChild(path);
        tr.appendChild(tags);
        tr.appendChild(stars);
        tr.appendChild(last_viewed);
        tr.appendChild(size);
        // Add to table
        this.table.appendChild(tr);
    }
    doApiRequest() {
        return __awaiter(this, void 0, void 0, function* () {
            const query = {
                Index: this.index,
                Count: this.file_count,
                TagWhitelist: this.tag_whitelist.length == 0 ? undefined : this.tag_whitelist,
                TagBlacklist: this.tag_blacklist.length == 0 ? undefined : this.tag_blacklist,
                Path: this.query == "" ? undefined : this.query,
                Sort: this.sortMethod,
                SortReverse: this.sortReverse,
            };
            setCookie("search", JSON.stringify(query), {
                Secure: true
            });
            this.files = yield apiSearch(query);
            const idx = this.files.map((v) => {
                return v.getId();
            });
            setCookie("file_idx", JSON.stringify(idx));
        });
    }
    refresh() {
        return __awaiter(this, void 0, void 0, function* () {
            this.index = 0;
            // Get the files
            // page_num*this.file_count, this.file_count
            yield this.doApiRequest();
            // Clear table
            this.table.innerHTML = "";
            for (let i = 0; i != this.files.length; i++) {
                this.addMmFile(i);
            }
            this.index = this.file_count;
        });
    }
    requestPage() {
        return __awaiter(this, void 0, void 0, function* () {
            // Get the files
            // page_num*this.file_count, this.file_count
            yield this.doApiRequest();
            // Clear table
            this.table.innerHTML = "";
            for (let i = 0; i != this.files.length; i++) {
                this.addMmFile(i);
            }
            this.index += this.file_count;
        });
    }
    nextPage() {
        return __awaiter(this, void 0, void 0, function* () {
            console.log(`this.index: ${this.index} this.file_count: ${this.file_count}, this.files.length: ${this.files.length}`);
            if (this.file_count > this.files.length) {
                return;
            }
            yield this.requestPage();
        });
    }
    prevPage() {
        return __awaiter(this, void 0, void 0, function* () {
            console.log(`this.index: ${this.index} new ${this.index - (this.file_count * 2)} this.file_count ${this.file_count}`);
            if (this.index < (this.file_count * 2)) {
                return;
            }
            this.index -= this.file_count * 2;
            yield this.requestPage();
        });
    }
    setWhitelistTags(v) {
        let tags = [];
        v.forEach(e => {
            e = e.toLowerCase().trim();
            if (e.length == 0) {
                return;
            }
            tags.push(e);
        });
        console.log(`Set tags to ${tags} (${tags.length} elements)`);
        this.tag_whitelist = tags;
    }
    setBlacklistTags(v) {
        let tags = [];
        v.forEach(e => {
            e = e.toLowerCase().trim();
            if (e.length == 0) {
                return;
            }
            tags.push(e);
        });
        console.log(`Set tags to ${tags} (${tags.length} elements)`);
        this.tag_blacklist = tags;
    }
    setQuery(v) {
        this.query = v;
    }
    setSortMethod(method = "none", reverse = false) {
        this.sortMethod = method;
        this.sortReverse = reverse;
    }
    setCount(count) {
        if (typeof count == "string") {
            count = Number(count);
        }
        if (0 >= count) {
            alert(`Count must be greater then 0, was ${count}`);
            return;
        }
        if (200 < count) {
            alert(`Count must be less then 200, was ${count}`);
            return;
        }
        this.file_count = count;
    }
    openFile(url) {
        console.log(`Open: ${url}`);
        if (this.modalMode) {
            this.modal.style.display = "flex";
            this.iframe.src = url;
        }
        else {
            window.location.href = url;
        }
    }
    setSearchQuery(qr) {
        if (qr.Path) {
            this.setQuery(qr.Path);
        }
        if (qr.TagWhitelist) {
            this.setWhitelistTags(qr.TagWhitelist);
        }
        if (qr.TagBlacklist) {
            this.setBlacklistTags(qr.TagBlacklist);
        }
        if (qr.Index) {
            this.index = qr.Index;
        }
        if (qr.Count) {
            this.setCount(qr.Count);
        }
        if (qr.Sort) {
            this.setSortMethod(qr.Sort);
        }
        if (qr.SortReverse) {
            this.sortReverse = qr.SortReverse;
        }
    }
}
window.onload = () => {
    var fh = new FileManager();
    // Setup
    let nextbutton = document.getElementById("next_page");
    let backbutton = document.getElementById("prev_page");
    let wltaginput = document.getElementById("wl_tags");
    let bltaginput = document.getElementById("bl_tags");
    let submitbutton = document.getElementById("submit");
    let queryinput = document.getElementById("query");
    let sortMethod = document.getElementById("method");
    let sortReverse = document.getElementById("reverse_sort");
    let openInNewPage = document.getElementById("open_in_new");
    let count = document.getElementById("count");
    //let updatecheck = document.getElementById("update_date") as HTMLInputElement
    count.onchange = () => {
        fh.setCount(count.value);
        fh.refresh();
    };
    fh.setCount(count.value);
    nextbutton.onclick = () => {
        fh.nextPage();
    };
    backbutton.onclick = () => {
        fh.prevPage();
    };
    openInNewPage.onclick = () => {
        fh.openInNewTab = !fh.openInNewTab;
        const paths = document.getElementsByClassName("path");
        for (const i in paths) {
            if (fh.openInNewTab) {
                // Open in new tab
                paths[i].target = "_blank";
            }
            else {
                // Open in this tab
                paths[i].target = "";
            }
        }
    };
    const search = getCookie("search");
    if (search) {
        const opts = JSON.parse(search);
        fh.setSearchQuery(opts);
        if (opts.Path) {
            queryinput.value = opts.Path;
        }
        if (opts.Sort) {
            sortMethod.value = opts.Sort;
        }
        if (opts.TagWhitelist) {
            wltaginput.value = opts.TagWhitelist.join(",");
        }
        if (opts.TagBlacklist) {
            bltaginput.value = opts.TagBlacklist.join(",");
        }
        if (opts.Count) {
            count.value = `${opts.Count}`;
        }
        if (opts.SortReverse) {
            sortReverse.checked = opts.SortReverse;
        }
    }
    submitbutton.onclick = () => {
        if (sortMethod.value != "none" && sortMethod.value != "size" && sortMethod.value != "stars" && sortMethod.value != "date" && sortMethod.value != "id" && sortMethod.value != "random") {
            alert(`Invalid sortMethod: ${sortMethod}`);
            return;
        }
        console.log("submit");
        fh.setWhitelistTags(wltaginput.value.split(","));
        fh.setBlacklistTags(bltaginput.value.split(","));
        fh.setQuery(queryinput.value);
        fh.setSortMethod(sortMethod.value, sortReverse.checked);
        fh.setCount(count.value);
        fh.openInNewTab = openInNewPage.checked;
        fh.refresh();
    };
    let button = document.getElementById("random_button");
    button.onclick = () => {
        apiGetRandomFile().then(file => {
            console.log(file);
            window.location.href = `/file?id=${file.getId()}`;
        });
    };
    let modalButton = document.getElementById("pupup_mode");
    fh.modalMode = modalButton.checked;
    modalButton.onclick = () => {
        fh.modalMode = modalButton.checked;
    };
    const modalSpan = document.getElementById("modalClose");
    const modal = document.getElementById("filePopup");
    const iframe = document.getElementById("modalDisplay");
    modalSpan.onclick = (ev) => {
        modal.style.display = "none";
        iframe.src = "";
    };
    fh.requestPage();
};
window.onclick = (ev) => {
    const modal = document.getElementById("filePopup");
    const iframe = document.getElementById("modalDisplay");
    if (ev.target == modal) {
        modal.style.display = "none";
        iframe.src = "";
    }
};
