class Ehentai extends ComicSource {
    name = "exhentai 镜像"
    key = "exhentai_mirror"
    version = "1.0.0"
    minAppVersion = "1.5.3"
    url = ""

    parseUrl(url) {
        let segments = url.split("/")
        let id = segments[4]
        let token = segments[5]
        return { id, token }
    }

    get baseUrl() {
        return "https://ex.4545810.xyz"
    }

    getStarsFromPosition(position) {
        let i = 0;
        while (position[i] !== ";") {
            i++;
            if (i === position.length) break;
        }
        switch (position.substring(0, i)) {
            case "background-position:0px -1px": return 5;
            case "background-position:0px -21px": return 4.5;
            case "background-position:-16px -1px": return 4;
            case "background-position:-16px -21px": return 3.5;
            case "background-position:-32px -1px": return 3;
            case "background-position:-32px -21px": return 2.5;
            case "background-position:-48px -1px": return 2;
            case "background-position:-48px -21px": return 1.5;
            case "background-position:-64px -1px": return 1;
            case "background-position:-64px -21px": return 0.5;
        }
        return 0.5;
    }

    async onLoadFailed() {
        throw "You may not have permission to access this page."
    }

    async getGalleries(url, isLeaderBoard) {
        let t = isLeaderBoard ? 1 : 0;
        let res
        try {
            res = await Network.get(url, {});
        } catch (e) {
            throw e
        }
        if (res.status !== 200) {
            throw `Invalid status code: ${res.status}`
        }
        if (res.body.trim().length === 0) {
            throw "Failed to load page"
        }
        if (res.body[0] !== '<') {
            throw "Failed to load page"
        }
        let document = new HtmlDocument(res.body);
        let galleries = [];

        for (let item of document.querySelectorAll("table.itg.gltc > tbody > tr")) {
            try {
                let type = item.children[0 + t].children[0].text;
                let time = item.children[1 + t].children[2].children[0].text;
                let stars = this.getStarsFromPosition(item.children[1 + t].children[2].children[1].attributes["style"])
                let cover = item.children[1 + t].children[1].children[0].children[0].attributes["src"];
                if (cover[0] === 'd') {
                    cover = item.children[1 + t].children[1].children[0].children[0].attributes["data-src"];
                }
                let title = item.children[2 + t].children[0].children[0].text;
                let link = item.children[2 + t].children[0].attributes["href"];
                let uploader = "";
                let pages = 0;
                try {
                    if (url.includes("/favorites.php")) {
                        pages = Number(item.children[1 + t].children[1].children[1].children[1].children[1].text.match(/\d+/)[0]);
                    } else {
                        pages = Number(item.children[3 + t].children[1].text.match(/\d+/)[0]);
                        uploader = item.children[3 + t].children[0].children[0].text;
                    }
                } catch (e) {}
                let tags = [];
                let language = null
                for (let node of item.children[2 + t].children[0].children[1].children) {
                    let tag = node.attributes["title"]
                    if (tag.startsWith("language:")) {
                        let l = tag.split(":")[1].trim()
                        language = l === 'translated' ? language : l
                        continue
                    }
                    tags.push(tag)
                }
                galleries.push(new Comic({
                    id: link, title: title, subTitle: uploader, cover: cover, tags: tags,
                    description: time, stars: stars, maxPage: pages, language: language
                }));
            } catch (e) {}
        }

        for (let item of document.querySelectorAll("div.gl1t")) {
            try {
                let title = item.querySelector("a")?.text ?? "Unknown";
                let time = item.querySelectorAll("div.gl5t > div > div").find((e) => !isNaN(Date.parse(e.text)))?.text;
                let coverPath = item.querySelector("img")?.attributes["src"] ?? "";
                let stars = this.getStarsFromPosition(item.querySelector("div.gl5t > div > div.ir")?.attributes["style"] ?? "");
                let link = item.querySelector("a")?.attributes["href"] ?? "";
                let pages = Number(item.querySelectorAll("div.gl5t > div > div").find((e) => e.text.includes("page"))?.text.match(/\d+/)[0] ?? "0");
                galleries.push(new Comic({
                    id: link, title: title, cover: coverPath, description: time, stars: stars, maxPage: pages,
                }));
            } catch (e) {}
        }

        for (let item of document.querySelectorAll("table.itg.glte > tbody > tr")) {
            try {
                let title = item.querySelector("td.gl2e > div > a > div > div.glink")?.text ?? "Unknown";
                let time = item.querySelectorAll("td.gl2e > div > div.gl3e > div").find((e) => !isNaN(Date.parse(e.text)))?.text ?? "Unknown";
                let uploader = item.querySelector("td.gl2e > div > div.gl3e > div > a")?.text ?? "Unknown";
                let coverPath = item.querySelector("td.gl1e > div > a > img")?.attributes["src"] ?? "";
                let stars = this.getStarsFromPosition(item.querySelector("td.gl2e > div > div.gl3e > div.ir")?.attributes["style"] ?? "");
                let link = item.querySelector("td.gl1e > div > a")?.attributes["href"] ?? "";
                let tags = item.querySelectorAll('div.gt, div.gtl').map((e) => e.attributes["title"] ?? "");
                let pages = Number(item.querySelectorAll("td.gl2e > div > div.gl3e > div").find((e) => e.text.includes("page"))?.text.match(/\d+/)[0] ?? "");
                let language = tags.find((e) => e.startsWith("language:") && !e.includes('translated'))?.split(":")[1].trim() ?? null;
                galleries.push(new Comic({
                    id: link, title: title, subTitle: uploader, cover: coverPath, tags: tags,
                    description: time, stars: stars, maxPage: pages, language: language
                }))
            } catch (e) {}
        }

        for (let item of document.querySelectorAll("table.itg.gltm > tbody > tr")) {
            try {
                let title = item.querySelector("td.gl3m > a > div.glink")?.text ?? "Unknown";
                let time = item.querySelectorAll("td.gl2m > div").find((e) => !isNaN(Date.parse(e.text)))?.text ?? "Unknown";
                let uploader = item.querySelector("td.gl5m > div > a")?.text ?? "Unknown";
                let coverPath = item.querySelector("td.gl2m > div > div > img")?.attributes["src"];
                if (coverPath[0] === 'd') {
                    coverPath = item.querySelector("td.gl2m > div > div > img")?.attributes["data-src"];
                }
                let stars = this.getStarsFromPosition(item.querySelector("td.gl4m > div.ir")?.attributes["style"] ?? "");
                let link = item.querySelector("td.gl3m > a")?.attributes["href"] ?? "";
                galleries.push(new Comic({
                    id: link, title: title, subTitle: uploader, cover: coverPath, description: time, stars: stars,
                }))
            } catch (e) {}
        }

        let nextButton = document.querySelector("a#dnext");
        let next = nextButton?.attributes["href"]
        document.dispose()
        return { comics: galleries, next: next }
    }

