import { apiGetCollection, apiGetTags } from "./api.js"

window.onload = async () => {
    const table = document.getElementById("table_body") as HTMLTableElement
    // So we get all tags
    const tags = await apiGetTags()
    tags.forEach(async t => {
        if(t.startsWith("collection:")) {
            const nameTd = document.createElement("td")
            const nameElement = document.createElement("a")
            nameElement.text = t.split(":")[1]
            nameElement.href = `/file?collection=${nameElement.text}`
            nameTd.appendChild(nameElement)
            const fileCountTd = document.createElement("td")
            const fileCount = document.createElement("a")
            // Get collection info
            apiGetCollection(nameElement.text).then(e => {
                fileCount.text = `${e.length}`
            })
            fileCount.text = `(not set)`
            fileCountTd.appendChild(fileCount)
            const tr = document.createElement("tr")
            tr.appendChild(nameTd)
            tr.appendChild(fileCountTd)
            table.appendChild(tr)
        }
    });
}