import { apiGetVersion } from "./api.js"

window.onload = () => {
    apiGetVersion().then((v) => {
        const apiE = document.getElementById("api_version") as HTMLHeadElement
        apiE.textContent = `${v.FileDb.CodeName} (${v.FileDb.String})`
        const dbE = document.getElementById("db_version") as HTMLHeadElement
        let s = `${v.Database.CodeName} (${v.Database.String})`
        if(v.FileDb.Major !=  v.Database.Major) {
            s += " (Unsupported)";
            (document.getElementById("unsupported") as HTMLHeadElement).hidden = false
            
        } else if (v.FileDb.Minor != v.Database.Minor) {
            s += " (Outdated)"
        }
        dbE.textContent = s
    })
}