    explore = [
        { title: "eh latest", type: "multiPageComicList", loadNext: (next) => this.getGalleries(next ?? this.baseUrl, false) },
        { title: "eh popular", type: "multiPageComicList", loadNext: (next) => this.getGalleries(next ?? `${this.baseUrl}/popular`, false) },
    ]

    category = { title: "exhentai", parts: [], enableRankingPage: true }

    categoryComics = {
        ranking: {
            options: ["15-yesterday", "13-month", "12-year", "11-all"],
            load: async (option, page) => {
                let res = await this.getGalleries(`https://e-hentai.org/toplist.php?tl=${option}&p=${page-1}`, true);
                return { comics: res.comics, maxPage: 200 }
            }
        }
    }

    search = {
        loadNext: async (keyword, options, next) => {
            let category = JSON.parse(options[0]);
            let stars = options[1];
            let language = options[2];
            let fcats = 1023
            if (!Array.isArray(category)) category = [category];
            for (let c of category) fcats -= 1 << Number(c)
            if (language && !keyword.includes("language:")) keyword += ` language:${language}`
            let url = `${this.baseUrl}/?f_search=${encodeURIComponent(keyword)}`
            if (fcats) url += `&f_cats=${fcats}`
            if (stars) url += `&f_srdd=${stars}`
            return this.getGalleries(next ?? url, false);
        },
        optionList: [
            { type: "multi-select", options: ["0-Misc", "1-Doujinshi", "2-Manga", "3-Artist CG", "4-Game CG", "5-Image Set", "6-Cosplay", "7-Asian Porn", "8-Non-H", "9-Western"], label: "Category", default: ["0","1","2","3","4","5","6","7","8","9"] },
            { type: "dropdown", options: ["-<none>", "0-0", "1-1", "2-2", "3-3", "4-4", "5-5"], label: "Min Stars" },
            { type: "dropdown", options: ["-<none>", "chinese-Chinese", "english-English", "japanese-Japanese"], label: "Language" },
        ],
        enableTagsSuggestions: true,
    }

