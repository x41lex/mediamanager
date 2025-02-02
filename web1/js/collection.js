var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { apiGetCollection, apiGetTags } from "./api.js";
window.onload = () => __awaiter(void 0, void 0, void 0, function* () {
    const table = document.getElementById("table_body");
    // So we get all tags
    const tags = yield apiGetTags();
    tags.forEach((t) => __awaiter(void 0, void 0, void 0, function* () {
        if (t.startsWith("collection:")) {
            const nameTd = document.createElement("td");
            const nameElement = document.createElement("a");
            nameElement.text = t.split(":")[1];
            nameElement.href = `/file?collection=${nameElement.text}`;
            nameTd.appendChild(nameElement);
            const fileCountTd = document.createElement("td");
            const fileCount = document.createElement("a");
            // Get collection info
            apiGetCollection(nameElement.text).then(e => {
                fileCount.text = `${e.length}`;
            });
            fileCount.text = `(not set)`;
            fileCountTd.appendChild(fileCount);
            const tr = document.createElement("tr");
            tr.appendChild(nameTd);
            tr.appendChild(fileCountTd);
            table.appendChild(tr);
        }
    }));
});
