import {
    PostData, PostLink, ModerationLevel, ModerationEntry, Command, ImageData,
    ModerationAction, fileTypes
} from "../client/common"
import { importTemplate } from "../client/util/render"
import countries from "../client/posts/countries"
import lang from "../client/lang"
import { secondsToTime } from "../client/util/time"

const modLevelStrings = ["", "janitor", "moderators", "owners", "admin"];

let imageRoot: string;

export class Post {
    public el: HTMLElement;

    public editing: boolean;
    public sage: boolean;
    public image: ImageData;
    public time: number;
    public id: number;
    public op: number;
    public body: string;
    public name: string;
    public trip: string;
    public auth: ModerationLevel;
    public board: string;
    public flag: string;
    public links: PostLink[];
    public commands: Command[];
    public moderation: ModerationEntry[];

    constructor(attrs: PostData) {
        extend(this, attrs);
    }

    public async render() {
        this.el = document.createElement("article");
        if (this.isDeleted()) {
            this.el.classList.add("deleted");
        }
        this.el.id = `p${this.id}`;
        this.el.append(importTemplate("article"));

        this.el.querySelector("blockquote").innerHTML = this.body;
        if (this.moderation && this.moderation.length) {
            this.renderModerationLog();
        }
        this.renderHeader();
        if (this.image) {
            await this.renderImage();
        }
    }

    // Render the header on top of the post
    private renderHeader() {
        this.renderTime();
        this.renderName();

        const nav = this.el.querySelector("nav"),
            quote = nav.lastElementChild as HTMLAnchorElement;
        quote.textContent = this.id.toString();

        // Render country flag, if any
        if (this.flag) {
            const el = this.el.querySelector(".flag") as HTMLImageElement;
            el.src = `/assets/flags/${this.flag}.svg`;
            el.title = countries[this.flag] || this.flag;
            el.hidden = false;
            if (this.flag.includes("us-")) {
                const sec = el.cloneNode(true) as HTMLImageElement;
                sec.src = "/assets/flags/us.svg";
                sec.title = countries["us"] || "us";
                el.insertAdjacentElement("beforebegin", sec);
            }
        }
    }