    comic = {
        loadInfo: async (id) => {
            let res = await Network.get(id, { 'cookie': 'nw=1' });
            if (res.status !== 200) throw `Invalid status code: ${res.status}`
            if (res.body.trim().length === 0) throw "Exception: empty data"
            let document = new HtmlDocument(res.body);

            let tags = new Map();
            for (let tr of document.querySelectorAll("div#taglist > table > tbody > tr")) {
                tags.set(
                    tr.children[0].text.substring(0, tr.children[0].text.length - 1),
                    tr.children[1].children.map((e) => e.children[0].attributes["onclick"].split(":")[1].split("'")[0])
                )
            }

            let maxPage = "1"
            for (let element of document.querySelectorAll("td.gdt2")) {
                if (element.text.includes("page")) maxPage = element.text.match(/\d+/)[0];
            }

            let isFavorited = true;
            if (document.querySelector("a#favoritelink")?.text === " Add to Favorites") isFavorited = false;

            let coverPath = document.querySelector("div#gleft > div#gd1 > div").attributes["style"];
            coverPath = RegExp("https?://([-a-zA-Z0-9.]+(/\\S*)?\\.(?:jpg|jpeg|gif|png|webp))").exec(coverPath)[0];

            let uploader = document.getElementById("gdn")?.children[0]?.text
            let stars = Number(document.getElementById("rating_label")?.text?.split(':')?.at(1)?.trim());
            let category = document.querySelector("div.cs").text;
            tags.set("Category", [category])
            if (uploader) tags.set("uploader", [uploader]);
            let time = document.querySelector("div#gdd > table > tbody > tr > td.gdt2").text
            let title = document.querySelector("h1#gn").text;
            let subtitle = document.querySelector("h1#gj")?.text;
            if (subtitle != null && subtitle.trim() === "") subtitle = null;
            let comments = this.comic.parseComments(document)

            let comic = new ComicDetails({
                id: id, title: title, subTitle: subtitle, cover: coverPath, tags: tags, stars: stars,
                maxPage: Number(maxPage), isFavorite: isFavorited, uploadTime: time, url: id, comments: comments.comments,
            })
            document.dispose()
            return comic;
        },

        loadThumbnails: async (id, next) => {
            let url = id
            if (next != null) url += `?p=${next}`
            let res = await Network.get(url, { 'cache-time': 'long', 'prevent-parallel': 'true', 'cookie': 'nw=1' });
            if (res.status !== 200) throw `Invalid status code: ${res.status}`
            let document = new HtmlDocument(res.body);
            let parseImageUrl = (e) => {
                let style = e.attributes['style'];
                let width = Number(style.split('width:')[1].split('px')[0])
                let height = Number(style.split('height:')[1].split('px')[0])
                let r = style.split("background:transparent url(")[1]
                let url = r.split(")")[0]
                let range = '';
                if (r.includes('px')) {
                    let position = Number(r.split(') -')[1].split('px')[0])
                    range += `x=${position}-${position + width}`
                }
                if (height) range += `${range ? "&" : ""}y=0-${height}`;
                if (range) url += `@${range}`;
                return url;
            };
            let images = document.querySelectorAll("div.gdtm > div").map((e) => parseImageUrl(e));
            images.push(...document.querySelectorAll("div.gdtl > a > img").map((e) => e.attributes["src"]))
            if (images.length === 0) {
                for (let e of document.querySelectorAll("div.gt100 > a > div").map(e => e.children.length === 0 ? e : e.children[0])) images.push(parseImageUrl(e))
                for (let e of document.querySelectorAll("div.gt200 > a > div").map(e => e.children.length === 0 ? e : e.children[0])) images.push(parseImageUrl(e))
            }
            let urls = document.querySelectorAll("table.ptb > tbody > tr > td > a").map((e) => e.attributes["href"])
            let pageNumbers = urls.map((e) => { let n = Number(e.split("=")[1]); if (isNaN(n)) return 0; return n })
            let maxPage = Math.max(...pageNumbers)
            let current = 0
            if (next) current = Number(next)
            current += 1
            if (current > maxPage) current = null
            else current = current.toString()
            let _urls = document.querySelectorAll("div#gdt a").map((e) => e.attributes["href"])
            document.dispose()
            return { thumbnails: images, urls: _urls, next: current }
        },

        loadEp: async (comicId, epId) => {
            let comic = await this.comic.loadInfo(comicId)
            return { images: Array.from({ length: comic.maxPage }, (_, i) => i.toString()) }
        },

        onImageLoad: async (image, comicId, epId, nl) => {
            let page = Number(image)
            let url = comicId
            if (page > 0) url += "?p=" + page

            let res = await Network.get(url, { 'cookie': 'nw=1' })
            if (res.status !== 200) throw `Invalid status code: ${res.status}`
            let document = new HtmlDocument(res.body)

            let parseBgUrl = (el) => {
                let style = el.attributes['style'];
                if (!style) return null;
                let m = style.match(/url\(["']?([^"')]+)["']?\)/);
                if (!m) return null;
                let bgUrl = m[1];
                let w = style.split('width:')[1]?.split('px')[0]
                let h = style.split('height:')[1]?.split('px')[0]
                let pos = style.split(') -')[1]?.split('px')[0]
                let range = '';
                if (w) range += `x=${pos}-${Number(pos) + Number(w)}`
                if (h) range += `${range ? "&" : ""}y=0-${h}`;
                if (range) bgUrl += `@${range}`;
                return bgUrl;
            }

            let imgEl = document.querySelector("div.gt100 > a > div")
                || document.querySelector("div.gt200 > a > div")
                || document.querySelector("div.gdtm > div")

            let imageUrl = null
            if (imgEl) {
                let child = imgEl.children.length === 0 ? imgEl : imgEl.children[0]
                imageUrl = parseBgUrl(child)
            }
            if (!imageUrl) {
                let img = document.querySelector("div.gt100 > a > img")
                    || document.querySelector("div.gt200 > a > img")
                if (img) imageUrl = img.attributes["src"]
            }

            document.dispose()
            if (!imageUrl) throw "Failed to parse image URL from page"
            return { url: imageUrl, headers: { 'referer': this.baseUrl } }
        },

        onThumbnailLoad: (url) => {
            if (url.includes('s.exhentai.org')) url = url.replace("s.exhentai.org", "ehgt.org")
            return { url: url, headers: { 'referer': this.baseUrl } }
        },

        parseComments: (document) => {
            let comments = []
            for (let c of document.querySelectorAll('div.c1')) {
                let name = c.querySelector('div.c3 > a')?.text ?? ""
                let time = c.querySelector('div.c3')?.text?.split("Posted on")?.at(1)?.split('by')?.at(0)?.trim() ?? 'unknown'
                let content = typeof appVersion ? c.querySelector('div.c6').innerHTML : c.querySelector('div.c6').text
                let score = Number(c.querySelector('div.c5 > span')?.text)
                if (isNaN(score)) score = null
                let id = c.previousElementSibling?.attributes['name']?.match(/\d+/)[0] ?? '0'
                comments.push(new Comment({ id, content, time, userName: name, score }))
            }
            return { comments, maxPage: 1 }
        },

        loadComments: async (comicId, subId, page, replyTo) => {
            let res = await Network.get(`${comicId}?hc=1`, { 'cookie': 'nw=1' });
            if (res.status !== 200) throw `Invalid status code: ${res.status}`
            let document = new HtmlDocument(res.body)
            let result = this.comic.parseComments(document)
            document.dispose()
            return result
        },

        onClickTag: (namespace, tag) => {
            if (namespace == "Category") {
                const categories = ["misc", "doujinshi", "manga", "artist cg", "game cg", "image set", "cosplay", "asian porn", "non-h", "western"];
                return { page: "search", attributes: { keyword: "", options: [categories.indexOf(tag.toLowerCase()).toString(), "", ""] } };
            }
            if (tag.includes(' ')) tag = `"${tag}"`
            return { action: 'search', keyword: `${namespace}:${tag}`, param: null }
        },

        link: {
            domains: ['e-hentai.org', 'exhentai.org', 'ex.4545810.xyz'],
            linkToId: (url) => {
                if (url.includes('?')) url = url.split('?')[0]
                let match = RegExp("https?://(e-|ex)hentai\\.org/g/(\\d+)/(\\w+)/?$").exec(url)
                if (match) return `${this.baseUrl}/g/${match[2]}/${match[3]}/`
                let match2 = RegExp("https?://ex\\.4545810\\.xyz/g/(\\d+)/(\\w+)/?$").exec(url)
                if (match2) return `${this.baseUrl}/g/${match2[1]}/${match2[2]}/`
                return null
            }
        },
        enableTagsTranslate: true,
    }

