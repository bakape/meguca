import { Post } from './model'
import {
    makeFrag, importTemplate, getID, escape, firstChild, pad, on
} from '../util'
import { parseBody, relativeTime, renderPostLink } from './render'
import ImageHandler from "./images"
import { ViewAttrs } from "../base"
import { findSyncwatches } from "./syncwatch"
import lang from "../lang"
import { page, mine, posts } from "../state"
import options from "../options"
import countries from "./countries"
import { secondsToTime } from "../util/time"
import { ModerationAction } from '../common';


const modLevelStrings = ["", "janitors", "moderators", "owners", "admin"];

// Base post view class
export default class PostView extends ImageHandler {
    constructor(model: Post, el: HTMLElement | null) {
        const attrs: ViewAttrs = { model }
        if (el) {
            attrs.el = el
            if (el.classList.contains("deleted") && !model.isDeleted()) {
                // Hide own deletions
                el.classList.remove("deleted");
            }
        } else {
            attrs.class = 'glass'
            if (model.editing) {
                attrs.class += ' editing'
            }
            if (model.id === model.op) {
                attrs.class += " op"
            }
            if (model.isDeleted()) {
                attrs.class += " deleted"
                if (options.hideBinned) {
                    attrs.class += " hidden"
                }
            }
            attrs.tag = "article"
            attrs.id = "p" + model.id
        }
        super(attrs)

        this.model.view = this
        if (!el) {
            this.el.append(importTemplate("article"))
            this.render()
            this.autoExpandImage()
        } else if (this.model.moderation) {
            // Localize moderation log
            this.renderModerationLog();
        }
    }

    // Render the element contents, but don't insert it into the DOM
    public render() {
        if (this.model.subject) {
            const el = this.el.querySelector("h3")
            el.innerHTML = `「${escape(this.model.subject)}」`
            el.hidden = false
        }

        this.el.querySelector("blockquote").innerHTML = parseBody(this.model)
        if (this.model.backlinks) {
            this.renderBacklinks()
        }
        if (this.model.moderation && this.model.moderation.length) {
            this.renderModerationLog()
        }
        this.renderHeader()
        if (this.model.image) {
            this.renderImage(false)
        }
    }

    // Get the current Element for text to be written to
    private buffer(): Element {
        const { state: { spoiler, quote, bold, italic, red, blue } } = this.model
        let buf = this.el.querySelector("blockquote") as Element
        for (let b of [quote, spoiler, bold, italic, red, blue]) {
            if (b) {
                buf = buf.lastElementChild
            }
        }
        return buf
    }

    // check if we can see the post or have scrolled past it
    public scrolledPast() {
        const rect = this.el.getBoundingClientRect(),
            viewW = document.documentElement.clientWidth,
            viewH = document.documentElement.clientHeight;
        return rect.bottom < viewH && rect.left > 0 && rect.left < viewW
    }

    // Replace the current body with a reparsed fragment
    public reparseBody() {
        const bq = this.el.querySelector("blockquote")
        bq.innerHTML = ""
        bq.append(makeFrag(parseBody(this.model)))
        if (this.model.state.haveSyncwatch) {
            findSyncwatches(this.el)
        }
    }

    // Append a string to the current text buffer
    public appendString(s: string) {
        this.buffer().append(s)
    }

    // Remove one character from the current buffer
    public backspace() {
        const buf = this.buffer()
        // Merge multiple successive nodes created by appendString()
        buf.normalize()
        buf.innerHTML = buf.innerHTML.slice(0, -1)
    }