    // Renders classic absolute timestamp
    private renderTime() {
        const el = this.el.querySelector("time");
        const d = new Date(this.time * 1000);
        el.textContent = `${pad(d.getDate())} ${lang.time.calendar[d.getMonth()]} `
            + `${d.getFullYear()} (${lang.time.week[d.getDay()]}) `
            + `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
    }

    // Render the name and tripcode in the header
    private renderName() {
        const el = this.el.querySelector(".name");
        let html = "";
        if (this.name || !this.trip) {
            html += `<span>${
                this.name ? escape(this.name) : lang.posts["anon"]}</span>`;
        }
        if (this.trip) {
            html += `<code>!${escape(this.trip)}</code>`;
        }
        if (this.auth) { // Render staff title
            el.classList.add("admin");
            html += `<span>## ${
                lang.posts[modLevelStrings[this.auth]] || "??"}</span>`;
        }
        el.classList.toggle("sage", !!this.sage);
        el.innerHTML = html;
    }

    private renderModerationLog() {
        for (let { type, length, by, data } of this.moderation) {
            let s: string;
            switch (type) {
                case ModerationAction.banPost:
                    s = this.format("banned", by, secondsToTime(length)
                        .toUpperCase(), data);
                    break;
                case ModerationAction.shadowBinPost:
                    s = this.format("shadowBinned", by, secondsToTime(length)
                        .toUpperCase(), data);
                    break;
                case ModerationAction.deletePost:
                    // Shadow bins require a reason
                    if (data) {
                        s = this.format("deletedReason", by, data);
                    } else {
                        s = this.format("deleted", by);
                    }
                    break;
                case ModerationAction.deleteImage:
                    s = this.format("imageDeleted", by);
                    break;
                case ModerationAction.spoilerImage:
                    s = this.format("imageSpoilered", by);
                    break;
                case ModerationAction.lockThread:
                    s = this.format(
                        "threadLockToggled",
                        lang.posts[data === "true" ? "locked" : "unlocked"],
                        by,
                    );
                    break;
                case ModerationAction.meidoVision:
                    s = this.format("viewedSameIP", by);
                    break;
                case ModerationAction.purgePost:
                    s = this.format("purgedPost", by, data);
                    break;
                case ModerationAction.unbanPost:
                    s = this.format("unbanned", by);
                    break;
                case ModerationAction.redirectIP:
                    s = this.format("redirectIP", data, by);
                    break;
                case ModerationAction.redirectThread:
                    s = this.format("redirectThread", data, by);
                    break;
                default:
                    continue;
            }
            const el = document.createElement("b");
            el.classList.add("admin", "post-moderation");
            el.append(s, document.createElement("br"));
            this.el.querySelector(".post-container").append(el);
        }
    }

    // Render the figure and figcaption of a post
    private async renderImage() {
        const el = importTemplate("figure").firstChild as HTMLElement;
        this.el.querySelector(".post-container").prepend(el);
        (el.firstElementChild as HTMLElement).hidden = false;
        await this.renderThumbnail();
        this.renderFigCaption();
    }

    // Render the actual thumbnail image
    private async renderThumbnail() {
        const el = this.el.querySelector("figure a");
        let thumb: string,
            [, , thumbWidth, thumbHeight] = this.image.dims;

        if (this.image.thumb_type === fileTypes.noFile) {
            // No thumbnail exists
            let file: string;
            switch (this.image.file_type) {
                case fileTypes.webm:
                case fileTypes.mp4:
                case fileTypes.mp3:
                case fileTypes.ogg:
                case fileTypes.flac:
                    file = "audio";
                    break;
                default:
                    file = "file";
            }
            thumb = `/assets/${file}.png`;
            thumbHeight = thumbWidth = 150;
        } else if (this.image.spoiler) {
            thumb = "/assets/spoil/default.jpg";
            thumbHeight = thumbWidth = 150;
        } else {
            thumb = await thumbPath(this.image.sha1, this.image.thumb_type);
        }

        (el.firstElementChild as HTMLImageElement).src = thumb;
        (el.firstElementChild as HTMLImageElement).width = thumbWidth;
        (el.firstElementChild as HTMLImageElement).height = thumbHeight;
        // Remove any existing classes
        (el.firstElementChild as HTMLImageElement).className = "";
    }

    // Render the information caption above the image
    private renderFigCaption() {
        const el = importTemplate("figcaption").firstChild as HTMLElement;
        this.el.querySelector("header").after(el);

        const [info, ...tmp] = Array.from(el.children) as HTMLElement[];
        let link = tmp[tmp.length - 1];

        const arr = [];

        if (this.image.audio) {
            arr.push("♫");
        }

        if (this.image.length) {
            let s: string;
            if (this.image.length < 60) {
                s = `0:${pad(this.image.length)}`;
            } else {
                const min = Math.floor(this.image.length / 60);
                s = `${pad(min)}:${pad(this.image.length - min * 60)}`;
            }
            arr.push(s);
        }

        let s: string;
        if (this.image.size < (1 << 10)) {
            s = this.image.size + " B";
        } else if (this.image.size < (1 << 20)) {
            s = Math.round(this.image.size / (1 << 10)) + " KB";
        } else {
            const text = Math.round(this.image.size / (1 << 20) * 10)
                .toString();
            s = `${text.slice(0, -1)}.${text.slice(-1)} MB`;
        }
        arr.push(s);

        const [w, h] = this.image.dims;
        if (w || h) {
            arr.push(`${w}x${h}`);
        }

        if (this.image.artist) {
            arr.push(this.image.artist);
        }
        if (this.image.title) {
            arr.push(this.image.title);
        }

        let html = "";
        for (let s of arr) {
            html += `<span>${escape(s)}</span>`;
        }
        info.innerHTML = html;

        // Render name of an image
        const ext = fileTypes[this.image.file_type],
            name = `${escape(this.image.name)}.${ext}`;
        link.innerHTML = name;

        el.hidden = false;
    }

    private isDeleted(): boolean {
        if (!this.moderation) {
            return false;
        }
        for (let { type } of this.moderation) {
            if (type === ModerationAction.deletePost) {
                return true;
            }
        }

        return false;
    }

    // C-like sprintf() but only for `%s` tags
    private format(formatKey: string, ...args: string[]): string {
        let i = 0;
        return lang.format[formatKey].replace(/%s/g, _ =>
            args[i++]);
    }
}

// Copy all properties from the source object to the destination object. Nested
// objects are extended recursively.
function extend(dest: {}, source: {}) {
	for (let key in source) {
		const val = source[key]
		if (typeof val === "object" && val !== null) {
			const d = dest[key]
			if (d) {
				extend(d, val)
			} else {
				dest[key] = val
			}
		} else {
			dest[key] = val
		}
	}
}

async function getImageRoot(): Promise<string> {
    if (!imageRoot) {
        const res = await fetch("/json/config");
        const { imageRootOverride } = await res.json();
        imageRoot = imageRootOverride || "/assets/images";
    }
    return imageRoot;
}

// Get the thumbnail path of an image, accounting for thumbnail of specific
// type not being present
async function thumbPath(sha1: string, thumbType: fileTypes): Promise<string> {
    return `${await getImageRoot()}/thumb/${sha1}.${fileTypes[thumbType]}`;
}

// Pad an integer with a leading zero, if below 10
function pad(n: number): string {
	return (n < 10 ? '0' : '') + n;
}

const escapeMap: { [key: string]: string } = {
	"&": "&amp;",
	"'": "&#39;", // "&#39;" is shorter than "&apos
	"<": "&lt;",
	">": "&gt;",
	"\"": "&#34;", // "&#34;" is shorter than "&quot;"
};

// Escape a user-submitted unsafe string to protect against XSS.
function escape(str: string): string {
	return str.replace(/[&'<>"]/g, char =>
		escapeMap[char]);
}