    settings = {
        domain: { title: "domain", type: "select", options: [{ value: 'ex.4545810.xyz' }], default: 'ex.4545810.xyz' },
    }

    translation = {
        'zh_CN': {
            "domain": "域名", "language": "语言", "artist": "画师", "male": "男性", "female": "女性", "mixed": "混合",
            "other": "其它", "parody": "原作", "character": "角色", "group": "团队", "cosplayer": "Coser",
            "reclass": "重新分类", "uploader": "上传者", "Languages": "语言", "Artists": "画师", "Characters": "角色",
            "Groups": "团队", "Tags": "标签", "Parodies": "原作", "Categories": "分类", "Category": "分类",
            "Min Stars": "最少星星", "Language": "语言",
        },
        'zh_TW': {
            'domain': '域名', "language": "語言", "artist": "畫師", "male": "男性", "female": "女性", "mixed": "混合",
            "other": "其他", "parody": "原作", "character": "角色", "group": "團隊", "cosplayer": "Coser",
            "reclass": "重新分類", "uploader": "上傳者", "Languages": "語言", "Artists": "畫師", "Characters": "角色",
            "Groups": "團隊", "Tags": "標籤", "Parodies": "原作", "Categories": "分類", "Category": "分類",
            "Min Stars": "最少星星", "Language": "語言",
        },
        'en_US': {
            "domain": "Domain", "language": "Language", "artist": "Artist", "male": "Male", "female": "Female", "mixed": "Mixed",
            "other": "Other", "parody": "Parody", "character": "Character", "group": "Group", "cosplayer": "Cosplayer",
            "reclass": "Reclass", "uploader": "Uploader", "Languages": "Languages", "Artists": "Artists", "Characters": "Characters",
            "Groups": "Groups", "Tags": "Tags", "Parodies": "Parodies", "Categories": "Categories", "Category": "Category",
            "Min Stars": "Min Stars", "Language": "Language",
        },
    }
}