    // Render links to posts linking to this post
    public renderBacklinks() {
        // Find backlink span or create one
        let el = firstChild(this.el, ch =>
            ch.classList.contains("backlinks"))
        if (!el) {
            el = document.createElement("span")
            el.classList.add("spaced", "backlinks")
            this.el.append(el)
        }

        // Get already rendered backlink IDs
        const rendered = new Set<number>()
        for (let em of Array.from(el.children)) {
            const id = (em.firstChild as HTMLElement).getAttribute("data-id")
            rendered.add(parseInt(id))
        }

        let html = ""
        for (let idStr in this.model.backlinks) {
            const id = parseInt(idStr)
            // Confirm link not already rendered
            if (rendered.has(id)) {
                continue
            }
            rendered.add(id)
            const bl = this.model.backlinks[id]
            html += "<em>"
                + renderPostLink({
                    id: id,
                    op: bl.op,
                    board: bl.board,
                })
                + "</em>"
        }

        el.append(makeFrag(html))
    }

    // Render the header on top of the post
    protected renderHeader() {
        this.renderTime()
        this.renderName()
        if (this.model.sticky) {
            this.renderSticky()
        }

        const nav = this.el.querySelector("nav"),
            link = nav.firstElementChild as HTMLAnchorElement,
            quote = nav.lastElementChild as HTMLAnchorElement,
            { id, flag } = this.model
        let url = `#p${id}`
        if (!page.thread && !page.catalog) {
            url = `/all/${id}?last=100` + url
        }
        quote.href = link.href = url
        quote.textContent = id.toString()

        // Render country flag, if any
        if (flag) {
            const el = this.el.querySelector(".flag") as HTMLElement
            el.setAttribute("src", `/assets/flags/${flag}.svg`)
            el.setAttribute("title", countries[flag] || flag)
            el.hidden = false
            if (flag.includes("us-")) {
                const sec = el.cloneNode(true) as HTMLElement
                sec.setAttribute("src", `/assets/flags/us.svg`)
                sec.setAttribute("title", countries["us"] || "us")
                el.insertAdjacentElement("beforebegin", sec)
            }
        }
    }

    // Renders a time element. Can be either absolute or relative.
    public renderTime() {
        const abs = this.readableTime()
        const rel = relativeTime(this.model.time)
        const el = this.el.querySelector("time")
        // this is called on all posts in a thread by a timer
        // minimize DOM mutations when there's nothing to update
        const currentTitle = el.getAttribute("title")
        const newTitle = options.relativeTime ? abs : rel
        if (currentTitle != newTitle) {
            el.setAttribute("title", newTitle)
        }
        const currentText = el.textContent;
        const newText = options.relativeTime ? rel : abs;
        if (currentText != newText) {
            el.textContent = newText
        }
    }

    // Renders classic absolute timestamp
    private readableTime(): string {
        const d = new Date(this.model.time * 1000)
        return `${pad(d.getDate())} ${lang.time.calendar[d.getMonth()]} `
            + `${d.getFullYear()} (${lang.time.week[d.getDay()]}) `
            + `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    }

    // Close an open post and clean up
    public closePost() {
        this.setEditing(false)
        this.reparseBody()
    }

    // Stop post from displaying
    public hide() {
        this.el.classList.add("hidden")
    }

    // Stop hiding the post
    public unhide() {
        this.el.classList.remove("hidden")
    }

    // Render the name and tripcode in the header
    public renderName() {
        const el = this.el.querySelector(".name")
        if (options.anonymise) {
            el.innerHTML = lang.posts["anon"]
            return
        }

        let html = ""
        const { trip, name, auth, sage, id } = this.model
        if (name || !trip) {
            html += `<span>${name ? escape(name) : lang.posts["anon"]}</span>`
        }
        if (trip) {
            html += `<code>!${escape(trip)}</code>`
        }
        if (auth) { // Render staff title
            el.classList.add("admin")
            html +=
                `<span>## ${lang.posts[modLevelStrings[auth]] || "??"}</span>`;
        }
        if (mine.has(id)) {
            html += `<i>${lang.posts["you"]}</i>`
        }
        el.classList.toggle("sage", !!sage)
        el.innerHTML = html
    }

    // Render related mod-log status
    public renderModerationLog() {
        this.uncheckModerationBox()
        if (page.catalog) {
            return;
        }
        const pc = this.el.querySelector(".post-container");
        for (let el of Array.from(pc.children)) {
            if (el.classList.contains("post-moderation")) {
                el.remove();
            }
        }

        if (!this.model.moderation) {
            return;
        }
        for (let { type, length, by, data } of this.model.moderation) {
            let s: string;
            switch (type) {
                case ModerationAction.banPost:
                    s = this.format('banned', by, secondsToTime(length)
                        .toUpperCase(), data);
                    break;
                case ModerationAction.shadowBinPost:
                    if (mine.has(this.model.id)) {
                        // Hide own deletes from user
                        continue;
                    }
                    s = this.format('shadowBinned', by, secondsToTime(length)
                        .toUpperCase(), data);
                    break;
                case ModerationAction.deletePost:
                    if (mine.has(this.model.id)) {
                        // Hide own deletes from user
                        continue;
                    }
                    // Shadow bins require a reason
                    if (data) {
                        s = this.format("deletedReason", by, data);
                    } else {
                        s = this.format("deleted", by);
                    }
                    break;
                case ModerationAction.deleteImage:
                    s = this.format('imageDeleted', by);
                    break;
                case ModerationAction.spoilerImage:
                    s = this.format("imageSpoilered", by)
                    break;
                case ModerationAction.lockThread:
                    s = this.format("threadLockToggled",
                        lang.posts[data === 'true' ? "locked" : "unlocked"],
                        by)
                    break;
                case ModerationAction.meidoVision:
                    s = this.format("viewedSameIP", by);
                    break;
                case ModerationAction.purgePost:
                    s = this.format("purgedPost", by, data);
                    break;
                case ModerationAction.unbanPost:
                    s = this.format('unbanned', by);
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
            const el = document.createElement('b');
            el.setAttribute("class", "admin post-moderation");
            el.append(s, document.createElement("br"));
            pc.append(el);
        }
    }

    // C-like sprintf() but only for `%s` tags
    private format(formatKey: string, ...args: string[]): string {
        let i = 0;
        return lang.format[formatKey].replace(/%s/g, _ =>
            args[i++]);
    }

    // Add or remove highlight to post
    public setHighlight(on: boolean) {
        this.el.classList.toggle("highlight", on)
    }

    // Set display as an open post, that is being edited
    public setEditing(on: boolean) {
        this.el.classList.toggle("editing", on)
    }

    // Render the sticky status of a thread OP
    public renderSticky() {
        this.renderIcon("sticky", this.model.sticky)
    }

    // Render thread lock status
    public renderLocked() {
        this.renderIcon("locked", this.model.locked)
    }

    // Render an SVG icon in the header
    private renderIcon(id: string, render: boolean) {
        const old = this.el.querySelector("." + id)
        if (old) {
            old.remove()
        }
        if (render) {
            this.el.querySelector(".mod-checkbox").after(importTemplate(id))
        }
    }

    // Inserts PostView back into the thread ordered by id
    public reposition() {
        // Insert before first post with greater ID
        const { id, op } = this.model,
            sec = document.querySelector(`section[data-id="${op}"]`)
        if (!sec) {
            return
        }
        for (let el of Array.from(sec.children)) {
            switch (el.tagName) {
                case "ARTICLE":
                    if (getID(el) > id) {
                        el.before(this.el)
                        return
                    }
                    break
                case "ASIDE": // On board pages
                    el.before(this.el)
                    return
            }
        }

        // This post should be last or no posts in thread
        sec.append(this.el)
    }
}

function updateTimeTooltip(event: MouseEvent) {
    // tooltip only needs updates when the text node contains absolute time
    if (options.relativeTime) {
        return
    }
    if (!(event.target instanceof HTMLElement)) {
        return
    }
    const target = event.target
    const post = target.closest("article[id^=p]")
    const postId = post && post.id.match(/\d+/)[0] as any | 0
    const model = postId && posts.get(postId)
    const view = model && model.view
    if (!view) {
        return;
    }

    view.renderTime();
}

on(document, "mouseover", updateTimeTooltip, {
    passive: true,
    selector: "time",
})
